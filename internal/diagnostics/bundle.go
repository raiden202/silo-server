package diagnostics

import (
	"archive/tar"
	"bytes"
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
	// Standard tar writers (GNU tar, Python tarfile, Apache Commons Compress)
	// pad the archive with zero blocks to a record boundary, 10240 bytes by
	// default; 64 KiB also covers writers configured with larger blocking
	// factors.
	maxTarTrailingPaddingBytes = 64 * 1024
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
	CompressedBytes int64
	// UncompressedBytes is the total size of the decompressed tar stream —
	// headers, entry payloads, end-of-archive marker, and padding — matching
	// what a client counts on the way into its gzip writer (and `gzip -l`).
	UncompressedBytes int64
	SHA256            string
	Entries           []string
	// EmbeddedManifest is the raw bytes of the archive's first entry
	// (manifest.json). Per the bundle contract this is the part-1 manifest with
	// the `archive` object removed; Ingest compares the two so a stored archive
	// can never carry a manifest that disagrees with the accepted report.
	EmbeddedManifest []byte
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

	uncompressed := &uncompressedCounter{r: gz}
	tr := tar.NewReader(uncompressed)
	info := BundleInfo{
		Entries: make([]string, 0, len(allowedBundleEntries)),
	}
	buffer := make([]byte, defaultBundleReadBufferSize)
	var payloadBytes int64

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
		isManifestEntry := len(info.Entries) == 0
		if isManifestEntry && name != "manifest.json" {
			return BundleInfo{}, fmt.Errorf("%w: first entry must be manifest.json", ErrInvalidBundle)
		}
		info.Entries = append(info.Entries, name)

		if hdr.Size > limits.MaxEntryBytes {
			return BundleInfo{}, ErrEntryTooLarge
		}
		if isManifestEntry && hdr.Size > MaxManifestBytes {
			return BundleInfo{}, fmt.Errorf("%w: embedded manifest exceeds %d bytes", ErrInvalidBundle, MaxManifestBytes)
		}
		if payloadBytes+hdr.Size > limits.MaxUncompressedBytes {
			return BundleInfo{}, ErrUncompressedTooLarge
		}

		var manifestCapture *bytes.Buffer
		var capture io.Writer
		if isManifestEntry {
			manifestCapture = &bytes.Buffer{}
			manifestCapture.Grow(int(hdr.Size))
			capture = manifestCapture
		}
		entryBytes, err := copyTarEntry(tr, buffer, limits, metered, &payloadBytes, capture)
		if err != nil {
			return BundleInfo{}, err
		}
		if entryBytes != hdr.Size {
			return BundleInfo{}, fmt.Errorf("%w: entry size mismatch for %s", ErrInvalidBundle, name)
		}
		if manifestCapture != nil {
			info.EmbeddedManifest = manifestCapture.Bytes()
		}
	}

	if len(info.Entries) == 0 {
		return BundleInfo{}, fmt.Errorf("%w: empty archive", ErrInvalidBundle)
	}
	if err := rejectPostTarData(uncompressed, buffer); err != nil {
		return BundleInfo{}, err
	}
	if err := gz.Close(); err != nil {
		return BundleInfo{}, classifyBundleReadError(err)
	}
	gzipClosed = true
	if err := rejectTrailingCompressedData(metered); err != nil {
		return BundleInfo{}, err
	}
	if uncompressed.count > limits.MaxUncompressedBytes {
		return BundleInfo{}, ErrUncompressedTooLarge
	}
	if ratioExceeded(uncompressed.count, metered.Count(), limits.MaxCompressionRatio) {
		return BundleInfo{}, ErrCompressionRatio
	}

	info.UncompressedBytes = uncompressed.count
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
	// PAX/GNU extension records are consumed by archive/tar before the next
	// header surfaces, so bytes carried in them never reach the allowlist and
	// count checks. The contract fixtures are plain USTAR; reject anything else
	// rather than let extension records smuggle data past validation.
	if hdr.Format == tar.FormatPAX || hdr.Format == tar.FormatGNU {
		return "", fmt.Errorf("%w: unsupported tar format", ErrInvalidBundle)
	}
	if len(hdr.PAXRecords) > 0 {
		return "", fmt.Errorf("%w: unsupported tar extension records", ErrInvalidBundle)
	}
	if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA {
		return "", fmt.Errorf("%w: unsupported tar entry type", ErrInvalidBundle)
	}
	if hdr.Size < 0 {
		return "", fmt.Errorf("%w: negative tar entry size", ErrInvalidBundle)
	}
	// Reject names that are not already trimmed instead of normalizing them:
	// otherwise a padded name like "manifest.json " would be recorded as the
	// allowlisted entry while the archive stores a different literal member.
	name := hdr.Name
	if name != strings.TrimSpace(name) {
		return "", fmt.Errorf("%w: unsafe entry name", ErrInvalidBundle)
	}
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

func copyTarEntry(tr *tar.Reader, buffer []byte, limits BundleLimits, metered *compressedMeter, total *int64, capture io.Writer) (int64, error) {
	var entryBytes int64
	for {
		n, err := tr.Read(buffer)
		if n > 0 {
			if capture != nil {
				_, _ = capture.Write(buffer[:n])
			}
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

func rejectPostTarData(r io.Reader, buffer []byte) error {
	var padding int64
	for {
		n, err := r.Read(buffer)
		if errors.Is(err, ErrCompressedTooLarge) {
			return ErrCompressedTooLarge
		}
		if n > 0 {
			for _, b := range buffer[:n] {
				if b != 0 {
					return fmt.Errorf("%w: trailing data after tar archive", ErrInvalidBundle)
				}
			}
			padding += int64(n)
			if padding > maxTarTrailingPaddingBytes {
				return fmt.Errorf("%w: excessive padding after tar archive", ErrInvalidBundle)
			}
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

// uncompressedCounter tracks the total number of decompressed bytes read out
// of the gzip stream, including tar headers, end-of-archive blocks, and
// record padding.
type uncompressedCounter struct {
	r     io.Reader
	count int64
}

func (c *uncompressedCounter) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.count += int64(n)
	return n, err
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
