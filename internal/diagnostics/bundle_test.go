package diagnostics

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"errors"
	"strings"
	"testing"
)

type testArchiveEntry struct {
	name     string
	body     []byte
	typeflag byte
}

func TestValidateBundle(t *testing.T) {
	validEntries := []testArchiveEntry{
		{name: "manifest.json", body: []byte(`{"schema_version":1}`)},
		{name: "logs.jsonl", body: []byte("{}\n")},
	}

	tests := []struct {
		name    string
		entries []testArchiveEntry
		limits  BundleLimits
		mutate  func([]byte) []byte
		wantErr error
	}{
		{
			name:    "happy path",
			entries: validEntries,
		},
		{
			name: "wrong first entry",
			entries: []testArchiveEntry{
				{name: "device.json", body: []byte(`{}`)},
				{name: "manifest.json", body: []byte(`{}`)},
			},
			wantErr: ErrInvalidBundle,
		},
		{
			name: "disallowed name",
			entries: []testArchiveEntry{
				{name: "manifest.json", body: []byte(`{}`)},
				{name: "extra.txt", body: []byte("nope")},
			},
			wantErr: ErrInvalidBundle,
		},
		{
			name: "traversal name",
			entries: []testArchiveEntry{
				{name: "manifest.json", body: []byte(`{}`)},
				{name: "../logs.jsonl", body: []byte("nope")},
			},
			wantErr: ErrInvalidBundle,
		},
		{
			name: "ratio bomb",
			entries: []testArchiveEntry{
				{name: "manifest.json", body: []byte(`{}`)},
				{name: "logs.jsonl", body: []byte(strings.Repeat("a", 1024*1024))},
			},
			limits: BundleLimits{
				MaxUncompressedBytes: 2 * 1024 * 1024,
			},
			wantErr: ErrCompressionRatio,
		},
		{
			name: "over count",
			entries: append([]testArchiveEntry{
				{name: "manifest.json", body: []byte(`{}`)},
			}, repeatedEntries("logs.jsonl", 16)...),
			wantErr: ErrTooManyEntries,
		},
		{
			name:    "oversize",
			entries: validEntries,
			limits:  BundleLimits{MaxCompressedBytes: 64},
			wantErr: ErrCompressedTooLarge,
		},
		{
			name:    "truncated gzip",
			entries: validEntries,
			mutate: func(raw []byte) []byte {
				return raw[:len(raw)-8]
			},
			wantErr: ErrInvalidBundle,
		},
		{
			name:    "rejects trailing tar data inside gzip stream",
			entries: validEntries,
			mutate: func([]byte) []byte {
				return buildTestArchiveWithPostTarPayload(t, validEntries, []byte("hidden"))
			},
			wantErr: ErrInvalidBundle,
		},
		{
			name:    "rejects concatenated gzip member",
			entries: validEntries,
			mutate: func(raw []byte) []byte {
				return append(raw, buildTestGzipMember(t, bytes.Repeat([]byte{0}, 1024))...)
			},
			wantErr: ErrInvalidBundle,
		},
		{
			name: "rejects symlinks",
			entries: []testArchiveEntry{
				{name: "manifest.json", body: []byte(`{}`)},
				{name: "logs.jsonl", typeflag: tar.TypeSymlink},
			},
			wantErr: ErrInvalidBundle,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := buildTestArchive(t, tt.entries)
			if tt.mutate != nil {
				raw = tt.mutate(raw)
			}
			info, err := ValidateBundle(bytes.NewReader(raw), tt.limits)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("ValidateBundle error = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateBundle error = %v", err)
			}
			if info.CompressedBytes != int64(len(raw)) {
				t.Fatalf("compressed bytes = %d, want %d", info.CompressedBytes, len(raw))
			}
			sum := sha256.Sum256(raw)
			if info.SHA256 != fmtHex(sum[:]) {
				t.Fatalf("sha256 = %s, want %s", info.SHA256, fmtHex(sum[:]))
			}
			if got, want := strings.Join(info.Entries, ","), "manifest.json,logs.jsonl"; got != want {
				t.Fatalf("entries = %s, want %s", got, want)
			}
			if info.UncompressedBytes != int64(len(validEntries[0].body)+len(validEntries[1].body)) {
				t.Fatalf("uncompressed bytes = %d", info.UncompressedBytes)
			}
		})
	}
}

func repeatedEntries(name string, count int) []testArchiveEntry {
	entries := make([]testArchiveEntry, 0, count)
	for i := 0; i < count; i++ {
		entries = append(entries, testArchiveEntry{name: name, body: []byte("{}\n")})
	}
	return entries
}

func buildTestArchive(t *testing.T, entries []testArchiveEntry) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, entry := range entries {
		typeflag := entry.typeflag
		if typeflag == 0 {
			typeflag = tar.TypeReg
		}
		hdr := &tar.Header{
			Name:     entry.name,
			Mode:     0o600,
			Size:     int64(len(entry.body)),
			Typeflag: typeflag,
		}
		if typeflag != tar.TypeReg && typeflag != tar.TypeRegA {
			hdr.Size = 0
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write header: %v", err)
		}
		if hdr.Size > 0 {
			if _, err := tw.Write(entry.body); err != nil {
				t.Fatalf("write body: %v", err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buf.Bytes()
}

func buildTestArchiveWithPostTarPayload(t *testing.T, entries []testArchiveEntry, trailing []byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, entry := range entries {
		typeflag := entry.typeflag
		if typeflag == 0 {
			typeflag = tar.TypeReg
		}
		hdr := &tar.Header{
			Name:     entry.name,
			Mode:     0o600,
			Size:     int64(len(entry.body)),
			Typeflag: typeflag,
		}
		if typeflag != tar.TypeReg && typeflag != tar.TypeRegA {
			hdr.Size = 0
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("write header: %v", err)
		}
		if hdr.Size > 0 {
			if _, err := tw.Write(entry.body); err != nil {
				t.Fatalf("write body: %v", err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if _, err := gz.Write(trailing); err != nil {
		t.Fatalf("write trailing gzip payload: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buf.Bytes()
}

func buildTestGzipMember(t *testing.T, payload []byte) []byte {
	t.Helper()

	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(payload); err != nil {
		t.Fatalf("write gzip payload: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buf.Bytes()
}

func fmtHex(raw []byte) string {
	const table = "0123456789abcdef"
	out := make([]byte, len(raw)*2)
	for i, b := range raw {
		out[i*2] = table[b>>4]
		out[i*2+1] = table[b&0x0f]
	}
	return string(out)
}
