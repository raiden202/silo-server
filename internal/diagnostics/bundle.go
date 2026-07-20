package diagnostics

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"
	"math"
	"path"
	"strings"

	"github.com/Silo-Server/silo-server/internal/diagnostics/contract"
)

const (
	DefaultMaxBundleEntries     = 16
	DefaultMaxCompressionRatio  = int64(200)
	defaultBundleReadBufferSize = 32 * 1024
)

var (
	ErrCompressedTooLarge   = errors.New("diagnostics bundle compressed size exceeds limit")
	ErrUncompressedTooLarge = errors.New("diagnostics bundle uncompressed size exceeds limit")
	ErrEntryTooLarge        = errors.New("diagnostics bundle entry size exceeds limit")
	ErrTooManyEntries       = errors.New("diagnostics bundle has too many entries")
	ErrInvalidBundle        = errors.New("invalid diagnostics bundle")
	ErrCompressionRatio     = errors.New("diagnostics bundle compression ratio exceeds limit")
)

type BundleLimits struct {
	MaxCompressedBytes   int64
	MaxUncompressedBytes int64
	MaxEntryBytes        int64
	MaxEntries           int
	MaxCompressionRatio  int64
}

type BundleInfo struct {
	CompressedBytes   int64
	UncompressedBytes int64
	SHA256            string
	Entries           []string
}

var allowedBundleEntries = allowlistMap(contract.ArchiveEntryAllowlist)

func ValidateBundle(r io.Reader, limits BundleLimits) (BundleInfo, error) {
	limits = normalizeBundleLimits(limits)
	metered := newCompressedMeter(r, limits.MaxCompressedBytes)

	gz, err := gzip.NewReader(metered)
	if err != nil {
		if isBundleUploadAbortError(err) {
			return BundleInfo{}, err
		}
		if errors.Is(err, ErrCompressedTooLarge) {
			return BundleInfo{}, ErrCompressedTooLarge
		}
		return BundleInfo{}, fmt.Errorf("%w: open gzip: %v", ErrInvalidBundle, err)
	}
	gz.Multistream(false)
	gzipClosed := false
	defer func() {
		if !gzipClosed {
			_ = gz.Close()
		}
	}()

	tr := tar.NewReader(gz)
	info := BundleInfo{
		Entries: make([]string, 0, len(allowedBundleEntries)),
	}
	buffer := make([]byte, defaultBundleReadBufferSize)

	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return BundleInfo{}, classifyBundleReadError(err)
		}

		if len(info.Entries) >= limits.MaxEntries {
			return BundleInfo{}, ErrTooManyEntries
		}

		name, err := validateEntryHeader(hdr)
		if err != nil {
			return BundleInfo{}, err
		}
		if len(info.Entries) == 0 && name != "manifest.json" {
			return BundleInfo{}, fmt.Errorf("%w: first entry must be manifest.json", ErrInvalidBundle)
		}
		info.Entries = append(info.Entries, name)

		if hdr.Size > limits.MaxEntryBytes {
			return BundleInfo{}, ErrEntryTooLarge
		}
		if info.UncompressedBytes+hdr.Size > limits.MaxUncompressedBytes {
			return BundleInfo{}, ErrUncompressedTooLarge
		}

		entryBytes, err := copyTarEntry(tr, buffer, limits, metered, &info.UncompressedBytes)
		if err != nil {
			return BundleInfo{}, err
		}
		if entryBytes != hdr.Size {
			return BundleInfo{}, fmt.Errorf("%w: entry size mismatch for %s", ErrInvalidBundle, name)
		}
	}

	if len(info.Entries) == 0 {
		return BundleInfo{}, fmt.Errorf("%w: empty archive", ErrInvalidBundle)
	}
	if err := rejectPostTarData(gz, buffer); err != nil {
		return BundleInfo{}, err
	}
	if err := gz.Close(); err != nil {
		return BundleInfo{}, classifyBundleReadError(err)
	}
	gzipClosed = true
	if err := rejectTrailingCompressedData(metered); err != nil {
		return BundleInfo{}, err
	}
	if ratioExceeded(info.UncompressedBytes, metered.Count(), limits.MaxCompressionRatio) {
		return BundleInfo{}, ErrCompressionRatio
	}

	info.CompressedBytes = metered.Count()
	info.SHA256 = hex.EncodeToString(metered.Sum())
	return info, nil
}

func normalizeBundleLimits(limits BundleLimits) BundleLimits {
	if limits.MaxCompressedBytes <= 0 {
		limits.MaxCompressedBytes = DefaultMaxBundleBytes
	}
	if limits.MaxUncompressedBytes <= 0 {
		limits.MaxUncompressedBytes = DefaultMaxUncompressed
	}
	if limits.MaxEntryBytes <= 0 || limits.MaxEntryBytes > limits.MaxUncompressedBytes {
		limits.MaxEntryBytes = limits.MaxUncompressedBytes
	}
	if limits.MaxEntries <= 0 {
		limits.MaxEntries = DefaultMaxBundleEntries
	}
	if limits.MaxCompressionRatio <= 0 {
		limits.MaxCompressionRatio = DefaultMaxCompressionRatio
	}
	return limits
}

func validateEntryHeader(hdr *tar.Header) (string, error) {
	if hdr == nil {
		return "", fmt.Errorf("%w: missing tar header", ErrInvalidBundle)
	}
	if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
		return "", fmt.Errorf("%w: unsupported tar entry type", ErrInvalidBundle)
	}
	if hdr.Size < 0 {
		return "", fmt.Errorf("%w: negative tar entry size", ErrInvalidBundle)
	}
	name := strings.TrimSpace(hdr.Name)
	if name == "" || strings.Contains(name, "\\") || strings.HasPrefix(name, "/") || path.IsAbs(name) {
		return "", fmt.Errorf("%w: unsafe entry name", ErrInvalidBundle)
	}
	clean := path.Clean(name)
	if clean == "." || clean != name || strings.HasPrefix(clean, "../") || strings.Contains(clean, "/../") {
		return "", fmt.Errorf("%w: unsafe entry name", ErrInvalidBundle)
	}
	if _, ok := allowedBundleEntries[name]; !ok {
		return "", fmt.Errorf("%w: disallowed entry %q", ErrInvalidBundle, name)
	}
	return name, nil
}

func copyTarEntry(tr *tar.Reader, buffer []byte, limits BundleLimits, metered *compressedMeter, total *int64) (int64, error) {
	var entryBytes int64
	for {
		n, err := tr.Read(buffer)
		if n > 0 {
			entryBytes += int64(n)
			if entryBytes > limits.MaxEntryBytes {
				return entryBytes, ErrEntryTooLarge
			}
			*total += int64(n)
			if *total > limits.MaxUncompressedBytes {
				return entryBytes, ErrUncompressedTooLarge
			}
			if ratioExceeded(*total, metered.Count(), limits.MaxCompressionRatio) {
				return entryBytes, ErrCompressionRatio
			}
		}
		if errors.Is(err, io.EOF) {
			return entryBytes, nil
		}
		if err != nil {
			return entryBytes, classifyBundleReadError(err)
		}
	}
}

func rejectPostTarData(gz *gzip.Reader, buffer []byte) error {
	for {
		n, err := gz.Read(buffer)
		if errors.Is(err, ErrCompressedTooLarge) {
			return ErrCompressedTooLarge
		}
		if n > 0 {
			return fmt.Errorf("%w: trailing data after tar archive", ErrInvalidBundle)
		}
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return classifyBundleReadError(err)
		}
	}
}

func rejectTrailingCompressedData(metered *compressedMeter) error {
	var one [1]byte
	n, err := metered.Read(one[:])
	if errors.Is(err, ErrCompressedTooLarge) {
		return ErrCompressedTooLarge
	}
	if n > 0 {
		return fmt.Errorf("%w: trailing data after gzip stream", ErrInvalidBundle)
	}
	if errors.Is(err, io.EOF) {
		return nil
	}
	if err != nil {
		return classifyBundleReadError(err)
	}
	return nil
}

func ratioExceeded(uncompressed, compressed, maxRatio int64) bool {
	if compressed <= 0 || maxRatio <= 0 {
		return false
	}
	if compressed > math.MaxInt64/maxRatio {
		return false
	}
	return uncompressed > compressed*maxRatio
}

func classifyBundleReadError(err error) error {
	if isBundleUploadAbortError(err) {
		return err
	}
	if errors.Is(err, ErrCompressedTooLarge) {
		return ErrCompressedTooLarge
	}
	if errors.Is(err, io.ErrUnexpectedEOF) {
		return fmt.Errorf("%w: truncated archive", ErrInvalidBundle)
	}
	return fmt.Errorf("%w: %v", ErrInvalidBundle, err)
}

type bundleUploadAbortError struct {
	err error
}

func (e *bundleUploadAbortError) Error() string {
	if e == nil || e.err == nil {
		return "diagnostics bundle upload aborted"
	}
	return e.err.Error()
}

func (e *bundleUploadAbortError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func isBundleUploadAbortError(err error) bool {
	var abortErr *bundleUploadAbortError
	return errors.As(err, &abortErr)
}

func allowlistMap(entries []string) map[string]struct{} {
	allowed := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		allowed[entry] = struct{}{}
	}
	return allowed
}

type compressedMeter struct {
	r     io.Reader
	max   int64
	n     int64
	hash  hash.Hash
	limit bool
}

func newCompressedMeter(r io.Reader, max int64) *compressedMeter {
	return &compressedMeter{
		r:     r,
		max:   max,
		hash:  sha256.New(),
		limit: max > 0,
	}
}

func (m *compressedMeter) Read(p []byte) (int, error) {
	if m.limit {
		remaining := m.max - m.n
		if remaining < 0 {
			return 0, ErrCompressedTooLarge
		}
		if int64(len(p)) > remaining+1 {
			p = p[:int(remaining+1)]
		}
	}
	n, err := m.r.Read(p)
	if n > 0 {
		m.n += int64(n)
		_, _ = m.hash.Write(p[:n])
		if m.limit && m.n > m.max {
			return n, ErrCompressedTooLarge
		}
	}
	return n, err
}

func (m *compressedMeter) ReadByte() (byte, error) {
	var one [1]byte
	n, err := m.Read(one[:])
	if n == 1 {
		if errors.Is(err, io.EOF) {
			err = nil
		}
		return one[0], err
	}
	return 0, err
}

func (m *compressedMeter) Count() int64 {
	return m.n
}

func (m *compressedMeter) Sum() []byte {
	return m.hash.Sum(nil)
}
