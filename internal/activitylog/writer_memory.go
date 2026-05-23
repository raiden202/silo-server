package activitylog

// MemoryWriter buffers entries in a Go channel for the consumer goroutine.
type MemoryWriter struct {
	ch chan LogEntry
}

// NewMemoryWriter creates a Writer backed by an in-memory channel.
// bufSize controls the channel buffer capacity.
func NewMemoryWriter(bufSize int) *MemoryWriter {
	return &MemoryWriter{ch: make(chan LogEntry, bufSize)}
}

func (w *MemoryWriter) Write(entry LogEntry) {
	select {
	case w.ch <- entry:
	default:
		// Drop entry if buffer is full — acceptable trade-off for memory-only mode
	}
}

func (w *MemoryWriter) Close() error {
	close(w.ch)
	return nil
}

// Chan returns the underlying channel for the consumer to read from.
func (w *MemoryWriter) Chan() <-chan LogEntry {
	return w.ch
}
