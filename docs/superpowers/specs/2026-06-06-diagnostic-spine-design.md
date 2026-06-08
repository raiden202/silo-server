# Diagnostic Spine Design

## Context

Two live issues showed that Silo can produce enough logs to debug a problem, but the evidence is too scattered:

- Playback startup can sit for 9-14 seconds before the first transcoded segment. The latest trace showed ffmpeg blocked in kernel file-read wait on `/mnt/sharedrives/zd-storage-ceph` while the scanner was active and host I/O wait was high.
- The Movies library page can repeatedly refresh while Movies International is scanning. Logs showed a frontend/API loop with many `/api/v1/catalog` requests and many client-cancelled catalog requests.

The goal is to make these failure modes obvious during normal operation, without adding a large observability platform dependency as a prerequisite.

## Goals

- Correlate frontend actions, API requests, catalog queries, playback sessions, ffmpeg processes, scan runs, and logs.
- Explain why a catalog query or playback request happened, not only that it happened.
- Add low-cardinality metrics for the problem areas we already know matter.
- Provide admin-only diagnostic views/endpoints that consolidate evidence for playback, catalog refresh loops, and active scans.
- Keep instrumentation portable so Silo can later send the same data to Sentry, OpenTelemetry, Grafana, or another backend.

## Non-Goals

- Do not build a full hosted/self-hosted observability stack in this step.
- Do not replace existing structured logs.
- Do not add high-cardinality metric labels such as raw paths, content IDs, query fingerprints, or playback session IDs.
- Do not expose diagnostic data to non-admin users.

## Architecture

### Correlation Context

Introduce a lightweight diagnostic context that can carry:

- `request_id`
- `trace_id`
- `user_id`
- `profile_id`
- `library_id`
- `content_id`
- `playback_session_id`
- `scan_id`
- `ffmpeg_pid`
- `route`

The request logger already has some of this data. The implementation should extend existing middleware and helper APIs rather than creating a parallel logging system.

Frontend requests should include a stable client-side action/request correlation header when practical, for example `X-Silo-Client-Trace-ID`. Backend logs should echo it when present and generate one when missing.

### Diagnostic Events

Add structured domain events for the places where "why did this happen?" matters:

- `catalog.query.started`
- `catalog.query.completed`
- `catalog.query.cancelled`
- `catalog.query.invalidated`
- `catalog.refetch.triggered`
- `realtime.catalog.event.received`
- `playback.ffmpeg.spawned`
- `playback.first_segment.ready`
- `playback.segment_gap.detected`
- `scanner.progress`
- `scanner.completed`
- `scanner.io_pressure.snapshot`

These events should be regular structured logs at first. Later they can be mirrored into Sentry breadcrumbs or OpenTelemetry spans/events.

### Metrics

Add minimal metrics with bounded labels:

- `silo_catalog_requests_total{route,status,source,library_id}`
- `silo_catalog_cancelled_total{route,source,library_id}`
- `silo_catalog_request_duration_ms{route,source,library_id}`
- `silo_playback_startup_ms{mode,video_codec,audio_codec,hw_accel}`
- `silo_playback_first_segment_ms{mode,video_codec,audio_codec,hw_accel}`
- `silo_scanner_files_processed_total{library_id,phase}`
- `silo_scanner_active{library_id}`
- `silo_host_iowait_percent`

Metric labels should use library IDs and coarse codec/mode values, never raw file paths or query JSON.

### Playback Diagnostics

Add an admin-only playback diagnostics endpoint:

`GET /api/v1/admin/diagnostics/playback/{session_id}`

It should return:

- session identity and content/library IDs
- ffmpeg command summary
- ffmpeg PID if still running
- process start time
- first segment observed time
- segment production timeline
- restart count
- startup wait/recovery decisions
- stderr tail when available
- recent host pressure snapshot if available

This endpoint should assemble existing in-memory/session state plus the new segment monitor data. It does not need to scrape old log files.

### Catalog Diagnostics

Add admin-only diagnostics for catalog/UI loops:

`GET /api/v1/admin/diagnostics/catalog/activity`

It should return recent activity for the current node:

- recent catalog queries
- query status: started, completed, cancelled
- route/source/library ID
- request ID and client trace ID
- invalidation/refetch reason when known
- active scan library IDs
- recent realtime catalog events

The frontend should also expose a development/admin diagnostics panel showing:

- active route
- active React Query catalog keys
- last catalog refetch triggers
- last realtime catalog events
- current scan libraries
- recent API cancellation counts

### Scanner Diagnostics

Surface active scanner state in one place:

- active scan ID
- library ID
- phase
- current scope
- processed/total files
- files per second
- matched/retried counters
- whether playback is currently reading from the same mount family when that can be inferred

This should reuse scan queue progress state rather than polling scanner internals separately.

## Error Handling

- Context cancellation should not be logged as a backend error when the client intentionally aborted the request. It should be recorded as a cancellation event/counter.
- Diagnostic endpoints must tolerate missing data and return partial results with clear fields instead of failing.
- If telemetry export fails in the future, normal Silo behavior must continue.

## Privacy And Security

- Admin diagnostics only.
- Avoid raw media paths in metrics and frontend diagnostic panels.
- Keep full ffmpeg command and raw paths restricted to admin endpoints and structured server logs.
- Redact tokens and authorization headers from any captured frontend/backend telemetry.

## Rollout Plan

1. Add correlation context and improve request/catalog cancellation logging.
2. Add playback diagnostic state and the playback diagnostics endpoint.
3. Add catalog invalidation/refetch reason logging in the frontend and backend request logs.
4. Add scanner state to admin diagnostics.
5. Add minimal metrics exports.
6. Evaluate hosted Sentry for frontend errors/session replay and backend traces once the diagnostic context is in place.

## Testing

- Unit test correlation extraction and propagation helpers.
- Unit test catalog cancellation classification so client aborts are not counted as internal server failures.
- Unit test playback diagnostic timeline assembly.
- Add frontend tests for catalog refetch reason recording and visible-range stability.
- Add integration-style tests for admin diagnostics endpoints with partial/missing data.

## Success Criteria

- A slow playback start can be explained from one session ID without SSHing into the host.
- A catalog refresh loop shows the trigger that caused each refetch.
- Scanning one library does not silently force unrelated active catalog views to refetch without a recorded reason.
- Context-cancelled catalog requests are visible as cancellations rather than noisy internal errors.
- Sentry or OpenTelemetry can be added later without rewriting the instrumentation surface.
