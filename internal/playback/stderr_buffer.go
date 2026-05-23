package playback

import (
	"strings"
	"sync"
	"unicode/utf8"
)

const stderrTailMaxBytes = 64 * 1024

type boundedTailBuffer struct {
	mu     sync.Mutex
	max    int
	buffer []byte
}

func newBoundedTailBuffer(max int) *boundedTailBuffer {
	if max <= 0 {
		max = stderrTailMaxBytes
	}
	return &boundedTailBuffer{max: max}
}

func (b *boundedTailBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.buffer = append(b.buffer, p...)
	if overflow := len(b.buffer) - b.max; overflow > 0 {
		b.buffer = append([]byte(nil), b.buffer[overflow:]...)
	}
	return len(p), nil
}

func (b *boundedTailBuffer) Reset() {
	b.mu.Lock()
	b.buffer = b.buffer[:0]
	b.mu.Unlock()
}

func (b *boundedTailBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.ToValidUTF8(string(b.buffer), "\uFFFD")
}

func truncateUTF8String(value string, maxChars int) (string, bool) {
	if maxChars <= 0 {
		return "", value != ""
	}
	if utf8.RuneCountInString(value) <= maxChars {
		return value, false
	}
	runes := []rune(value)
	return string(runes[:maxChars]), true
}
