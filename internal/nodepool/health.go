package nodepool

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// healthResponse is the JSON response from a node's /health endpoint.
type healthResponse struct {
	Status     string `json:"status"`
	ActiveJobs int    `json:"active_jobs"`
}

// CheckNode pings a node's /health endpoint and returns its health status
// and active job count.
func CheckNode(ctx context.Context, n *Node) (healthy bool, activeJobs int) {
	client := &http.Client{Timeout: 5 * time.Second}
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, n.URL+"/api/v1/health", nil)
	if err != nil {
		return false, 0
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, 0
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, 0
	}

	var hr healthResponse
	if err := json.NewDecoder(resp.Body).Decode(&hr); err != nil {
		return false, 0
	}

	return true, hr.ActiveJobs
}

// HealthChecker runs periodic health checks on all nodes in both pools,
// updating in-memory state and optionally persisting to the database.
type HealthChecker struct {
	proxyPool     *ProxyPool
	transcodePool *TranscodePool
	repo          *Repository // may be nil (proxy/transcode modes have no DB)
	interval      time.Duration
}

// NewHealthChecker creates a health checker for the given pools.
func NewHealthChecker(proxyPool *ProxyPool, transcodePool *TranscodePool, repo *Repository) *HealthChecker {
	return &HealthChecker{
		proxyPool:     proxyPool,
		transcodePool: transcodePool,
		repo:          repo,
		interval:      30 * time.Second,
	}
}

// Start runs health checks in a background goroutine. Stops when ctx is cancelled.
func (hc *HealthChecker) Start(ctx context.Context) {
	go func() {
		hc.checkAll(ctx)
		ticker := time.NewTicker(hc.interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				hc.checkAll(ctx)
			}
		}
	}()
}

func (hc *HealthChecker) checkAll(ctx context.Context) {
	var allNodes []*Node
	allNodes = append(allNodes, hc.proxyPool.Nodes()...)
	allNodes = append(allNodes, hc.transcodePool.Nodes()...)

	var wg sync.WaitGroup
	for _, n := range allNodes {
		wg.Go(func() {
			healthy, activeJobs := CheckNode(ctx, n)

			wasHealthy := n.Healthy
			n.Healthy = healthy
			n.ActiveJobs = activeJobs
			now := time.Now()
			n.LastHealthCheck = &now

			if wasHealthy && !healthy {
				slog.Warn("stream node unhealthy", "id", n.ID, "name", n.Name, "url", n.URL)
			} else if !wasHealthy && healthy {
				slog.Info("stream node recovered", "id", n.ID, "name", n.Name, "url", n.URL)
			}

			if hc.repo != nil {
				if err := hc.repo.UpdateHealth(ctx, n.ID, healthy, activeJobs); err != nil {
					slog.Error("failed to persist node health", "id", n.ID, "error", err)
				}
			}
		})
	}
	wg.Wait()
}
