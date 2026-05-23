package opslog

// MemoryWriter buffers entries in a Go channel for the consumer goroutine.
type MemoryWriter struct {
	ch chan Entry
}

func NewMemoryWriter(bufSize int) *MemoryWriter {
	return &MemoryWriter{ch: make(chan Entry, bufSize)}
}

func (w *MemoryWriter) Write(entry Entry) {
	select {
	case w.ch <- entry:
	default:
	}
}

func (w *MemoryWriter) Close() error {
	close(w.ch)
	return nil
}

func (w *MemoryWriter) Chan() <-chan Entry {
	return w.ch
}
