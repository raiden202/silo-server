package nodepool

import "sync"

// TranscodePool manages transcode nodes with least-connections selection.
// Thread-safe for concurrent use.
type TranscodePool struct {
	nodes []*Node
	mu    sync.RWMutex
}

// NewTranscodePool creates an empty transcode pool.
func NewTranscodePool() *TranscodePool {
	return &TranscodePool{}
}

// SetNodes replaces the node list.
func (p *TranscodePool) SetNodes(nodes []*Node) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.nodes = nodes
}

// Acquire returns the healthy node with the fewest active jobs.
// Returns nil if no healthy nodes are available.
func (p *TranscodePool) Acquire() *Node {
	p.mu.RLock()
	defer p.mu.RUnlock()

	var best *Node
	for _, n := range p.nodes {
		if !n.Healthy || !n.Enabled {
			continue
		}
		if best == nil || n.ActiveJobs < best.ActiveJobs {
			best = n
		}
	}
	return best
}

// FindByURL returns the node with the given URL, or nil if not found.
// Used for soft-affinity during quality switches.
func (p *TranscodePool) FindByURL(url string) *Node {
	p.mu.RLock()
	defer p.mu.RUnlock()
	for _, n := range p.nodes {
		if n.URL == url {
			return n
		}
	}
	return nil
}

// Nodes returns a copy of the current node list.
func (p *TranscodePool) Nodes() []*Node {
	p.mu.RLock()
	defer p.mu.RUnlock()
	cp := make([]*Node, len(p.nodes))
	copy(cp, p.nodes)
	return cp
}
