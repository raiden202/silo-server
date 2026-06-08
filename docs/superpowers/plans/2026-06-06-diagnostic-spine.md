# Diagnostic Spine Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the first diagnostic spine slice so playback startup delays and catalog/UI refresh loops can be explained from correlated events instead of ad hoc SSH log spelunking.

**Architecture:** Add a small diagnostics package that stores bounded in-memory event timelines and exposes helpers for request trace IDs, catalog activity, and playback timelines. Backend handlers and middleware record structured events; the frontend adds a client trace header and records catalog refetch reasons. Admin-only diagnostics endpoints return recent activity without adding a required external observability platform.

**Tech Stack:** Go, chi middleware, pgx-backed API handlers, slog structured logs, React, TanStack Query, Vitest.

---

## File Structure

- Create `internal/diagnostics/context.go`: request/client trace helpers and header constants.
- Create `internal/diagnostics/recorder.go`: bounded in-memory recorder for catalog and playback diagnostic events.
- Create `internal/diagnostics/recorder_test.go`: recorder behavior and bounds tests.
- Modify `internal/api/middleware/request_logger.go`: include client trace ID in request logs.
- Modify `internal/api/handlers/catalog.go`: classify context cancellation as a cancellation diagnostic, not an internal catalog error.
- Modify `internal/api/router.go`: wire diagnostics recorder and admin endpoints.
- Create `internal/api/handlers/diagnostics.go`: admin-only diagnostics endpoints.
- Create `internal/api/handlers/diagnostics_test.go`: endpoint JSON and partial-data tests.
- Modify `internal/playback/transcode.go`: record first-segment and segment-gap diagnostics through an optional recorder hook.
- Modify `internal/api/handlers/playback.go`: attach playback diagnostics recorder to transcode sessions.
- Modify `web/src/api/client.ts`: attach `X-Silo-Client-Trace-ID` to API requests.
- Create `web/src/lib/clientTrace.ts`: browser trace ID helper.
- Create `web/src/lib/diagnostics/catalogActivity.ts`: frontend ring buffer for catalog refetch reasons.
- Modify `web/src/components/RealtimeEventsProvider.tsx`: record realtime catalog event/refetch reasons and avoid broad active catalog refetch when a catalog event has a different `library_id` than active catalog queries.
- Modify `web/src/hooks/queries/catalog.ts`: record query start/complete/cancel reasons.
- Add focused tests under `web/src/**.test.ts(x)` for trace header and catalog refetch filtering.

---

### Task 1: Backend Trace Context And Recorder

**Files:**
- Create: `internal/diagnostics/context.go`
- Create: `internal/diagnostics/recorder.go`
- Test: `internal/diagnostics/recorder_test.go`

- [ ] **Step 1: Write recorder tests**

Create `internal/diagnostics/recorder_test.go`:

```go
package diagnostics

import (
	"net/http"
	"testing"
	"time"
)

func TestClientTraceIDFromRequestPrefersHeader(t *testing.T) {
	req, err := http.NewRequest(http.MethodGet, "/api/v1/catalog", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set(ClientTraceHeader, "client-trace-123")

	if got := ClientTraceIDFromRequest(req); got != "client-trace-123" {
		t.Fatalf("ClientTraceIDFromRequest() = %q, want header value", got)
	}
}

func TestRecorderKeepsNewestCatalogEvents(t *testing.T) {
	rec := NewRecorder(2)
	rec.RecordCatalog(CatalogEvent{Kind: "catalog.query.started", RequestID: "old"})
	rec.RecordCatalog(CatalogEvent{Kind: "catalog.query.completed", RequestID: "middle"})
	rec.RecordCatalog(CatalogEvent{Kind: "catalog.query.cancelled", RequestID: "new"})

	events := rec.CatalogEvents()
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].RequestID != "middle" || events[1].RequestID != "new" {
		t.Fatalf("events = %+v, want newest two in order", events)
	}
}

func TestRecorderKeepsPlaybackEventsBySession(t *testing.T) {
	rec := NewRecorder(10)
	now := time.Now()
	rec.RecordPlayback(PlaybackEvent{Kind: "playback.ffmpeg.spawned", SessionID: "s1", At: now})
	rec.RecordPlayback(PlaybackEvent{Kind: "playback.first_segment.ready", SessionID: "s2", At: now})
	rec.RecordPlayback(PlaybackEvent{Kind: "playback.first_segment.ready", SessionID: "s1", At: now})

	events := rec.PlaybackEvents("s1")
	if len(events) != 2 {
		t.Fatalf("len(events) = %d, want 2", len(events))
	}
	if events[0].Kind != "playback.ffmpeg.spawned" || events[1].Kind != "playback.first_segment.ready" {
		t.Fatalf("events = %+v, want s1 events only in order", events)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
go test ./internal/diagnostics -count=1
```

Expected: FAIL because `internal/diagnostics` does not exist.

- [ ] **Step 3: Implement diagnostics context and recorder**

Create `internal/diagnostics/context.go`:

```go
package diagnostics

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"
)

const ClientTraceHeader = "X-Silo-Client-Trace-ID"

func ClientTraceIDFromRequest(r *http.Request) string {
	if r == nil {
		return ""
	}
	return sanitizeTraceID(r.Header.Get(ClientTraceHeader))
}

func NewTraceID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	return hex.EncodeToString(b[:])
}

func sanitizeTraceID(value string) string {
	value = strings.TrimSpace(value)
	if len(value) > 128 {
		value = value[:128]
	}
	return value
}
```

Create `internal/diagnostics/recorder.go`:

```go
package diagnostics

import (
	"slices"
	"sync"
	"time"
)

const DefaultEventLimit = 500

type CatalogEvent struct {
	At            time.Time `json:"at"`
	Kind          string    `json:"kind"`
	RequestID     string    `json:"request_id,omitempty"`
	ClientTraceID string    `json:"client_trace_id,omitempty"`
	Route         string    `json:"route,omitempty"`
	Source        string    `json:"source,omitempty"`
	LibraryID     int       `json:"library_id,omitempty"`
	Status        string    `json:"status,omitempty"`
	Reason        string    `json:"reason,omitempty"`
	DurationMS    int64     `json:"duration_ms,omitempty"`
}

type PlaybackEvent struct {
	At             time.Time `json:"at"`
	Kind           string    `json:"kind"`
	SessionID      string    `json:"session_id"`
	PlaybackID     string    `json:"playback_session_id,omitempty"`
	Segment        string    `json:"segment,omitempty"`
	ElapsedMS      int64     `json:"elapsed_ms,omitempty"`
	FFmpegPID      int       `json:"ffmpeg_pid,omitempty"`
	RestartCount   int       `json:"restart_count,omitempty"`
	Reason         string    `json:"reason,omitempty"`
	SizeBytes      int64     `json:"size_bytes,omitempty"`
	OutputDir      string    `json:"output_dir,omitempty"`
	TargetVideo    string    `json:"target_video_codec,omitempty"`
	TargetAudio    string    `json:"target_audio_codec,omitempty"`
	HardwareAccel  string    `json:"hw_accel,omitempty"`
}

type Recorder struct {
	mu       sync.RWMutex
	limit    int
	catalog  []CatalogEvent
	playback []PlaybackEvent
}

func NewRecorder(limit int) *Recorder {
	if limit <= 0 {
		limit = DefaultEventLimit
	}
	return &Recorder{limit: limit}
}

func (r *Recorder) RecordCatalog(event CatalogEvent) {
	if r == nil {
		return
	}
	if event.At.IsZero() {
		event.At = time.Now().UTC()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.catalog = appendBounded(r.catalog, event, r.limit)
}

func (r *Recorder) CatalogEvents() []CatalogEvent {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	return slices.Clone(r.catalog)
}

func (r *Recorder) RecordPlayback(event PlaybackEvent) {
	if r == nil {
		return
	}
	if event.At.IsZero() {
		event.At = time.Now().UTC()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.playback = appendBounded(r.playback, event, r.limit)
}

func (r *Recorder) PlaybackEvents(sessionID string) []PlaybackEvent {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []PlaybackEvent
	for _, event := range r.playback {
		if event.SessionID == sessionID {
			out = append(out, event)
		}
	}
	return out
}

func appendBounded[T any](values []T, next T, limit int) []T {
	values = append(values, next)
	if len(values) <= limit {
		return values
	}
	return values[len(values)-limit:]
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run:

```bash
go test ./internal/diagnostics -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/diagnostics
git commit -m "feat: add diagnostic event recorder"
```

---

### Task 2: Request And Catalog Cancellation Diagnostics

**Files:**
- Modify: `internal/api/middleware/request_logger.go`
- Modify: `internal/api/handlers/catalog.go`
- Test: `internal/api/handlers/catalog_test.go`

- [ ] **Step 1: Write cancellation classification test**

Add this test to `internal/api/handlers/catalog_test.go`:

```go
func TestCatalogErrorStatusTreatsContextCancellationAsClientCancel(t *testing.T) {
	code, errorCode, message, isCancel := catalogErrorStatus(context.Canceled)
	if code != 499 {
		t.Fatalf("status = %d, want 499", code)
	}
	if errorCode != "client_cancelled" {
		t.Fatalf("errorCode = %q, want client_cancelled", errorCode)
	}
	if message == "" {
		t.Fatal("message is empty")
	}
	if !isCancel {
		t.Fatal("isCancel = false, want true")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/api/handlers -run TestCatalogErrorStatusTreatsContextCancellationAsClientCancel -count=1
```

Expected: FAIL because `catalogErrorStatus` does not exist.

- [ ] **Step 3: Implement cancellation status and trace fields**

In `internal/api/handlers/catalog.go`, import `context` and add this helper near `HandleGetCatalog`:

```go
func catalogErrorStatus(err error) (status int, code string, message string, cancelled bool) {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return 499, "client_cancelled", "Catalog request was cancelled", true
	}
	if errors.Is(err, catalog.ErrInvalidCatalogRequest) {
		return http.StatusBadRequest, "bad_request", err.Error(), false
	}
	if errors.Is(err, catalog.ErrCatalogSourceNotFound) {
		return http.StatusNotFound, "not_found", "Catalog source not found", false
	}
	return http.StatusInternalServerError, "internal_error", "Failed to resolve catalog", false
}
```

Replace the current error ladder in `HandleGetCatalog` with:

```go
status, code, message, cancelled := catalogErrorStatus(err)
	if cancelled {
		slog.Info("catalog: resolve cancelled",
			"err_msg", err.Error(),
			"client_trace_id", diagnostics.ClientTraceIDFromRequest(r),
		)
	} else if status == http.StatusInternalServerError {
		slog.Error("catalog: resolve failed", "err_msg", err.Error())
	}
	writeError(w, status, code, message)
	return
```

In `internal/api/middleware/request_logger.go`, add `client_trace_id` to the request log attributes:

```go
if clientTraceID := diagnostics.ClientTraceIDFromRequest(r); clientTraceID != "" {
	attrs = append(attrs, slog.String("client_trace_id", clientTraceID))
}
```

Use the existing local attribute variable names in the middleware; do not rewrite the middleware structure.

- [ ] **Step 4: Run handler and middleware tests**

Run:

```bash
go test ./internal/api/handlers ./internal/api/middleware -count=1
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/api/handlers/catalog.go internal/api/handlers/catalog_test.go internal/api/middleware/request_logger.go
git commit -m "feat: record catalog cancellations with trace context"
```

---

### Task 3: Frontend Client Trace And Catalog Refetch Reasons

**Files:**
- Create: `web/src/lib/clientTrace.ts`
- Create: `web/src/lib/diagnostics/catalogActivity.ts`
- Modify: `web/src/api/client.ts`
- Modify: `web/src/hooks/queries/catalog.ts`
- Modify: `web/src/components/RealtimeEventsProvider.tsx`
- Test: `web/src/lib/clientTrace.test.ts`
- Test: `web/src/lib/diagnostics/catalogActivity.test.ts`

- [ ] **Step 1: Write frontend diagnostics tests**

Create `web/src/lib/clientTrace.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { getClientTraceId } from "./clientTrace";

describe("getClientTraceId", () => {
  it("returns one stable id per page lifetime", () => {
    const first = getClientTraceId();
    const second = getClientTraceId();
    expect(first).toBe(second);
    expect(first.length).toBeGreaterThan(8);
  });
});
```

Create `web/src/lib/diagnostics/catalogActivity.test.ts`:

```ts
import { describe, expect, it } from "vitest";
import { getCatalogActivity, recordCatalogActivity, resetCatalogActivity } from "./catalogActivity";

describe("catalogActivity", () => {
  it("keeps recent events in insertion order", () => {
    resetCatalogActivity();
    recordCatalogActivity({ kind: "catalog.query.started", route: "/catalog", libraryId: 1 });
    recordCatalogActivity({ kind: "catalog.refetch.triggered", reason: "realtime", libraryId: 1 });

    expect(getCatalogActivity()).toMatchObject([
      { kind: "catalog.query.started", route: "/catalog", libraryId: 1 },
      { kind: "catalog.refetch.triggered", reason: "realtime", libraryId: 1 },
    ]);
  });
});
```

- [ ] **Step 2: Run tests to verify they fail**

Run:

```bash
cd web && pnpm vitest run src/lib/clientTrace.test.ts src/lib/diagnostics/catalogActivity.test.ts
```

Expected: FAIL because the modules do not exist.

- [ ] **Step 3: Implement frontend trace helpers**

Create `web/src/lib/clientTrace.ts`:

```ts
let clientTraceId: string | null = null;

export function getClientTraceId() {
  if (clientTraceId) return clientTraceId;
  const cryptoObj = globalThis.crypto;
  clientTraceId =
    cryptoObj && "randomUUID" in cryptoObj
      ? cryptoObj.randomUUID()
      : `trace-${Date.now().toString(36)}-${Math.random().toString(36).slice(2)}`;
  return clientTraceId;
}
```

Create `web/src/lib/diagnostics/catalogActivity.ts`:

```ts
export interface CatalogActivityEvent {
  at: string;
  kind: string;
  route?: string;
  reason?: string;
  libraryId?: number;
  source?: string;
  status?: string;
}

const MAX_EVENTS = 200;
const events: CatalogActivityEvent[] = [];

export function recordCatalogActivity(event: Omit<CatalogActivityEvent, "at"> & { at?: string }) {
  events.push({
    at: event.at ?? new Date().toISOString(),
    kind: event.kind,
    route: event.route,
    reason: event.reason,
    libraryId: event.libraryId,
    source: event.source,
    status: event.status,
  });
  if (events.length > MAX_EVENTS) {
    events.splice(0, events.length - MAX_EVENTS);
  }
}

export function getCatalogActivity() {
  return [...events];
}

export function resetCatalogActivity() {
  events.splice(0, events.length);
}
```

- [ ] **Step 4: Add header and query activity recording**

In `web/src/api/client.ts`, add the header:

```ts
headers.set("X-Silo-Client-Trace-ID", getClientTraceId());
```

Import `getClientTraceId` from `@/lib/clientTrace`. Preserve existing auth and content-type headers.

In `web/src/hooks/queries/catalog.ts`, record:

```ts
recordCatalogActivity({
  kind: "catalog.query.started",
  route: "/catalog",
  source: state.source,
  libraryId: state.library_id,
});
```

Wrap `fetchCatalogPage` calls in the query functions so completion and cancellation can be recorded:

```ts
try {
  const result = await fetchCatalogPage(state, limit, offset, { signal }, includeTotal, snapshot);
  recordCatalogActivity({ kind: "catalog.query.completed", route: "/catalog", source: state.source, libraryId: state.library_id });
  return result;
} catch (error) {
  recordCatalogActivity({
    kind: error instanceof DOMException && error.name === "AbortError" ? "catalog.query.cancelled" : "catalog.query.failed",
    route: "/catalog",
    source: state.source,
    libraryId: state.library_id,
  });
  throw error;
}
```

Apply the same pattern for page 0 and remaining pages.

- [ ] **Step 5: Scope realtime catalog refetches**

In `web/src/components/RealtimeEventsProvider.tsx`, before broad refetch, record:

```ts
recordCatalogActivity({
  kind: "realtime.catalog.event.received",
  reason: message.event,
  libraryId: typeof message.data === "object" && message.data && "library_id" in message.data ? Number((message.data as { library_id?: unknown }).library_id) : undefined,
});
```

Change the active catalog refetch call in `invalidateCatalogState` to skip active catalog list queries whose `library_id` does not match the event library ID when an event library ID is present. Implement a helper:

```ts
function activeCatalogQueryMatchesLibrary(queryKey: unknown, libraryId?: number) {
  if (!libraryId || !Array.isArray(queryKey)) return true;
  const params = queryKey[2] as { library_id?: number } | undefined;
  return params?.library_id == null || params.library_id === libraryId;
}
```

Then use `predicate` with `refetchQueries` and record `catalog.refetch.triggered` only for matching active queries.

- [ ] **Step 6: Run frontend tests**

Run:

```bash
cd web && pnpm vitest run src/lib/clientTrace.test.ts src/lib/diagnostics/catalogActivity.test.ts
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add web/src/api/client.ts web/src/hooks/queries/catalog.ts web/src/components/RealtimeEventsProvider.tsx web/src/lib/clientTrace.ts web/src/lib/clientTrace.test.ts web/src/lib/diagnostics/catalogActivity.ts web/src/lib/diagnostics/catalogActivity.test.ts
git commit -m "feat: trace frontend catalog refetch activity"
```

---

### Task 4: Playback Diagnostics Endpoint

**Files:**
- Create: `internal/api/handlers/diagnostics.go`
- Create: `internal/api/handlers/diagnostics_test.go`
- Modify: `internal/api/router.go`
- Modify: `internal/playback/transcode.go`
- Modify: `internal/api/handlers/playback.go`

- [ ] **Step 1: Write endpoint test**

Create `internal/api/handlers/diagnostics_test.go`:

```go
package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Silo-Server/silo-server/internal/diagnostics"
)

func TestDiagnosticsHandlerReturnsPlaybackTimeline(t *testing.T) {
	rec := diagnostics.NewRecorder(10)
	rec.RecordPlayback(diagnostics.PlaybackEvent{Kind: "playback.ffmpeg.spawned", SessionID: "session-1", FFmpegPID: 123})
	rec.RecordPlayback(diagnostics.PlaybackEvent{Kind: "playback.first_segment.ready", SessionID: "session-1", Segment: "seg_00000.m4s", ElapsedMS: 13500})
	handler := NewDiagnosticsHandler(rec)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/diagnostics/playback/session-1", nil)
	rr := httptest.NewRecorder()

	handler.HandlePlayback(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var body struct {
		SessionID string                       `json:"session_id"`
		Events    []diagnostics.PlaybackEvent `json:"events"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.SessionID != "session-1" || len(body.Events) != 2 {
		t.Fatalf("body = %+v, want session timeline", body)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run:

```bash
go test ./internal/api/handlers -run TestDiagnosticsHandlerReturnsPlaybackTimeline -count=1
```

Expected: FAIL because `NewDiagnosticsHandler` does not exist.

- [ ] **Step 3: Implement diagnostics handler**

Create `internal/api/handlers/diagnostics.go`:

```go
package handlers

import (
	"net/http"

	"github.com/Silo-Server/silo-server/internal/diagnostics"
	"github.com/go-chi/chi/v5"
)

type DiagnosticsHandler struct {
	recorder *diagnostics.Recorder
}

func NewDiagnosticsHandler(recorder *diagnostics.Recorder) *DiagnosticsHandler {
	return &DiagnosticsHandler{recorder: recorder}
}

func (h *DiagnosticsHandler) HandlePlayback(w http.ResponseWriter, r *http.Request) {
	sessionID := chi.URLParam(r, "session_id")
	if sessionID == "" {
		sessionID = pathTail(r.URL.Path)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"session_id": sessionID,
		"events":     h.recorder.PlaybackEvents(sessionID),
	})
}

func (h *DiagnosticsHandler) HandleCatalogActivity(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"events": h.recorder.CatalogEvents(),
	})
}

func pathTail(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}
```

- [ ] **Step 4: Wire handler in router**

In `internal/api/router.go`, create one recorder during router construction:

```go
diagnosticsRecorder := diagnostics.NewRecorder(diagnostics.DefaultEventLimit)
diagnosticsHandler := handlers.NewDiagnosticsHandler(diagnosticsRecorder)
```

Wire admin routes:

```go
r.Route("/admin/diagnostics", func(r chi.Router) {
	r.Get("/playback/{session_id}", diagnosticsHandler.HandlePlayback)
	r.Get("/catalog/activity", diagnosticsHandler.HandleCatalogActivity)
})
```

Pass `diagnosticsRecorder` into playback/catalog handlers through new optional fields. Keep routes behind existing admin middleware in the admin route group.

- [ ] **Step 5: Record playback events**

In `internal/playback/transcode.go`, add an optional callback field on the transcode session/config:

```go
type DiagnosticRecorder interface {
	RecordPlayback(diagnostics.PlaybackEvent)
}
```

Record `playback.ffmpeg.spawned` when ffmpeg starts and `playback.first_segment.ready` when `seg_00000.m4s` is first observed. Use the existing segment monitor added during playback debugging.

- [ ] **Step 6: Run backend tests**

Run:

```bash
go test ./internal/diagnostics ./internal/api/handlers ./internal/playback -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/api/handlers/diagnostics.go internal/api/handlers/diagnostics_test.go internal/api/router.go internal/playback/transcode.go internal/api/handlers/playback.go
git commit -m "feat: expose playback diagnostics timeline"
```

---

### Task 5: Catalog Activity Endpoint And Scanner State

**Files:**
- Modify: `internal/api/handlers/diagnostics.go`
- Modify: `internal/scanqueue/service.go`
- Modify: `internal/api/router.go`
- Test: `internal/api/handlers/diagnostics_test.go`

- [ ] **Step 1: Extend diagnostics endpoint test**

Add this test to `internal/api/handlers/diagnostics_test.go`:

```go
func TestDiagnosticsHandlerReturnsCatalogActivity(t *testing.T) {
	rec := diagnostics.NewRecorder(10)
	rec.RecordCatalog(diagnostics.CatalogEvent{Kind: "catalog.query.cancelled", Route: "/catalog", LibraryID: 1, Reason: "client_cancelled"})
	handler := NewDiagnosticsHandler(rec)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/diagnostics/catalog/activity", nil)
	rr := httptest.NewRecorder()

	handler.HandleCatalogActivity(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rr.Code)
	}
	var body struct {
		Events []diagnostics.CatalogEvent `json:"events"`
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if len(body.Events) != 1 || body.Events[0].Kind != "catalog.query.cancelled" {
		t.Fatalf("body = %+v, want catalog activity", body)
	}
}
```

- [ ] **Step 2: Run test**

Run:

```bash
go test ./internal/api/handlers -run 'TestDiagnosticsHandlerReturns(CatalogActivity|PlaybackTimeline)' -count=1
```

Expected: PASS if Task 4 handler already returns catalog events; FAIL if catalog endpoint was not implemented.

- [ ] **Step 3: Record catalog events in handler**

In `internal/api/handlers/catalog.go`, add an optional recorder field to `CatalogHandler` and record:

```go
h.Diagnostics.RecordCatalog(diagnostics.CatalogEvent{
	Kind:          "catalog.query.started",
	RequestID:     requestIDFromContext(r.Context()),
	ClientTraceID: diagnostics.ClientTraceIDFromRequest(r),
	Route:         "/catalog",
	Source:        req.Source,
	LibraryID:     firstLibraryID(req.Query.LibraryIDs),
})
```

On success, record `catalog.query.completed` with duration. On context cancellation, record `catalog.query.cancelled` with reason `client_cancelled`.

- [ ] **Step 4: Add scan progress snapshots**

In `internal/scanqueue/service.go`, where `scan queue: progress` is logged, also record or expose the latest progress through a small `LatestRuns()` method. Return bounded state:

```go
type DiagnosticScanState struct {
	ScanID         string `json:"scan_id"`
	LibraryID      int    `json:"library_id"`
	Phase          string `json:"phase"`
	Message        string `json:"message"`
	ProcessedFiles int    `json:"processed_files"`
	TotalFiles     int    `json:"total_files"`
	MatchedFiles   int    `json:"matched_files"`
	RetriedItems   int    `json:"retried_items"`
}
```

Add this state to `GET /api/v1/admin/diagnostics/catalog/activity` under `active_scans`.

- [ ] **Step 5: Run tests**

Run:

```bash
go test ./internal/api/handlers ./internal/scanqueue -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/api/handlers/catalog.go internal/api/handlers/diagnostics.go internal/api/handlers/diagnostics_test.go internal/scanqueue/service.go internal/api/router.go
git commit -m "feat: expose catalog and scan diagnostic activity"
```

---

### Task 6: Remote Build And Live Verification

**Files:**
- No new source files unless previous tasks reveal compile fixes.

- [ ] **Step 1: Run focused local tests**

Run:

```bash
go test ./internal/diagnostics ./internal/api/handlers ./internal/api/middleware ./internal/playback ./internal/scanqueue -count=1
cd web && pnpm vitest run src/lib/clientTrace.test.ts src/lib/diagnostics/catalogActivity.test.ts
```

Expected: all tests pass.

- [ ] **Step 2: Build server**

Run:

```bash
go build ./cmd/silo
```

Expected: build exits 0.

- [ ] **Step 3: Install on `silo-new` native runtime**

Use the existing native install flow:

```bash
rsync -az --delete --exclude .git /Users/jimcole/projects/silo/silo-server/ root@silo-new:/tmp/silo-server-build/
ssh root@silo-new 'cd /tmp/silo-server-build && go test ./internal/diagnostics ./internal/api/handlers ./internal/api/middleware ./internal/playback ./internal/scanqueue -count=1 && go build -ldflags "-X main.buildVersion=diagnostic-spine" -o /opt/silo-native/silo ./cmd/silo && old=$(cat /opt/silo-native/silo.pid 2>/dev/null || true); [ -n "$old" ] && kill "$old" || true; nohup /opt/silo-native/run-host.sh >/opt/silo-native/logs/silo.log 2>&1 &'
```

Expected: remote tests pass, binary builds, native server restarts.

- [ ] **Step 4: Verify health**

Run:

```bash
ssh root@silo-new 'for i in $(seq 1 30); do curl -fsS http://127.0.0.1:8090/api/v1/health && exit 0; sleep 1; done; exit 1'
```

Expected: health JSON is returned.

- [ ] **Step 5: Verify diagnostics during reproduction**

While the user opens Movies and starts playback, run:

```bash
ssh root@silo-new 'tail -n 500 /opt/silo-native/logs/silo.log | grep -E "catalog.query|catalog.refetch|realtime.catalog|playback.ffmpeg|playback.first_segment|client_trace_id" | tail -n 120'
```

Expected: logs show why catalog queries/refetches happen and show playback first-segment timing with session IDs.

---

## Self-Review

- Spec coverage: correlation context is covered in Tasks 1-3; catalog cancellation/refetch diagnostics in Tasks 2-3 and 5; playback diagnostics endpoint in Task 4; scanner state in Task 5; remote verification in Task 6.
- Deferred intentionally: external Sentry/OpenTelemetry export and full metrics endpoint are not implemented in this first plan. The spec says to evaluate Sentry after context is in place, and metrics can follow once event fields are stable.
- Placeholder scan: no open-ended implementation steps remain.
- Type consistency: diagnostics event names and JSON field names match the design spec.

## Rollback Note

2026-06-06: Behavior-changing mitigations were intentionally rolled back so the next live test can reproduce and diagnose the original symptoms. Kept: diagnostics recorder, request/client trace IDs, catalog activity timeline, playback ffmpeg/segment timeline, and admin diagnostics endpoints. Rolled back: longer playback segment wait/restart gating, library-scoped realtime catalog refetch filtering, and catalog cancellation HTTP reclassification from 500 to 499.
