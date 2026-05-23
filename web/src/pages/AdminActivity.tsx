import type { ReactNode } from "react";
import { useState, useMemo } from "react";
import { Link } from "react-router";
import { AdminSessionActions } from "@/components/AdminSessionActions";
import { useRealtimeEvents } from "@/components/realtimeEventsContext";
import { useOperationalLogs } from "@/hooks/queries/admin/logs";
import { Input } from "@/components/ui/input";
import { Button } from "@/components/ui/button";
import type { AdminSession, OperationalLogEntry, IPUserEntry } from "@/api/types";
import { useIPUsers } from "@/hooks/queries/admin/ips";
import { useAdminSessions } from "@/hooks/queries/admin/stats";
import {
  formatAudioDetail,
  formatAudioSummary,
  formatDecisionLabel,
  formatSessionBitrate,
  formatVideoDetail,
  formatVideoSummary,
} from "@/pages/adminActivityPresentation";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  RefreshCw,
  Search,
  Radio,
  Filter,
  X,
  Play,
  Terminal,
  ChevronDown,
  ChevronUp,
} from "lucide-react";

type SortField = "username" | "media" | "method" | "node" | "started";
type SortDir = "asc" | "desc";

export default function AdminActivity() {
  const { data: sessions = [], isLoading, refetch: refresh } = useAdminSessions();
  const { connectionState } = useRealtimeEvents();
  const error = undefined;
  const [search, setSearch] = useState("");
  const [methodFilter, setMethodFilter] = useState<string | null>(null);
  const [nodeFilter, setNodeFilter] = useState<string | null>(null);
  const [typeFilter, setTypeFilter] = useState<string | null>(null);
  const [sortField, setSortField] = useState<SortField>("started");
  const [sortDir, setSortDir] = useState<SortDir>("desc");
  const [ipSearch, setIPSearch] = useState("");
  const [activeIP, setActiveIP] = useState("");
  const { data: ipUsers = [], isLoading: ipLoading } = useIPUsers(activeIP);

  // Aggregate counts
  const methods = useMemo(() => {
    const counts: Record<string, number> = {};
    for (const s of sessions)
      counts[s.play_method || "unknown"] = (counts[s.play_method || "unknown"] || 0) + 1;
    return counts;
  }, [sessions]);

  const nodes = useMemo(() => {
    const counts: Record<string, number> = {};
    for (const s of sessions)
      counts[s.reporting_node || "unknown"] = (counts[s.reporting_node || "unknown"] || 0) + 1;
    return counts;
  }, [sessions]);

  // Filter + sort
  const filtered = useMemo(() => {
    let result = sessions;
    if (search) {
      const q = search.toLowerCase();
      result = result.filter(
        (s) =>
          s.username?.toLowerCase().includes(q) ||
          s.media_title?.toLowerCase().includes(q) ||
          s.series_name?.toLowerCase().includes(q) ||
          s.episode_name?.toLowerCase().includes(q),
      );
    }
    if (methodFilter) result = result.filter((s) => s.play_method === methodFilter);
    if (nodeFilter) result = result.filter((s) => s.reporting_node === nodeFilter);
    if (typeFilter) result = result.filter((s) => s.media_type === typeFilter);

    return [...result].sort((a, b) => {
      let cmp = 0;
      switch (sortField) {
        case "username":
          cmp = (a.username || "").localeCompare(b.username || "");
          break;
        case "media":
          cmp = getDisplayTitle(a).localeCompare(getDisplayTitle(b));
          break;
        case "method":
          cmp = (a.play_method || "").localeCompare(b.play_method || "");
          break;
        case "node":
          cmp = (a.reporting_node || "").localeCompare(b.reporting_node || "");
          break;
        case "started":
          cmp = new Date(a.started_at).getTime() - new Date(b.started_at).getTime();
          break;
      }
      return sortDir === "asc" ? cmp : -cmp;
    });
  }, [sessions, search, methodFilter, nodeFilter, typeFilter, sortField, sortDir]);

  const toggleSort = (field: SortField) => {
    if (sortField === field) {
      setSortDir((d) => (d === "asc" ? "desc" : "asc"));
    } else {
      setSortField(field);
      setSortDir(field === "started" ? "desc" : "asc");
    }
  };

  const activeFilters = [methodFilter, nodeFilter, typeFilter].filter(Boolean).length;

  if (isLoading) return <div className="text-muted-foreground p-8">Loading activity...</div>;

  return (
    <div className="space-y-5 lg:space-y-6">
      {/* Header */}
      <div className="page-header">
        <div className="space-y-3">
          <div className="flex items-center gap-3">
            <h1 className="page-title text-[clamp(2rem,4vw,3rem)]">Activity</h1>
            {sessions.length > 0 && (
              <span className="live-badge flex items-center gap-1.5">
                <Radio className="h-3 w-3" />
                {sessions.length} live
              </span>
            )}
          </div>
          <p className="text-muted-foreground mt-1 text-[13px]">
            {sessions.length === 0
              ? "No active streams"
              : `${sessions.length} active stream${sessions.length !== 1 ? "s" : ""} across ${Object.keys(nodes).length} node${Object.keys(nodes).length !== 1 ? "s" : ""}`}
          </p>
        </div>
        <div className="flex items-center gap-3">
          <div className="text-right">
            <div className="text-muted-foreground text-[10px] font-semibold tracking-wider uppercase">
              Stream
            </div>
            <div className="text-[12px]">{formatConnectionState(connectionState)}</div>
            {error && <div className="text-muted-foreground text-[11px]">{error}</div>}
          </div>
          <Button variant="outline" size="sm" onClick={() => void refresh()}>
            <RefreshCw className="h-3.5 w-3.5" />
            Refresh
          </Button>
        </div>
      </div>

      {/* IP Lookup */}
      <details className="surface-panel rounded-2xl border-0">
        <summary className="cursor-pointer px-4 py-3 text-sm font-medium select-none">
          IP Lookup
        </summary>
        <div className="px-4 pb-4">
          <form
            onSubmit={(e) => {
              e.preventDefault();
              setActiveIP(ipSearch.trim());
            }}
            className="flex items-center gap-2"
          >
            <Input
              type="text"
              placeholder="IP lookup (e.g. 203.0.113.50)"
              value={ipSearch}
              onChange={(e) => setIPSearch(e.target.value)}
              className="max-w-xs font-mono text-sm"
            />
            <Button type="submit" variant="outline" size="sm" disabled={!ipSearch.trim()}>
              <Search className="mr-1 h-3.5 w-3.5" />
              Lookup
            </Button>
          </form>
          {activeIP && (
            <div className="mt-3">
              {ipLoading ? (
                <p className="text-muted-foreground text-sm">Searching...</p>
              ) : ipUsers.length === 0 ? (
                <p className="text-muted-foreground text-sm">
                  No users found for {activeIP} in the last 30 days.
                </p>
              ) : (
                <Table>
                  <TableHeader>
                    <TableRow>
                      <TableHead>User</TableHead>
                      <TableHead>First Seen</TableHead>
                      <TableHead>Last Seen</TableHead>
                      <TableHead className="text-right">Requests</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {ipUsers.map((entry: IPUserEntry) => (
                      <TableRow key={entry.user_id}>
                        <TableCell>
                          <Link
                            to={`/admin/users/${entry.user_id}`}
                            className="text-primary font-medium hover:underline"
                          >
                            {entry.username || `User #${entry.user_id}`}
                          </Link>
                        </TableCell>
                        <TableCell className="text-sm">
                          {new Date(entry.first_seen).toLocaleString()}
                        </TableCell>
                        <TableCell className="text-sm">
                          {new Date(entry.last_seen).toLocaleString()}
                        </TableCell>
                        <TableCell className="text-right">
                          {entry.request_count.toLocaleString()}
                        </TableCell>
                      </TableRow>
                    ))}
                  </TableBody>
                </Table>
              )}
            </div>
          )}
        </div>
      </details>

      {/* Summary strip */}
      {sessions.length > 0 && (
        <div className="surface-panel rounded-2xl border-0 p-4">
          {/* Method distribution bar */}
          <div className="mb-3">
            <div className="text-muted-foreground mb-2 text-[10px] font-semibold tracking-wider uppercase">
              Play Method
            </div>
            <div className="flex h-1.5 overflow-hidden rounded-full">
              {Object.entries(methods)
                .sort(([a], [b]) => a.localeCompare(b))
                .map(([method, count]) => (
                  <div
                    key={method}
                    className={`transition-all duration-500 ${methodBarColor(method)}`}
                    style={{ width: `${(count / sessions.length) * 100}%` }}
                  />
                ))}
            </div>
            <div className="mt-2 flex flex-wrap gap-x-4 gap-y-1">
              {Object.entries(methods)
                .sort(([a], [b]) => a.localeCompare(b))
                .map(([method, count]) => (
                  <button
                    key={method}
                    onClick={() => setMethodFilter(methodFilter === method ? null : method)}
                    className={`flex items-center gap-1.5 text-[11px] transition-opacity ${
                      methodFilter && methodFilter !== method ? "opacity-30" : ""
                    }`}
                  >
                    <span
                      className={`inline-block h-2 w-2 rounded-full ${methodDotColor(method)}`}
                    />
                    <span className="font-medium capitalize">{method}</span>
                    <span className="text-muted-foreground tabular-nums">{count}</span>
                  </button>
                ))}
            </div>
          </div>

          {/* Node breakdown */}
          {Object.keys(nodes).length > 1 && (
            <div className="border-border border-t pt-3">
              <div className="text-muted-foreground mb-2 text-[10px] font-semibold tracking-wider uppercase">
                By Node
              </div>
              <div className="flex flex-wrap gap-1.5">
                {Object.entries(nodes)
                  .sort(([, a], [, b]) => b - a)
                  .map(([node, count]) => (
                    <button
                      key={node}
                      onClick={() => setNodeFilter(nodeFilter === node ? null : node)}
                      className={`bg-surface border-border hover:border-primary/20 rounded-md border px-2.5 py-1 text-[11px] font-medium transition-all ${
                        nodeFilter === node
                          ? "border-primary/40 bg-primary/10 text-primary"
                          : nodeFilter
                            ? "opacity-30"
                            : ""
                      }`}
                    >
                      {node}
                      <span className="text-muted-foreground ml-1.5 tabular-nums">{count}</span>
                    </button>
                  ))}
              </div>
            </div>
          )}
        </div>
      )}

      {/* Search + filters */}
      <div className="flex flex-wrap items-center gap-2">
        <div className="relative min-w-[200px] flex-1">
          <Search className="text-muted-foreground pointer-events-none absolute top-1/2 left-3 h-3.5 w-3.5 -translate-y-1/2" />
          <Input
            placeholder="Filter by user or media..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="h-8 pl-9 text-[13px]"
          />
          {search && (
            <button
              onClick={() => setSearch("")}
              className="text-muted-foreground hover:text-foreground absolute top-1/2 right-2.5 -translate-y-1/2"
            >
              <X className="h-3.5 w-3.5" />
            </button>
          )}
        </div>
        <div className="flex gap-1">
          {(["movie", "series"] as const).map((t) => (
            <button
              key={t}
              onClick={() => setTypeFilter(typeFilter === t ? null : t)}
              className={`rounded-md border px-2.5 py-1.5 text-[11px] font-medium capitalize transition-all ${
                typeFilter === t
                  ? "border-primary/40 bg-primary/10 text-primary"
                  : "border-border bg-surface text-muted-foreground hover:text-foreground"
              }`}
            >
              {t === "movie" ? "Movies" : "Series"}
            </button>
          ))}
        </div>
        {activeFilters > 0 && (
          <button
            onClick={() => {
              setMethodFilter(null);
              setNodeFilter(null);
              setTypeFilter(null);
            }}
            className="text-muted-foreground hover:text-foreground flex items-center gap-1 text-[11px]"
          >
            <X className="h-3 w-3" />
            Clear filters
          </button>
        )}
      </div>

      {/* Filter status */}
      {(search || activeFilters > 0) && (
        <div className="text-muted-foreground text-[11px]">
          Showing {filtered.length} of {sessions.length} streams
        </div>
      )}

      {/* Stream table */}
      {filtered.length === 0 ? (
        <EmptyState hasData={sessions.length > 0} />
      ) : (
        <div className="bg-card border-border overflow-hidden rounded-lg border">
          {/* Table header */}
          <div className="border-border bg-surface/50 hidden grid-cols-[minmax(140px,1.5fr)_minmax(220px,2.2fr)_minmax(150px,1.4fr)_minmax(150px,1.4fr)_minmax(90px,1fr)_80px] items-center gap-3 border-b px-4 py-2.5 sm:grid">
            <SortHeader field="username" current={sortField} dir={sortDir} onClick={toggleSort}>
              User
            </SortHeader>
            <SortHeader field="media" current={sortField} dir={sortDir} onClick={toggleSort}>
              Stream
            </SortHeader>
            <SortHeader field="method" current={sortField} dir={sortDir} onClick={toggleSort}>
              Video
            </SortHeader>
            <div className="text-muted-foreground text-[10px] font-semibold tracking-wider uppercase">
              Audio
            </div>
            <SortHeader field="node" current={sortField} dir={sortDir} onClick={toggleSort}>
              Node
            </SortHeader>
            <SortHeader
              field="started"
              current={sortField}
              dir={sortDir}
              onClick={toggleSort}
              className="justify-end"
            >
              Time
            </SortHeader>
          </div>

          {/* Rows */}
          <div className="max-h-[calc(100vh-420px)] min-h-[200px] overflow-y-auto">
            {filtered.map((session, i) => (
              <StreamRow key={session.session_id} session={session} even={i % 2 === 0} />
            ))}
          </div>
        </div>
      )}
    </div>
  );
}

// --- Sub-components ---

function StreamRow({ session, even }: { session: AdminSession; even: boolean }) {
  const [ffmpegOpen, setFFmpegOpen] = useState(false);
  const title = getDisplayTitle(session);
  const subtitle = getDisplaySubtitle(session);
  const username = session.username || `User #${session.user_id}`;
  const elapsed = getElapsed(session.started_at);
  const historyHref = `/admin/history?user_id=${session.user_id}${session.profile_id ? `&profile_id=${encodeURIComponent(session.profile_id)}` : ""}`;
  const sourceContainer = session.source_container?.trim().toUpperCase();
  const streamBitrate = formatSessionBitrate(session.stream_bitrate_kbps);
  const streamMeta = [sourceContainer, streamBitrate].filter(Boolean).join(" · ");
  const userMeta = session.client_ip?.trim() || "—";
  const videoDecision = session.video_decision || session.play_method;
  const audioDecision =
    session.audio_decision || (session.transcode_audio ? "transcode" : session.play_method);
  const logsHref = `/admin/logs?playback_session_id=${encodeURIComponent(session.session_id)}&focus=playback`;
  const ffmpegLogsHref = `${logsHref}&component=ffmpeg`;
  const ffmpegLogs = useOperationalLogs(
    {
      playback_session_id: session.session_id,
      component: "ffmpeg",
      limit: 12,
    },
    ffmpegOpen,
  );
  const ffmpegRows = ffmpegLogs.data?.entries ?? [];

  return (
    <div
      className={`border-border/30 hover:bg-surface/60 border-b transition-colors duration-100 ${
        even ? "" : "bg-surface/20"
      }`}
    >
      {/* Desktop row */}
      <div className="hidden grid-cols-[minmax(140px,1.5fr)_minmax(220px,2.2fr)_minmax(150px,1.4fr)_minmax(150px,1.4fr)_minmax(90px,1fr)_80px] items-center gap-3 px-4 py-2.5 sm:grid">
        {/* User */}
        <div className="flex min-w-0 items-center gap-2">
          <div
            className="text-primary-foreground flex h-6 w-6 flex-shrink-0 items-center justify-center rounded-full text-[9px] font-bold"
            style={{ background: "var(--primary)" }}
          >
            {username.charAt(0).toUpperCase()}
          </div>
          <div className="min-w-0">
            <Link
              to={historyHref}
              className="hover:text-primary block truncate text-[13px] font-medium transition-colors"
            >
              {username}
            </Link>
            <div className="text-muted-foreground truncate text-[10px]">{userMeta}</div>
          </div>
        </div>

        {/* Stream */}
        <div className="min-w-0">
          {session.content_id ? (
            <Link
              to={`/item/${session.content_id}`}
              className="hover:text-primary block truncate text-[13px] font-medium transition-colors"
            >
              {title}
            </Link>
          ) : (
            <div className="truncate text-[13px] font-medium">{title}</div>
          )}
          {subtitle && <div className="text-muted-foreground truncate text-[10px]">{subtitle}</div>}
          {streamMeta && (
            <div className="text-muted-foreground truncate text-[10px]">{streamMeta}</div>
          )}
        </div>

        {/* Video */}
        <div className="min-w-0">
          <span
            className={`mb-1 inline-flex rounded border px-1.5 py-0.5 text-[9px] font-semibold ${methodBadgeColor(videoDecision)}`}
          >
            {formatDecisionLabel(videoDecision)}
          </span>
          <div className="truncate text-[12px] font-medium">{formatVideoSummary(session)}</div>
          <div className="text-muted-foreground truncate text-[10px]">
            {formatVideoDetail(session)}
          </div>
        </div>

        {/* Audio */}
        <div className="min-w-0">
          <span
            className={`mb-1 inline-flex rounded border px-1.5 py-0.5 text-[9px] font-semibold ${methodBadgeColor(audioDecision)}`}
          >
            {formatDecisionLabel(audioDecision)}
          </span>
          <div className="truncate text-[12px] font-medium">{formatAudioSummary(session)}</div>
          <div className="text-muted-foreground truncate text-[10px]">
            {formatAudioDetail(session)}
          </div>
        </div>

        {/* Node */}
        <div className="min-w-0">
          <div className="text-muted-foreground truncate text-[12px]">
            {session.node_display_name || session.reporting_node || "—"}
          </div>
          {(session.profile_name || session.profile_id) && (
            <div className="text-muted-foreground truncate text-[10px]">
              {session.profile_name || session.profile_id}
            </div>
          )}
        </div>

        {/* Duration */}
        <div className="text-muted-foreground text-right font-mono text-[12px] tabular-nums">
          <div>{elapsed}</div>
          <div className="mt-1 flex items-center justify-end gap-2">
            <AdminSessionActions session={session} compact />
            <button
              type="button"
              onClick={() => setFFmpegOpen((open) => !open)}
              className="text-primary inline-flex items-center gap-1 text-[11px]"
            >
              <Terminal className="h-3 w-3" />
              FFmpeg
              {ffmpegOpen ? <ChevronUp className="h-3 w-3" /> : <ChevronDown className="h-3 w-3" />}
            </button>
            <Link to={logsHref} className="text-primary inline-block text-[11px]">
              View Logs
            </Link>
            <Link to={ffmpegLogsHref} className="text-primary/80 inline-block text-[11px]">
              FFmpeg Logs
            </Link>
          </div>
        </div>
      </div>

      {/* Mobile row */}
      <div className="flex gap-3 px-4 py-3 sm:hidden">
        <div
          className="text-primary-foreground flex h-7 w-7 flex-shrink-0 items-center justify-center rounded-full text-[10px] font-bold"
          style={{ background: "var(--primary)" }}
        >
          {username.charAt(0).toUpperCase()}
        </div>
        <div className="min-w-0 flex-1">
          <div className="flex items-center gap-2">
            <Link
              to={historyHref}
              className="hover:text-primary truncate text-[13px] font-semibold transition-colors"
            >
              {username}
            </Link>
            <span
              className={`inline-flex flex-shrink-0 rounded border px-1.5 py-0.5 text-[9px] font-semibold ${methodBadgeColor(session.play_method)}`}
            >
              {session.play_method || "?"}
            </span>
          </div>
          <div className="text-muted-foreground mt-0.5 flex items-center gap-1.5 text-[11px]">
            {session.content_id ? (
              <Link
                to={`/item/${session.content_id}`}
                className="hover:text-primary truncate transition-colors"
              >
                {title}
              </Link>
            ) : (
              <span className="truncate">{title}</span>
            )}
            <span className="flex-shrink-0">·</span>
            <span className="flex-shrink-0 font-mono tabular-nums">{elapsed}</span>
          </div>
          <div className="text-muted-foreground mt-1 truncate text-[10px]">
            {[userMeta, streamMeta || null].filter(Boolean).join(" · ")}
          </div>
          <div className="mt-2 grid grid-cols-2 gap-2">
            <div className="rounded-md border border-white/6 bg-white/[0.03] px-2 py-1.5">
              <div className="mb-1 flex items-center gap-1.5">
                <span
                  className={`inline-flex rounded border px-1.5 py-0.5 text-[9px] font-semibold ${methodBadgeColor(videoDecision)}`}
                >
                  {formatDecisionLabel(videoDecision)}
                </span>
                <span className="text-muted-foreground text-[9px] tracking-wide uppercase">
                  Video
                </span>
              </div>
              <div className="truncate text-[11px] font-medium">{formatVideoSummary(session)}</div>
              <div className="text-muted-foreground truncate text-[10px]">
                {formatVideoDetail(session)}
              </div>
            </div>
            <div className="rounded-md border border-white/6 bg-white/[0.03] px-2 py-1.5">
              <div className="mb-1 flex items-center gap-1.5">
                <span
                  className={`inline-flex rounded border px-1.5 py-0.5 text-[9px] font-semibold ${methodBadgeColor(audioDecision)}`}
                >
                  {formatDecisionLabel(audioDecision)}
                </span>
                <span className="text-muted-foreground text-[9px] tracking-wide uppercase">
                  Audio
                </span>
              </div>
              <div className="truncate text-[11px] font-medium">{formatAudioSummary(session)}</div>
              <div className="text-muted-foreground truncate text-[10px]">
                {formatAudioDetail(session)}
              </div>
            </div>
          </div>
          <div className="mt-2 flex items-center gap-3">
            <AdminSessionActions session={session} compact />
            <button
              type="button"
              onClick={() => setFFmpegOpen((open) => !open)}
              className="text-primary inline-flex items-center gap-1 text-[11px] font-medium"
            >
              <Terminal className="h-3 w-3" />
              FFmpeg
              {ffmpegOpen ? <ChevronUp className="h-3 w-3" /> : <ChevronDown className="h-3 w-3" />}
            </button>
            <Link to={logsHref} className="text-primary inline-block text-[11px] font-medium">
              View Logs
            </Link>
            <Link
              to={ffmpegLogsHref}
              className="text-primary/80 inline-block text-[11px] font-medium"
            >
              FFmpeg Logs
            </Link>
          </div>
        </div>
      </div>

      {ffmpegOpen && (
        <FFmpegLogPanel
          sessionID={session.session_id}
          rows={ffmpegRows}
          isLoading={ffmpegLogs.isLoading}
          isFetching={ffmpegLogs.isFetching}
          logsHref={`${logsHref}&component=ffmpeg`}
        />
      )}
    </div>
  );
}

function FFmpegLogPanel({
  sessionID,
  rows,
  isLoading,
  isFetching,
  logsHref,
}: {
  sessionID: string;
  rows: OperationalLogEntry[];
  isLoading: boolean;
  isFetching: boolean;
  logsHref: string;
}) {
  return (
    <div className="terminal-surface border-border/50 bg-card border-t px-4 py-3">
      <div className="mb-2 flex items-center justify-between gap-3">
        <div>
          <div className="flex items-center gap-2">
            <div className="rounded-full border border-[var(--terminal-border)] bg-[var(--terminal-bg)] px-2 py-0.5 text-[10px] font-semibold tracking-[0.2em] text-[var(--terminal-fg)] uppercase">
              FFmpeg
            </div>
            <div className="text-foreground/85 text-[11px] font-medium">Live transcode console</div>
          </div>
          <div className="text-muted-foreground mt-1 font-mono text-[10px]">{sessionID}</div>
        </div>
        <div className="flex items-center gap-3">
          {isFetching && <div className="text-muted-foreground text-[10px]">Refreshing…</div>}
          <Link
            to={logsHref}
            className="text-[11px] font-medium text-[var(--terminal-fg)] hover:text-[var(--terminal-fg)]/80"
          >
            Open full ffmpeg logs
          </Link>
        </div>
      </div>

      <div className="overflow-hidden rounded-xl border border-[var(--terminal-border)] bg-[var(--terminal-bg)] shadow-[0_18px_60px_rgba(0,0,0,0.35)]">
        {isLoading ? (
          <div className="px-4 py-6 font-mono text-[11px] text-[var(--terminal-muted)]">
            Loading ffmpeg output…
          </div>
        ) : rows.length === 0 ? (
          <div className="px-4 py-6 font-mono text-[11px] text-[var(--terminal-muted)]">
            No ffmpeg rows yet for this session. If the session is direct play or remux without a
            transcode worker, nothing will appear here.
          </div>
        ) : (
          <div className="max-h-64 overflow-y-auto">
            {rows.map((row) => (
              <div
                key={row.id}
                className="grid grid-cols-[120px_1fr] gap-3 border-b border-[var(--terminal-border)]/30 px-4 py-2.5 last:border-b-0"
              >
                <div className="space-y-1">
                  <div className="font-mono text-[10px] text-[var(--terminal-muted)]">
                    {formatTimeOnly(row.timestamp)}
                  </div>
                  <div className="text-[10px] tracking-[0.18em] text-[var(--terminal-muted)]/60 uppercase">
                    {row.message.includes("stderr") ? "stderr" : "event"}
                  </div>
                </div>
                <div className="min-w-0">
                  <div className="font-mono text-[11px] leading-5 break-words text-[var(--terminal-fg)]">
                    {ffmpegRowText(row)}
                  </div>
                  <div className="mt-1 flex flex-wrap gap-x-3 gap-y-1 text-[10px] text-[var(--terminal-muted)]/60">
                    {row.node_id && <span>{row.node_id}</span>}
                    {stringAttr(row, "target_resolution") !== "-" && (
                      <span>{stringAttr(row, "target_resolution")}</span>
                    )}
                    {stringAttr(row, "hw_accel") !== "-" && (
                      <span>{stringAttr(row, "hw_accel")}</span>
                    )}
                    {stringAttr(row, "restart_count") !== "-" && (
                      <span>restart {stringAttr(row, "restart_count")}</span>
                    )}
                  </div>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}

function SortHeader({
  field,
  current,
  dir,
  onClick,
  children,
  className = "",
}: {
  field: SortField;
  current: SortField;
  dir: SortDir;
  onClick: (field: SortField) => void;
  children: ReactNode;
  className?: string;
}) {
  const active = current === field;
  return (
    <button
      onClick={() => onClick(field)}
      className={`text-muted-foreground flex items-center gap-1 text-[10px] font-semibold tracking-wider uppercase transition-colors ${
        active ? "text-foreground" : "hover:text-foreground"
      } ${className}`}
    >
      {children}
      {active && <span className="text-[8px]">{dir === "asc" ? "▲" : "▼"}</span>}
    </button>
  );
}

function formatConnectionState(state: "connecting" | "live" | "disconnected") {
  switch (state) {
    case "connecting":
      return "Connecting";
    case "live":
      return "Live";
    default:
      return "Disconnected";
  }
}

function ffmpegRowText(entry: OperationalLogEntry) {
  const line = stringAttr(entry, "ffmpeg_line");
  if (line !== "-") return line;
  const event = stringAttr(entry, "ffmpeg_event");
  if (event !== "-") return event;
  return entry.message;
}

function formatTimeOnly(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleTimeString();
}

function EmptyState({ hasData }: { hasData: boolean }) {
  return (
    <div className="text-muted-foreground flex flex-col items-center justify-center py-20 text-sm">
      {hasData ? (
        <>
          <Filter className="mb-3 h-8 w-8 opacity-20" />
          <span>No streams match your filters</span>
        </>
      ) : (
        <>
          <Play className="mb-3 h-8 w-8 opacity-20" />
          <span>No active streams</span>
        </>
      )}
    </div>
  );
}

// --- Helpers ---

function getDisplayTitle(session: AdminSession): string {
  if (session.series_name && session.season_number != null && session.episode_number != null) {
    return session.episode_name || `S${session.season_number}E${session.episode_number}`;
  }
  return session.media_title || `File #${session.media_file_id}`;
}

function getDisplaySubtitle(session: AdminSession): string | null {
  if (session.series_name && session.season_number != null && session.episode_number != null) {
    const ep = `S${session.season_number}E${session.episode_number}`;
    return session.series_name ? `${ep} · ${session.series_name}` : ep;
  }
  if (session.media_type === "movie") return "Movie";
  if (session.media_type === "series") return "Series";
  return null;
}

function getElapsed(dateStr: string): string {
  const diff = Math.max(0, Date.now() - new Date(dateStr).getTime());
  const totalSec = Math.floor(diff / 1000);
  const h = Math.floor(totalSec / 3600);
  const m = Math.floor((totalSec % 3600) / 60);
  const s = totalSec % 60;
  if (h > 0) return `${h}:${String(m).padStart(2, "0")}:${String(s).padStart(2, "0")}`;
  return `${m}:${String(s).padStart(2, "0")}`;
}

function stringAttr(entry: OperationalLogEntry, key: string) {
  const value = entry.attrs?.[key];
  if (typeof value === "string" && value.length > 0) return value;
  if (typeof value === "number") return String(value);
  return "-";
}

function methodBadgeColor(method: string): string {
  switch (method) {
    case "direct":
      return "bg-success/10 text-success border-success/15";
    case "remux":
      return "bg-info/10 text-info border-info/15";
    case "transcode":
      return "bg-warning/10 text-warning border-warning/15";
    default:
      return "bg-surface text-muted-foreground border-border";
  }
}

function methodBarColor(method: string): string {
  switch (method) {
    case "direct":
      return "bg-success";
    case "remux":
      return "bg-info";
    case "transcode":
      return "bg-warning";
    default:
      return "bg-muted-foreground";
  }
}

function methodDotColor(method: string): string {
  switch (method) {
    case "direct":
      return "bg-success";
    case "remux":
      return "bg-info";
    case "transcode":
      return "bg-warning";
    default:
      return "bg-muted-foreground";
  }
}
