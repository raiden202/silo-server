package jellycompat

import (
	"bytes"
	"crypto/sha1"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
)

const defaultImageProxyCacheControl = "public, max-age=300, must-revalidate"

func proxyImage(w http.ResponseWriter, resp *http.Response) {
	defer resp.Body.Close()
	copyImageProxyHeaders(w.Header(), resp.Header)
	if resp.StatusCode == http.StatusNotModified {
		w.WriteHeader(http.StatusNotModified)
		return
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		writeError(w, http.StatusBadGateway, "UpstreamError", "Failed to load image")
		return
	}
	w.WriteHeader(http.StatusOK)
	_, _ = io.Copy(w, resp.Body)
}

func copyImageProxyHeaders(dst, src http.Header) {
	for _, name := range []string{"Content-Type", "Cache-Control", "ETag", "Last-Modified", "Expires"} {
		values := src.Values(name)
		if len(values) == 0 {
			values = src[name]
		}
		if len(values) == 0 {
			continue
		}
		dst.Del(name)
		for _, value := range values {
			dst.Add(name, value)
		}
	}
	if dst.Get("Cache-Control") == "" {
		dst.Set("Cache-Control", defaultImageProxyCacheControl)
	}
}

func placeholderAvatarPNG(seed string) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, 128, 128))
	sum := sha1.Sum([]byte(seed))
	fill := color.RGBA{R: sum[0], G: sum[1], B: sum[2], A: 255}
	for y := range 128 {
		for x := range 128 {
			img.Set(x, y, fill)
		}
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
