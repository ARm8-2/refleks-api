package runsync

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
)

const (
	runMagic                 = "RFLK"
	runVersion         uint8 = 1
	runCompressionNone uint8 = 0
	runCompressionZstd uint8 = 1

	runHeaderSize   = 4 + 1 + 1 + 8
	runChecksumSize = 8

	statTypeString uint8 = 1
	statTypeInt    uint8 = 2
	statTypeFloat  uint8 = 3
	statTypeBool   uint8 = 4

	maxFileNameBytes = 1024
	maxStringBytes   = 1 << 20
	maxStatsEntries  = 50000
	maxEventRows     = 500000
	maxEventCols     = 128
	maxMousePoints   = 20000000
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
		return ParsedRefleksFile{}, fmt.Errorf("%w: unsupported version %d", ErrInvalidRunFile, version)
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
	parsed.EpochMilli = epoch
	parsed.Compression = compression
	parsed.FormatVersion = version
	parsed.Checksum = wantChecksum
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
	if err := skipEvents(decoded); err != nil {
		return ParsedRefleksFile{}, fmt.Errorf("%w: events: %v", ErrInvalidRunFile, err)
	}
	mouseMetrics, err := parseMouseMetrics(decoded)
	if err != nil {
		return ParsedRefleksFile{}, fmt.Errorf("%w: mouse trace: %v", ErrInvalidRunFile, err)
	}
	env, err := parseRunEnvironment(decoded)
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

func parseStats(r io.Reader) (parsedStats, error) {
	count, err := readUint32(r)
	if err != nil {
		return parsedStats{}, err
	}
	if count > maxStatsEntries {
		return parsedStats{}, fmt.Errorf("too many stats entries: %d", count)
	}

	out := parsedStats{}
	for i := uint32(0); i < count; i++ {
		key, err := readString(r, maxStringBytes)
		if err != nil {
			return parsedStats{}, err
		}
		t, err := readUint8(r)
		if err != nil {
			return parsedStats{}, err
		}

		var value any
		switch t {
		case statTypeString:
			v, err := readString(r, maxStringBytes)
			if err != nil {
				return parsedStats{}, err
			}
			value = v
		case statTypeInt:
			v, err := readInt64(r)
			if err != nil {
				return parsedStats{}, err
			}
			value = v
		case statTypeFloat:
			v, err := readFloat64(r)
			if err != nil {
				return parsedStats{}, err
			}
			value = v
		case statTypeBool:
			if _, err := readUint8(r); err != nil {
				return parsedStats{}, err
			}
			continue
		default:
			return parsedStats{}, fmt.Errorf("unknown stat type: %d", t)
		}

		normalizedKey := strings.ToLower(strings.TrimSpace(key))
		switch normalizedKey {
		case "scenario":
			if s, ok := value.(string); ok {
				out.ScenarioName = strings.TrimSpace(s)
			}
		case "score":
			if v, ok := asFloat64(value); ok {
				out.Score = float64Ptr(v)
			}
		case "accuracy":
			if v, ok := asFloat64(value); ok {
				out.Accuracy = float64Ptr(v)
			}
		case "real avg ttk", "avg ttk":
			if v, ok := asFloat64(value); ok {
				out.AvgTTKSeconds = float64Ptr(v)
			}
		case "duration":
			if v, ok := asFloat64(value); ok {
				out.DurationSecs = float64Ptr(v)
			}
		case "cm/360":
			if v, ok := asFloat64(value); ok {
				out.SensCM360 = float64Ptr(v)
			}
		}
	}

	return out, nil
}

func skipEvents(r io.Reader) error {
	rows, err := readUint32(r)
	if err != nil {
		return err
	}
	if rows > maxEventRows {
		return fmt.Errorf("too many event rows: %d", rows)
	}
	for i := uint32(0); i < rows; i++ {
		cols, err := readUint32(r)
		if err != nil {
			return err
		}
		if cols > maxEventCols {
			return fmt.Errorf("too many event columns: %d", cols)
		}
		for j := uint32(0); j < cols; j++ {
			if _, err := readString(r, maxStringBytes); err != nil {
				return err
			}
		}
	}
	return nil
}

func parseMouseMetrics(r io.Reader) (parsedMouseMetrics, error) {
	count, err := readUint32(r)
	if err != nil {
		return parsedMouseMetrics{}, err
	}
	if count > maxMousePoints {
		return parsedMouseMetrics{}, fmt.Errorf("too many mouse points: %d", count)
	}

	out := parsedMouseMetrics{HasTrace: count > 0}
	if count == 0 {
		return out, nil
	}

	var prevTS int64
	var prevX int32
	var prevY int32
	var hasPrev bool
	var totalDist float64
	var totalSeconds float64

	for i := uint32(0); i < count; i++ {
		ts, err := readInt64(r)
		if err != nil {
			return parsedMouseMetrics{}, err
		}
		x, err := readInt32(r)
		if err != nil {
			return parsedMouseMetrics{}, err
		}
		y, err := readInt32(r)
		if err != nil {
			return parsedMouseMetrics{}, err
		}
		if _, err := readInt32(r); err != nil {
			return parsedMouseMetrics{}, err
		}

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

func parseRunEnvironment(r io.Reader) (parsedEnvironment, error) {
	var err error
	readEnvString := func() (string, error) {
		return readString(r, maxStringBytes)
	}

	if _, err = readEnvString(); err != nil {
		return parsedEnvironment{}, err
	}
	if _, err = readEnvString(); err != nil {
		return parsedEnvironment{}, err
	}
	if _, err = readEnvString(); err != nil {
		return parsedEnvironment{}, err
	}
	if _, err = readEnvString(); err != nil {
		return parsedEnvironment{}, err
	}
	if _, err = readEnvString(); err != nil {
		return parsedEnvironment{}, err
	}
	steamID, err := readEnvString()
	if err != nil {
		return parsedEnvironment{}, err
	}
	steamUsername, err := readEnvString()
	if err != nil {
		return parsedEnvironment{}, err
	}
	if _, err = readEnvString(); err != nil {
		return parsedEnvironment{}, err
	}
	if _, err = readInt32(r); err != nil {
		return parsedEnvironment{}, err
	}
	if _, err = readEnvString(); err != nil {
		return parsedEnvironment{}, err
	}
	if _, err = readInt32(r); err != nil {
		return parsedEnvironment{}, err
	}
	if _, err = readFloat64(r); err != nil {
		return parsedEnvironment{}, err
	}
	if _, err = readInt32(r); err != nil {
		return parsedEnvironment{}, err
	}
	if _, err = readInt32(r); err != nil {
		return parsedEnvironment{}, err
	}
	if _, err = readUint8(r); err != nil {
		return parsedEnvironment{}, err
	}
	if _, err = readEnvString(); err != nil {
		return parsedEnvironment{}, err
	}
	mouseVID, err := readEnvString()
	if err != nil {
		return parsedEnvironment{}, err
	}
	mousePID, err := readEnvString()
	if err != nil {
		return parsedEnvironment{}, err
	}
	if _, err = readEnvString(); err != nil {
		return parsedEnvironment{}, err
	}
	if _, err = readEnvString(); err != nil {
		return parsedEnvironment{}, err
	}
	tracePoints, err := readInt32(r)
	if err != nil {
		return parsedEnvironment{}, err
	}
	if _, err = readFloat64(r); err != nil {
		return parsedEnvironment{}, err
	}
	if _, err = readInt32(r); err != nil {
		return parsedEnvironment{}, err
	}

	return parsedEnvironment{
		SteamID:       strings.TrimSpace(steamID),
		SteamUsername: strings.TrimSpace(steamUsername),
		MouseVID:      strings.TrimSpace(mouseVID),
		MousePID:      strings.TrimSpace(mousePID),
		TracePoints:   tracePoints,
	}, nil
}

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

func readUint8(r io.Reader) (uint8, error) {
	var v uint8
	if err := binary.Read(r, binary.LittleEndian, &v); err != nil {
		return 0, err
	}
	return v, nil
}

func readInt32(r io.Reader) (int32, error) {
	var v int32
	if err := binary.Read(r, binary.LittleEndian, &v); err != nil {
		return 0, err
	}
	return v, nil
}

func readInt64(r io.Reader) (int64, error) {
	var v int64
	if err := binary.Read(r, binary.LittleEndian, &v); err != nil {
		return 0, err
	}
	return v, nil
}

func readFloat64(r io.Reader) (float64, error) {
	var v float64
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
