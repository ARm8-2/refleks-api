package runs

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/klauspost/compress/zstd"
	"github.com/zeebo/xxh3"
	"google.golang.org/protobuf/encoding/protowire"
)

const (
	runMagic                 = "RFLK"
	runVersion         uint8 = 2
	runCompressionNone uint8 = 0
	runCompressionZstd uint8 = 1

	runHeaderSize   = 4 + 1 + 1 + 8
	runChecksumSize = 8

	maxFileNameBytes = 1024
	maxMousePoints   = 20000000
)

// ---- Field numbers (matches refleks/internal/runs/codec_protowire.go) ----

// RunStatsData field numbers
const (
	statsFieldSummary = 1
	statsFieldEvents  = 2
)

// RunStatsSummary field numbers
const (
	summaryFieldScenario   = 19
	summaryFieldScore      = 1
	summaryFieldAvgTTK     = 6
	summaryFieldRealAvgTTK = 45
	summaryFieldAccuracy   = 44
	summaryFieldCm360      = 46
	summaryFieldDuration   = 47
)

// RunEnvironment field numbers
const (
	envFieldSteamID     = 5
	envFieldPersonaName = 6
	envFieldMouseVID    = 16
	envFieldMousePID    = 17
	envFieldTracePoints = 20
)

type parsedStats struct {
	ScenarioName  string
	Score         *float64
	Accuracy      *float64
	AvgTTKSeconds *float64
	DurationSecs  *float64
	SensCM360     *float64
}

type parsedMouseMetrics struct {
	HasTrace bool
	AvgSpeed *float64
}

type parsedEnvironment struct {
	SteamID       string
	SteamUsername string
	MouseVID      string
	MousePID      string
	TracePoints   int32
}

// ParseRefleksFile validates a .refleks payload and extracts metadata for persistence.
func ParseRefleksFile(raw []byte) (ParsedRefleksFile, error) {
	if len(raw) < runHeaderSize+runChecksumSize {
		return ParsedRefleksFile{}, fmt.Errorf("%w: file too small", ErrInvalidRunFile)
	}
	if string(raw[:4]) != runMagic {
		return ParsedRefleksFile{}, fmt.Errorf("%w: bad magic", ErrInvalidRunFile)
	}

	version := raw[4]
	if version != runVersion {
		return ParsedRefleksFile{}, fmt.Errorf("%w: expected version %d, got %d", ErrInvalidRunFile, runVersion, version)
	}

	compression := raw[5]
	epoch := int64(binary.LittleEndian.Uint64(raw[6:14]))
	payload := raw[runHeaderSize : len(raw)-runChecksumSize]
	wantChecksum := binary.LittleEndian.Uint64(raw[len(raw)-runChecksumSize:])
	if gotChecksum := xxh3.Hash(payload); gotChecksum != wantChecksum {
		return ParsedRefleksFile{}, fmt.Errorf("%w: checksum mismatch", ErrInvalidRunFile)
	}

	parsed, err := parsePayload(payload, compression)
	if err != nil {
		return ParsedRefleksFile{}, err
	}
	parsed.PlayedAt = epoch
	parsed.FormatVersion = version
	return parsed, nil
}

func parsePayload(payload []byte, compression uint8) (ParsedRefleksFile, error) {
	payloadReader := bytes.NewReader(payload)
	decoded, closeDecoder, err := newPayloadDecoder(payloadReader, compression)
	if err != nil {
		return ParsedRefleksFile{}, fmt.Errorf("%w: %v", ErrInvalidRunFile, err)
	}
	defer closeDecoder()

	fileName, err := readString(decoded, maxFileNameBytes)
	if err != nil {
		return ParsedRefleksFile{}, fmt.Errorf("%w: read filename: %v", ErrInvalidRunFile, err)
	}
	if fileName == "" {
		return ParsedRefleksFile{}, fmt.Errorf("%w: filename is empty", ErrInvalidRunFile)
	}

	stats, err := parseStats(decoded)
	if err != nil {
		return ParsedRefleksFile{}, fmt.Errorf("%w: stats: %v", ErrInvalidRunFile, err)
	}
	if err := skipSection(decoded); err != nil {
		return ParsedRefleksFile{}, fmt.Errorf("%w: performances: %v", ErrInvalidRunFile, err)
	}
	mouseMetrics, err := parseMouseMetrics(decoded)
	if err != nil {
		return ParsedRefleksFile{}, fmt.Errorf("%w: mouse trace: %v", ErrInvalidRunFile, err)
	}
	env, err := parseEnvironment(decoded)
	if err != nil {
		return ParsedRefleksFile{}, fmt.Errorf("%w: environment: %v", ErrInvalidRunFile, err)
	}
	if err := ensureEOF(decoded); err != nil {
		return ParsedRefleksFile{}, fmt.Errorf("%w: %v", ErrInvalidRunFile, err)
	}

	hasMouseTrace := mouseMetrics.HasTrace || env.TracePoints > 0

	return ParsedRefleksFile{
		FileName:      fileName,
		ScenarioName:  stats.ScenarioName,
		SteamID:       env.SteamID,
		SteamUsername: env.SteamUsername,
		Score:         stats.Score,
		Accuracy:      stats.Accuracy,
		AvgTTKSeconds: stats.AvgTTKSeconds,
		DurationSecs:  stats.DurationSecs,
		SensCM360:     stats.SensCM360,
		HasMouseTrace: hasMouseTrace,
		AvgMouseSpeed: mouseMetrics.AvgSpeed,
		MouseVID:      env.MouseVID,
		MousePID:      env.MousePID,
	}, nil
}

// ---- Section parsers ----

func parseStats(r io.Reader) (parsedStats, error) {
	var size uint32
	if err := binary.Read(r, binary.LittleEndian, &size); err != nil {
		return parsedStats{}, err
	}
	if size == 0 {
		return parsedStats{}, nil
	}
	data := make([]byte, size)
	if _, err := io.ReadFull(r, data); err != nil {
		return parsedStats{}, err
	}
	return extractStatsSummary(data), nil
}

func extractStatsSummary(data []byte) parsedStats {
	var summary []byte
	// Walk RunStatsData: find field 1 (summary message).
	for len(data) > 0 {
		fn, wt, n := protowire.ConsumeTag(data)
		if n < 0 {
			break
		}
		data = data[n:]
		if fn == statsFieldSummary && wt == protowire.BytesType {
			summary, n = protowire.ConsumeBytes(data)
			if n < 0 {
				break
			}
			data = data[n:]
			break
		}
		consumed := protowire.ConsumeFieldValue(fn, wt, data)
		if consumed < 0 {
			break
		}
		data = data[consumed:]
	}
	if len(summary) == 0 {
		return parsedStats{}
	}
	return extractSummaryFields(summary)
}

func extractSummaryFields(data []byte) parsedStats {
	var s parsedStats
	for len(data) > 0 {
		fn, wt, n := protowire.ConsumeTag(data)
		if n < 0 {
			break
		}
		data = data[n:]
		consumed := readSummaryField(fn, wt, data, &s)
		if consumed == -1 {
			break
		}
		data = data[consumed:]
	}
	return s
}

func readSummaryField(fn protowire.Number, wt protowire.Type, data []byte, s *parsedStats) int {
	switch fn {
	case summaryFieldScenario:
		if wt == protowire.BytesType {
			v, n := protowire.ConsumeString(data)
			if n >= 0 {
				s.ScenarioName = strings.TrimSpace(v)
				return n
			}
		}
	case summaryFieldScore:
		if wt == protowire.Fixed64Type {
			bits, n := protowire.ConsumeFixed64(data)
			if n >= 0 {
				v := math.Float64frombits(bits)
				s.Score = float64Ptr(v)
				return n
			}
		}
	case summaryFieldAccuracy:
		if wt == protowire.Fixed64Type {
			bits, n := protowire.ConsumeFixed64(data)
			if n >= 0 {
				v := math.Float64frombits(bits)
				s.Accuracy = float64Ptr(v)
				return n
			}
		}
	case summaryFieldAvgTTK, summaryFieldRealAvgTTK:
		if wt == protowire.Fixed64Type {
			bits, n := protowire.ConsumeFixed64(data)
			if n >= 0 {
				v := math.Float64frombits(bits)
				s.AvgTTKSeconds = float64Ptr(v)
				return n
			}
		}
	case summaryFieldDuration:
		if wt == protowire.Fixed64Type {
			bits, n := protowire.ConsumeFixed64(data)
			if n >= 0 {
				v := math.Float64frombits(bits)
				s.DurationSecs = float64Ptr(v)
				return n
			}
		}
	case summaryFieldCm360:
		if wt == protowire.Fixed64Type {
			bits, n := protowire.ConsumeFixed64(data)
			if n >= 0 {
				v := math.Float64frombits(bits)
				s.SensCM360 = float64Ptr(v)
				return n
			}
		}
	}
	// Field not matched or wrong type: skip it.
	return protowire.ConsumeFieldValue(fn, wt, data)
}

func parseMouseMetrics(r io.Reader) (parsedMouseMetrics, error) {
	var size uint32
	if err := binary.Read(r, binary.LittleEndian, &size); err != nil {
		return parsedMouseMetrics{}, err
	}
	if size == 0 {
		return parsedMouseMetrics{HasTrace: false}, nil
	}
	if size < 4 {
		return parsedMouseMetrics{}, fmt.Errorf("mouse trace section too small: %d", size)
	}
	limited := io.LimitReader(r, int64(size))
	var count uint32
	if err := binary.Read(limited, binary.LittleEndian, &count); err != nil {
		return parsedMouseMetrics{}, err
	}

	out := parsedMouseMetrics{HasTrace: count > 0}
	if count == 0 {
		return out, nil
	}
	if count > maxMousePoints {
		return parsedMouseMetrics{}, fmt.Errorf("too many mouse points: %d", count)
	}

	var prevTS int64
	var prevX int32
	var prevY int32
	var hasPrev bool
	var totalDist float64
	var totalSeconds float64

	for i := uint32(0); i < count; i++ {
		var ts int64
		var x, y, buttons int32
		if err := binary.Read(limited, binary.LittleEndian, &ts); err != nil {
			return parsedMouseMetrics{}, err
		}
		if err := binary.Read(limited, binary.LittleEndian, &x); err != nil {
			return parsedMouseMetrics{}, err
		}
		if err := binary.Read(limited, binary.LittleEndian, &y); err != nil {
			return parsedMouseMetrics{}, err
		}
		if err := binary.Read(limited, binary.LittleEndian, &buttons); err != nil {
			return parsedMouseMetrics{}, err
		}
		_ = buttons

		if hasPrev {
			dtMillis := ts - prevTS
			if dtMillis > 0 {
				dx := float64(x - prevX)
				dy := float64(y - prevY)
				totalDist += math.Hypot(dx, dy)
				totalSeconds += float64(dtMillis) / 1000.0
			}
		}
		prevTS = ts
		prevX = x
		prevY = y
		hasPrev = true
	}

	if totalSeconds > 0 {
		avg := totalDist / totalSeconds
		out.AvgSpeed = &avg
	}

	return out, nil
}

func parseEnvironment(r io.Reader) (parsedEnvironment, error) {
	var size uint32
	if err := binary.Read(r, binary.LittleEndian, &size); err != nil {
		return parsedEnvironment{}, err
	}
	if size == 0 {
		return parsedEnvironment{}, nil
	}
	data := make([]byte, size)
	if _, err := io.ReadFull(r, data); err != nil {
		return parsedEnvironment{}, err
	}
	return extractEnvironment(data), nil
}

func extractEnvironment(data []byte) parsedEnvironment {
	var env parsedEnvironment
	for len(data) > 0 {
		fn, wt, n := protowire.ConsumeTag(data)
		if n < 0 {
			break
		}
		data = data[n:]
		consumed := readEnvironmentField(fn, wt, data, &env)
		if consumed == -1 {
			break
		}
		data = data[consumed:]
	}
	return env
}

func readEnvironmentField(fn protowire.Number, wt protowire.Type, data []byte, env *parsedEnvironment) int {
	switch fn {
	case envFieldSteamID:
		if wt == protowire.BytesType {
			v, n := protowire.ConsumeString(data)
			if n >= 0 {
				env.SteamID = strings.TrimSpace(v)
				return n
			}
		}
	case envFieldPersonaName:
		if wt == protowire.BytesType {
			v, n := protowire.ConsumeString(data)
			if n >= 0 {
				env.SteamUsername = strings.TrimSpace(v)
				return n
			}
		}
	case envFieldMouseVID:
		if wt == protowire.BytesType {
			v, n := protowire.ConsumeString(data)
			if n >= 0 {
				env.MouseVID = strings.TrimSpace(v)
				return n
			}
		}
	case envFieldMousePID:
		if wt == protowire.BytesType {
			v, n := protowire.ConsumeString(data)
			if n >= 0 {
				env.MousePID = strings.TrimSpace(v)
				return n
			}
		}
	case envFieldTracePoints:
		if wt == protowire.VarintType {
			v, n := protowire.ConsumeVarint(data)
			if n >= 0 {
				env.TracePoints = int32(v)
				return n
			}
		}
	}
	return protowire.ConsumeFieldValue(fn, wt, data)
}

// ---- Section skip helper ----

func skipSection(r io.Reader) error {
	var size uint32
	if err := binary.Read(r, binary.LittleEndian, &size); err != nil {
		return err
	}
	if size == 0 {
		return nil
	}
	_, err := io.CopyN(io.Discard, r, int64(size))
	return err
}

// ---- zstd decoder ----

func newPayloadDecoder(r io.Reader, compression uint8) (io.Reader, func(), error) {
	switch compression {
	case runCompressionNone:
		return r, func() {}, nil
	case runCompressionZstd:
		dec, err := zstd.NewReader(r)
		if err != nil {
			return nil, nil, err
		}
		return dec, dec.Close, nil
	default:
		return nil, nil, fmt.Errorf("unsupported compression %d", compression)
	}
}

func readString(r io.Reader, maxLen uint32) (string, error) {
	n, err := readUint32(r)
	if err != nil {
		return "", err
	}
	if n > maxLen {
		return "", fmt.Errorf("string too large: %d", n)
	}
	if n == 0 {
		return "", nil
	}
	buf := make([]byte, n)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

func ensureEOF(r io.Reader) error {
	var b [1]byte
	n, err := r.Read(b[:])
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return err
	}
	if n > 0 {
		return fmt.Errorf("trailing payload bytes")
	}
	return nil
}

func readUint32(r io.Reader) (uint32, error) {
	var v uint32
	if err := binary.Read(r, binary.LittleEndian, &v); err != nil {
		return 0, err
	}
	return v, nil
}

func asFloat64(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case int64:
		return float64(t), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		if err != nil {
			return 0, false
		}
		return f, true
	default:
		return 0, false
	}
}

func float64Ptr(v float64) *float64 {
	copy := v
	return &copy
}
