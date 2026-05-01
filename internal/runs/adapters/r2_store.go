package adapters

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"

	"refleks-api/internal/runs"
)

const defaultSignedURLTTL = 15 * time.Minute

// R2Config contains Cloudflare R2 connection settings.
type R2Config struct {
	Endpoint        string
	Region          string
	Bucket          string
	AccessKeyID     string
	SecretAccessKey string
	PublicBaseURL   string
	SignedURLTTL    time.Duration
}

// R2Store stores raw .refleks files in Cloudflare R2.
type R2Store struct {
	client        *s3.Client
	presignClient *s3.PresignClient
	bucket        string
	publicBaseURL string
	signedURLTTL  time.Duration
}

// NewR2Store creates an R2-backed object store.
func NewR2Store(ctx context.Context, cfg R2Config) (*R2Store, error) {
	endpoint := strings.TrimSpace(cfg.Endpoint)
	region := strings.TrimSpace(cfg.Region)
	bucket := strings.TrimSpace(cfg.Bucket)
	accessKeyID := strings.TrimSpace(cfg.AccessKeyID)
	secretAccessKey := strings.TrimSpace(cfg.SecretAccessKey)
	publicBaseURL := strings.TrimRight(strings.TrimSpace(cfg.PublicBaseURL), "/")
	signedURLTTL := cfg.SignedURLTTL

	if endpoint == "" {
		return nil, fmt.Errorf("r2 endpoint is required")
	}
	if region == "" {
		region = "auto"
	}
	if bucket == "" {
		return nil, fmt.Errorf("r2 bucket is required")
	}
	if accessKeyID == "" || secretAccessKey == "" {
		return nil, fmt.Errorf("r2 access key id and secret key are required")
	}
	if signedURLTTL <= 0 {
		signedURLTTL = defaultSignedURLTTL
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	client := s3.NewFromConfig(awsCfg, func(o *s3.Options) {
		o.UsePathStyle = true
		o.BaseEndpoint = &endpoint
	})
	presignClient := s3.NewPresignClient(client)

	return &R2Store{
		client:        client,
		presignClient: presignClient,
		bucket:        bucket,
		publicBaseURL: publicBaseURL,
		signedURLTTL:  signedURLTTL,
	}, nil
}

// Put writes one object.
func (s *R2Store) Put(ctx context.Context, objectKey string, data []byte) error {
	_, err := s.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket:        &s.bucket,
		Key:           &objectKey,
		Body:          bytes.NewReader(data),
		ContentLength: aws.Int64(int64(len(data))),
		ContentType:   strPtr("application/octet-stream"),
		CacheControl:  strPtr("public, max-age=31536000, immutable"),
	})
	if err != nil {
		return err
	}
	return nil
}

// Get reads one object and returns its streaming body and content length.
func (s *R2Store) Get(ctx context.Context, objectKey string) (io.ReadCloser, int64, error) {
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &s.bucket,
		Key:    &objectKey,
	})
	if err != nil {
		if isObjectNotFoundError(err) {
			return nil, 0, runs.ErrObjectNotFound
		}
		return nil, 0, err
	}

	size := int64(-1)
	if out.ContentLength != nil {
		size = *out.ContentLength
	}

	return out.Body, size, nil
}

// GetDownloadURL returns either a public URL or signed URL for an object.
func (s *R2Store) GetDownloadURL(ctx context.Context, objectKey, fileName string) (runs.DownloadURL, error) {
	if err := s.ensureObjectExists(ctx, objectKey); err != nil {
		return runs.DownloadURL{}, err
	}

	if s.publicBaseURL != "" {
		publicURL, err := buildPublicObjectURL(s.publicBaseURL, objectKey)
		if err != nil {
			return runs.DownloadURL{}, err
		}
		return runs.DownloadURL{
			URL:    publicURL,
			Access: "public",
		}, nil
	}

	in := &s3.GetObjectInput{
		Bucket:                     &s.bucket,
		Key:                        &objectKey,
		ResponseContentType:        strPtr("application/octet-stream"),
		ResponseContentDisposition: strPtr("attachment; filename=\"" + fileName + "\""),
	}

	presigned, err := s.presignClient.PresignGetObject(ctx, in, s3.WithPresignExpires(s.signedURLTTL))
	if err != nil {
		return runs.DownloadURL{}, err
	}
	expiresAt := time.Now().UTC().Add(s.signedURLTTL)

	return runs.DownloadURL{
		URL:       presigned.URL,
		Access:    "signed",
		ExpiresAt: &expiresAt,
	}, nil
}

// Delete removes one object if it exists.
func (s *R2Store) Delete(ctx context.Context, objectKey string) error {
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: &s.bucket,
		Key:    &objectKey,
	})
	if err != nil {
		return err
	}
	return nil
}

func (s *R2Store) ensureObjectExists(ctx context.Context, objectKey string) error {
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: &s.bucket,
		Key:    &objectKey,
	})
	if err == nil {
		return nil
	}
	if isObjectNotFoundError(err) {
		return runs.ErrObjectNotFound
	}
	return err
}

func isObjectNotFoundError(err error) bool {
	var apiErr smithy.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	code := apiErr.ErrorCode()
	return code == "NoSuchKey" || code == "NotFound"
}

func buildPublicObjectURL(baseURL, objectKey string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("invalid R2 public base URL: %w", err)
	}

	segments := strings.Split(strings.TrimLeft(objectKey, "/"), "/")
	for i := range segments {
		segments[i] = url.PathEscape(segments[i])
	}
	escapedObjectKey := strings.Join(segments, "/")
	u.Path = path.Join(u.Path, escapedObjectKey)
	return u.String(), nil
}

func strPtr(s string) *string {
	return &s
}
