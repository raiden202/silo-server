package jellycompat

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProxyImageDefaultsToRevalidatingCachePolicy(t *testing.T) {
	rec := httptest.NewRecorder()
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Header: http.Header{
			"Content-Type":  []string{"image/jpeg"},
			"ETag":          []string{`"abc"`},
			"Last-Modified": []string{"Tue, 12 May 2026 12:00:00 GMT"},
			"Expires":       []string{"Tue, 12 May 2026 12:05:00 GMT"},
		},
		Body: io.NopCloser(strings.NewReader("image-bytes")),
	}

	proxyImage(rec, resp)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != defaultImageProxyCacheControl {
		t.Fatalf("Cache-Control = %q, want %q", got, defaultImageProxyCacheControl)
	}
	if got := rec.Header().Get("ETag"); got != `"abc"` {
		t.Fatalf("ETag = %q", got)
	}
	if strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Fatalf("Cache-Control unexpectedly immutable: %q", rec.Header().Get("Cache-Control"))
	}
}

func TestProxyImageRelaysNotModified(t *testing.T) {
	rec := httptest.NewRecorder()
	resp := &http.Response{
		StatusCode: http.StatusNotModified,
		Header: http.Header{
			"ETag":          []string{`"abc"`},
			"Cache-Control": []string{"public, max-age=60"},
		},
		Body: io.NopCloser(strings.NewReader("")),
	}

	proxyImage(rec, resp)

	if rec.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want 304", rec.Code)
	}
	if got := rec.Header().Get("ETag"); got != `"abc"` {
		t.Fatalf("ETag = %q", got)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("304 body length = %d, want 0", rec.Body.Len())
	}
}

func TestProxyImageURLForwardsConditionalHeaders(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("If-None-Match"); got != `"abc"` {
			t.Fatalf("If-None-Match = %q", got)
		}
		if got := r.Header.Get("If-Modified-Since"); got != "Tue, 12 May 2026 12:00:00 GMT" {
			t.Fatalf("If-Modified-Since = %q", got)
		}
		w.Header().Set("ETag", `"abc"`)
		w.WriteHeader(http.StatusNotModified)
	}))
	defer upstream.Close()

	h := &ImagesHandler{httpClient: upstream.Client()}
	req := httptest.NewRequest(http.MethodGet, "/Items/1/Images/Primary", nil)
	req.Header.Set("If-None-Match", `"abc"`)
	req.Header.Set("If-Modified-Since", "Tue, 12 May 2026 12:00:00 GMT")
	rec := httptest.NewRecorder()

	h.proxyImageURL(rec, req, upstream.URL)

	if rec.Code != http.StatusNotModified {
		t.Fatalf("status = %d, want 304", rec.Code)
	}
}
