import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { useLocation, useParams, useSearchParams } from "react-router";
import {
  ChevronLeft,
  ChevronRight,
  Link,
  Play,
  Search,
  ShieldCheck,
  Users,
  X,
  Zap,
} from "lucide-react";
import { ApiClientError } from "@/api/client";
import { type BrowseItem } from "@/api/types";
import { Button } from "@/components/ui/button";
import { fetchCatalogPage, createCatalogSearchState } from "@/hooks/queries/catalog";
import { useCatalogItemDetail } from "@/hooks/queries/catalogRead";
import { useSeasons, useSeasonEpisodes } from "@/hooks/queries/episodes";
import { buildWatchTogetherInviteUrl } from "@/lib/watchTogether";
import { storage } from "@/utils/storage";
import { useWatchPlaybackController } from "@/playback/watchPlaybackContext";
import { useWatchTogetherRoomConnection } from "@/player/hooks/useWatchTogetherRoomConnection";
import { useDebounce } from "@/hooks/useDebounce";
import { toast } from "sonner";
import { decodeThumbhash } from "@/lib/thumbhash";
import { WatchTogetherSuggestionPanel } from "./WatchTogetherSuggestionPanel";

function describeRoomError(error: unknown) {
  if (error === "host_left" || error === "room_closed") {
    return "The room has ended.";
  }
  if (error instanceof ApiClientError) {
    if (error.status === 404) {
      return "Room not found.";
    }
    if (error.status === 410) {
      return "That room is no longer active.";
    }
    return error.message;
  }
  return error instanceof Error ? error.message : "Room is unavailable.";
}

function resultLabel(item: BrowseItem) {
  if (item.type === "episode") return `Episode${item.year > 0 ? ` · ${item.year}` : ""}`;
  if (item.type === "series") return `Series${item.year > 0 ? ` · ${item.year}` : ""}`;
  return `Movie${item.year > 0 ? ` · ${item.year}` : ""}`;
}

type SearchStep =
  | { stage: "results" }
  | { stage: "seasons"; series: BrowseItem }
  | { stage: "episodes"; series: BrowseItem; seasonNumber: number };

type PendingAction = "select" | "suggest";
type WatchTogetherRoomLocationState = {
  suppressAutoStartSelection?: {
    contentId: string;
    fileId?: number;
    libraryId?: number;
  };
};

/* ─── Poster card for search results ─── */
function SearchPosterCard({
  item,
  selected,
  onClick,
  index,
}: {
  item: BrowseItem;
  selected: boolean;
  onClick: () => void;
  index: number;
}) {
  const [loaded, setLoaded] = useState(false);
  const thumbhashUrl = item.poster_thumbhash ? decodeThumbhash(item.poster_thumbhash) : "";

  return (
    <button
      type="button"
      onClick={onClick}
      className="media-card group/card text-left"
      style={{ animationDelay: `${index * 50}ms` }}
    >
      <div
        className={`media-card-image relative aspect-[2/3] transition-all duration-200 ${
          selected ? "ring-primary ring-offset-background ring-2 ring-offset-2" : ""
        }`}
        style={
          thumbhashUrl
            ? {
                backgroundImage: `url(${thumbhashUrl})`,
                backgroundSize: "cover",
                backgroundPosition: "center",
              }
            : undefined
        }
      >
        {item.poster_url ? (
          <img
            src={item.poster_url}
            alt={item.title}
            className={`h-full w-full object-cover transition-opacity duration-300 ${
              loaded ? "opacity-100" : "opacity-0"
            }`}
            loading="lazy"
            onLoad={() => setLoaded(true)}
          />
        ) : (
          <div className="text-muted-foreground flex h-full w-full flex-col items-center justify-center gap-1 p-3 text-center text-xs">
            <span className="line-clamp-3 font-medium">{item.title}</span>
          </div>
        )}
        <div className="from-background/70 pointer-events-none absolute inset-x-0 bottom-0 h-20 bg-gradient-to-t to-transparent" />

        {/* Type badge */}
        {item.type === "series" && (
          <span className="glass-subtle absolute right-2 bottom-2 flex items-center gap-1 rounded-full border border-white/15 px-2 py-0.5 text-[10px] font-semibold tracking-wide text-white/90">
            Series <ChevronRight className="size-2.5" />
          </span>
        )}
      </div>
      <div className="px-0.5 pt-2.5">
        <div className="truncate text-[13px] font-semibold tracking-tight">{item.title}</div>
        <div className="text-muted-foreground mt-0.5 text-[11px] font-medium tracking-[0.12em] uppercase">
          {resultLabel(item)}
        </div>
      </div>
    </button>
  );
}

/* ─── Spotlight card for candidate confirmation ─── */
function CandidateSpotlight({
  candidate,
  candidateContext,
  pendingAction,
  submitting,
  isPlaying,
  onConfirm,
  onDismiss,
}: {
  candidate: BrowseItem;
  candidateContext: string | null;
  pendingAction: PendingAction;
  submitting: boolean;
  isPlaying: boolean;
  onConfirm: () => void;
  onDismiss: () => void;
}) {
  const [backdropLoaded, setBackdropLoaded] = useState(false);
  const [posterLoaded, setPosterLoaded] = useState(false);
  const thumbhashUrl = candidate.poster_thumbhash
    ? decodeThumbhash(candidate.poster_thumbhash)
    : "";

  const actionLabel = submitting
    ? "Updating..."
    : pendingAction === "suggest"
      ? "Suggest This"
      : isPlaying
        ? "Switch Everyone"
        : "Start for Everyone";

  return (
    <div className="animate-fade-in relative overflow-hidden rounded-xl border border-white/10">
      {/* Backdrop */}
      {candidate.backdrop_url && (
        <img
          src={candidate.backdrop_url}
          alt=""
          className={`absolute inset-0 h-full w-full object-cover transition-opacity duration-500 ${
            backdropLoaded ? "opacity-30" : "opacity-0"
          }`}
          onLoad={() => setBackdropLoaded(true)}
        />
      )}
      <div className="from-background via-background/95 absolute inset-0 bg-gradient-to-r to-transparent" />
      <div className="from-background/60 absolute inset-0 bg-gradient-to-t to-transparent" />

      <div className="relative flex gap-5 p-5 sm:p-6">
        {/* Poster */}
        <div
          className="hidden w-28 shrink-0 sm:block"
          style={
            thumbhashUrl
              ? {
                  backgroundImage: `url(${thumbhashUrl})`,
                  backgroundSize: "cover",
                }
              : undefined
          }
        >
          <div className="media-card-image aspect-[2/3] overflow-hidden">
            {candidate.poster_url ? (
              <img
                src={candidate.poster_url}
                alt={candidate.title}
                className={`h-full w-full object-cover transition-opacity duration-300 ${
                  posterLoaded ? "opacity-100" : "opacity-0"
                }`}
                onLoad={() => setPosterLoaded(true)}
              />
            ) : (
              <div className="bg-surface flex h-full items-center justify-center text-xs">
                No poster
              </div>
            )}
          </div>
        </div>

        {/* Info */}
        <div className="flex min-w-0 flex-1 flex-col justify-center gap-3">
          <div>
            <div className="text-muted-foreground text-[10px] font-semibold tracking-[0.18em] uppercase">
              {pendingAction === "suggest" ? "Your Suggestion" : "Ready to Play"}
            </div>
            <h3 className="mt-1.5 text-xl font-semibold tracking-tight sm:text-2xl">
              {candidate.title}
            </h3>
            <div className="text-muted-foreground mt-1 text-sm">
              {candidateContext ?? resultLabel(candidate)}
            </div>
          </div>

          {candidate.overview && (
            <p className="text-muted-foreground line-clamp-2 max-w-lg text-sm leading-relaxed">
              {candidate.overview}
            </p>
          )}

          <div className="mt-1 flex items-center gap-3">
            <Button type="button" onClick={onConfirm} disabled={submitting} className="gap-2">
              {pendingAction === "suggest" ? (
                <Zap className="size-3.5" />
              ) : (
                <Play className="size-3.5" />
              )}
              {actionLabel}
            </Button>
            <button
              type="button"
              onClick={onDismiss}
              className="text-muted-foreground hover:text-foreground flex items-center gap-1.5 text-sm transition-colors"
            >
              <X className="size-3.5" />
              Cancel
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

/* ─── Now Playing hero card ─── */
function NowPlayingHero({
  title,
  type,
  backdropUrl,
}: {
  title: string;
  type: string;
  backdropUrl?: string;
}) {
  const [loaded, setLoaded] = useState(false);

  return (
    <div className="relative overflow-hidden rounded-xl border border-white/10">
      {backdropUrl && (
        <img
          src={backdropUrl}
          alt=""
          className={`absolute inset-0 h-full w-full object-cover transition-opacity duration-700 ${
            loaded ? "opacity-25" : "opacity-0"
          }`}
          onLoad={() => setLoaded(true)}
        />
      )}
      <div className="from-background via-background/90 absolute inset-0 bg-gradient-to-r to-transparent" />

      <div className="relative px-6 py-6 sm:py-8">
        <div className="flex items-center gap-2">
          <div className="h-2 w-2 animate-pulse rounded-full bg-green-400" />
          <span className="text-[11px] font-semibold tracking-[0.18em] text-green-400/90 uppercase">
            Now Playing
          </span>
        </div>
        <h3 className="mt-2 text-2xl font-semibold tracking-tight sm:text-3xl">{title}</h3>
        <div className="text-muted-foreground mt-1 text-sm capitalize">{type}</div>
      </div>
    </div>
  );
}

/* ─── Connection status dot ─── */
function StatusDot({ state }: { state: string }) {
  const color =
    state === "connected"
      ? "bg-green-400"
      : state === "connecting"
        ? "bg-yellow-400 animate-pulse"
        : "bg-red-400";
  return <span className={`inline-block h-2 w-2 rounded-full ${color}`} />;
}

export default function WatchTogetherRoomPage() {
  const { roomId } = useParams<{ roomId: string }>();
  const location = useLocation();
  const [searchParams] = useSearchParams();
  const roomToken = searchParams.get("room_token");
  const playbackController = useWatchPlaybackController();
  const activePlaybackRequest = playbackController.state.request;
  const suppressRoomConnection =
    playbackController.state.mode === "foreground" &&
    activePlaybackRequest?.roomId === roomId &&
    activePlaybackRequest?.roomToken === roomToken;
  const roomConnection = useWatchTogetherRoomConnection({
    roomId: suppressRoomConnection ? null : roomId,
    roomToken: suppressRoomConnection ? null : roomToken,
  });
  const [query, setQuery] = useState("");
  const [candidate, setCandidate] = useState<BrowseItem | null>(null);
  const [candidateContext, setCandidateContext] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [searchStep, setSearchStep] = useState<SearchStep>({ stage: "results" });
  const lastAutoStartRevisionRef = useRef<number | null>(null);
  const suppressAutoStartSelectionRef = useRef(
    (location.state as WatchTogetherRoomLocationState | null)?.suppressAutoStartSelection ?? null,
  );
  const searchInputRef = useRef<HTMLInputElement>(null);
  const debouncedQuery = useDebounce(query.trim(), 200);

  useEffect(() => {
    suppressAutoStartSelectionRef.current =
      (location.state as WatchTogetherRoomLocationState | null)?.suppressAutoStartSelection ?? null;
  }, [location.state]);

  const searchState = useMemo(
    () => createCatalogSearchState("query", debouncedQuery ? { q: debouncedQuery } : {}),
    [debouncedQuery],
  );

  const searchQuery = useQuery({
    queryKey: ["watch-together-room-search", debouncedQuery],
    queryFn: ({ signal }) => fetchCatalogPage(searchState, 12, 0, { signal }),
    enabled: debouncedQuery.length > 0,
    staleTime: 60_000,
  });

  const searchResults = useMemo(
    () =>
      (searchQuery.data?.items ?? []).filter(
        (item) => item.type === "movie" || item.type === "series",
      ),
    [searchQuery.data?.items],
  );

  const seasonsQuery = useSeasons(
    searchStep.stage !== "results" ? searchStep.series.content_id : undefined,
  );
  const episodesQuery = useSeasonEpisodes(
    searchStep.stage === "episodes" ? searchStep.series.content_id : undefined,
    searchStep.stage === "episodes" ? searchStep.seasonNumber : -1,
  );

  const selectedContentId = roomConnection.room?.selected_content_id;
  const selectedLibraryId = roomConnection.room?.selected_library_id;
  const selectedItemQuery = useCatalogItemDetail(selectedContentId, selectedLibraryId);

  useEffect(() => {
    const room = roomConnection.room;
    if (!room || !roomId || !roomToken) {
      return;
    }

    const suppressedSelection = suppressAutoStartSelectionRef.current;
    if (suppressedSelection) {
      suppressAutoStartSelectionRef.current = null;
      const sameSelection =
        room.selected_content_id === suppressedSelection.contentId &&
        (room.selected_file_id ?? null) === (suppressedSelection.fileId ?? null) &&
        (room.selected_library_id ?? null) === (suppressedSelection.libraryId ?? null);
      if (sameSelection) {
        lastAutoStartRevisionRef.current = room.selection_revision;
        return;
      }
    }

    if (room.phase !== "playing" || !room.selected_content_id) {
      return;
    }
    if (lastAutoStartRevisionRef.current === room.selection_revision) {
      return;
    }
    lastAutoStartRevisionRef.current = room.selection_revision;
    playbackController.startPlayback({
      contentId: room.selected_content_id,
      fileId: room.selected_file_id,
      libraryId: room.selected_library_id,
      roomId,
      roomToken,
      restart: true,
    });
  }, [playbackController, roomConnection.room, roomId, roomToken]);

  const inviteUrl = buildWatchTogetherInviteUrl(roomConnection.room?.invite_path);
  const isHost = roomConnection.room?.self_can_manage_room === true;
  const isVoteMode = roomConnection.room?.selection_mode === "vote";
  const pendingAction: PendingAction = isVoteMode ? "suggest" : "select";
  const roomError = !roomId || !roomToken ? "Room token is required." : roomConnection.closedReason;

  const handleCopyInvite = useCallback(async () => {
    if (!inviteUrl) {
      return;
    }
    await navigator.clipboard.writeText(inviteUrl);
    toast.success(`Invite copied. Room code ${roomConnection.room?.code ?? ""}`.trim());
  }, [inviteUrl, roomConnection.room?.code]);

  const handleTogglePolicy = useCallback(async () => {
    const room = roomConnection.room;
    if (!room) {
      return;
    }
    await roomConnection.updatePolicy(
      room.guest_control_policy === "guest_play_pause" ? "host_only" : "guest_play_pause",
    );
  }, [roomConnection]);

  const handleConfirmCandidate = useCallback(async () => {
    if (!candidate || !roomId) {
      return;
    }
    setSubmitting(true);
    try {
      if (pendingAction === "suggest") {
        await roomConnection.createSuggestion({
          content_id: candidate.content_id,
          content_type: candidate.type === "episode" ? "episode" : "movie",
          title: candidate.title,
          subtitle: candidateContext ?? "",
          poster_url: candidate.poster_url ?? "",
        });
        setCandidate(null);
        setCandidateContext(null);
        toast.success("Suggestion added");
      } else {
        await roomConnection.selectItem({
          content_id: candidate.content_id,
        });
      }
    } catch (error) {
      toast.error(error instanceof Error ? error.message : "Failed to update room");
    } finally {
      setSubmitting(false);
    }
  }, [candidate, candidateContext, pendingAction, roomConnection, roomId]);

  const handleSelectSearchResult = useCallback((item: BrowseItem) => {
    if (item.type === "series") {
      setSearchStep({ stage: "seasons", series: item });
    } else {
      setCandidate(item);
      setCandidateContext(null);
    }
  }, []);

  const handleSelectSeason = useCallback(
    (seasonNumber: number) => {
      if (searchStep.stage === "seasons") {
        setSearchStep({ stage: "episodes", series: searchStep.series, seasonNumber });
      }
    },
    [searchStep],
  );

  const handleSelectEpisode = useCallback(
    (episode: { content_id: string; title: string; episode_number: number }) => {
      if (searchStep.stage === "episodes") {
        const seriesTitle = searchStep.series.title;
        const context = `${seriesTitle} · S${searchStep.seasonNumber} E${episode.episode_number}`;
        setCandidate({
          content_id: episode.content_id,
          type: "episode",
          title: episode.title,
          year: searchStep.series.year,
          genres: [],
          content_rating: "",
          status: "matched",
          rating_imdb: null,
          overview: "",
          poster_url: searchStep.series.poster_url,
          poster_thumbhash: "",
          backdrop_url: "",
          backdrop_thumbhash: "",
        } as BrowseItem);
        setCandidateContext(context);
        setSearchStep({ stage: "results" });
      }
    },
    [searchStep],
  );

  const handleBackSearchStep = useCallback(() => {
    if (searchStep.stage === "episodes") {
      setSearchStep({ stage: "seasons", series: searchStep.series });
    } else {
      setSearchStep({ stage: "results" });
    }
  }, [searchStep]);

  const handleDismissCandidate = useCallback(() => {
    setCandidate(null);
    setCandidateContext(null);
  }, []);

  if (!roomId || !roomToken) {
    return <div className="mx-auto max-w-5xl px-6 py-10 text-sm">Room token is required.</div>;
  }

  const isPlaying = roomConnection.room?.phase === "playing";
  const canSearch = isHost || isVoteMode;
  const hasSearchQuery = debouncedQuery.length > 0;
  const showSearchResults = searchStep.stage === "results" && hasSearchQuery;
  const showDrillDown = searchStep.stage !== "results";

  return (
    <div className="mx-auto flex w-full max-w-5xl flex-col gap-6 px-4 py-6 sm:px-6 sm:py-8">
      {/* ─── Room header bar ─── */}
      <div className="glass-subtle flex flex-wrap items-center justify-between gap-3 rounded-xl px-4 py-3 sm:px-5">
        <div className="flex items-center gap-4">
          <div>
            <div className="text-muted-foreground text-[10px] font-semibold tracking-[0.2em] uppercase">
              Watch Party
            </div>
            <div className="mt-0.5 text-lg font-semibold tracking-tight">
              {roomConnection.room?.code ?? roomId}
            </div>
          </div>

          <div className="bg-border hidden h-8 w-px sm:block" />

          <div className="hidden items-center gap-4 sm:flex">
            <div className="flex items-center gap-2 text-sm">
              <Users className="text-muted-foreground size-3.5" />
              <span>{roomConnection.room?.member_count ?? 0}</span>
            </div>
            <div className="flex items-center gap-2 text-sm">
              <StatusDot state={roomConnection.connectionState} />
              <span className="text-muted-foreground">
                {roomConnection.connectionState === "connected"
                  ? "Connected"
                  : roomConnection.connectionState === "connecting"
                    ? "Connecting..."
                    : "Disconnected"}
              </span>
            </div>
          </div>
        </div>

        <div className="flex items-center gap-2">
          {inviteUrl ? (
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => void handleCopyInvite()}
              className="gap-1.5"
            >
              <Link className="size-3.5" />
              <span className="hidden sm:inline">Invite</span>
            </Button>
          ) : null}
          {isHost ? (
            <>
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => void handleTogglePolicy()}
                className="gap-1.5"
              >
                <ShieldCheck className="size-3.5" />
                <span className="hidden sm:inline">
                  {roomConnection.room?.guest_control_policy === "guest_play_pause"
                    ? "Host Only"
                    : "Allow Pause"}
                </span>
              </Button>
              <Button
                type="button"
                variant="destructive"
                size="sm"
                onClick={() => void roomConnection.closeRoom()}
              >
                End
              </Button>
            </>
          ) : null}
        </div>
      </div>

      {/* ─── Error state ─── */}
      {roomError ? (
        <div className="rounded-xl border border-red-500/20 bg-red-500/10 px-5 py-4 text-sm text-red-300">
          {describeRoomError(roomError)}
        </div>
      ) : null}

      {/* ─── Now Playing hero ─── */}
      {isPlaying && roomConnection.room?.selected_content_id ? (
        <NowPlayingHero
          title={selectedItemQuery.data?.title ?? "Loading..."}
          type={selectedItemQuery.data?.type === "episode" ? "Episode" : "Movie"}
          backdropUrl={selectedItemQuery.data?.backdrop_url}
        />
      ) : null}

      {/* ─── Candidate spotlight ─── */}
      {candidate ? (
        <CandidateSpotlight
          candidate={candidate}
          candidateContext={candidateContext}
          pendingAction={pendingAction}
          submitting={submitting}
          isPlaying={isPlaying ?? false}
          onConfirm={() => void handleConfirmCandidate()}
          onDismiss={handleDismissCandidate}
        />
      ) : null}

      {/* ─── Search & discovery ─── */}
      {canSearch ? (
        <section>
          {/* Search bar */}
          <div className="relative">
            <Search className="text-muted-foreground pointer-events-none absolute top-1/2 left-4 size-4 -translate-y-1/2" />
            <input
              ref={searchInputRef}
              value={query}
              onChange={(event) => {
                setQuery(event.target.value);
                setSearchStep({ stage: "results" });
              }}
              placeholder={
                isVoteMode ? "Search to suggest something..." : "Search movies and series..."
              }
              className="border-border bg-surface placeholder:text-muted-foreground h-12 w-full rounded-xl border py-3 pr-4 pl-11 text-sm shadow-sm transition-all duration-200 outline-none focus:border-white/30 focus:ring-1 focus:ring-white/10"
            />
            {query && (
              <button
                type="button"
                onClick={() => {
                  setQuery("");
                  setSearchStep({ stage: "results" });
                }}
                className="text-muted-foreground hover:text-foreground absolute top-1/2 right-4 -translate-y-1/2 transition-colors"
              >
                <X className="size-4" />
              </button>
            )}
          </div>

          {/* Drill-down: Seasons / Episodes */}
          {showDrillDown ? (
            <div className="mt-5">
              <button
                type="button"
                onClick={handleBackSearchStep}
                className="text-muted-foreground hover:text-foreground mb-4 flex items-center gap-1.5 text-sm font-medium transition-colors"
              >
                <ChevronLeft className="size-3.5" />
                {searchStep.stage === "episodes" ? searchStep.series.title : "Back to results"}
              </button>

              {searchStep.stage === "seasons" ? (
                <div>
                  <div className="mb-3 flex items-center gap-3">
                    {searchStep.series.poster_url && (
                      <img
                        src={searchStep.series.poster_url}
                        alt=""
                        className="h-16 w-11 shrink-0 rounded-lg object-cover"
                      />
                    )}
                    <div>
                      <h3 className="text-lg font-semibold tracking-tight">
                        {searchStep.series.title}
                      </h3>
                      <div className="text-muted-foreground text-sm">Pick a season</div>
                    </div>
                  </div>

                  <div className="mt-4 grid grid-cols-2 gap-2 sm:grid-cols-3 md:grid-cols-4">
                    {seasonsQuery.isLoading
                      ? Array.from({ length: 4 }).map((_, i) => (
                          <div
                            key={i}
                            className="bg-surface animate-pulse rounded-xl border border-white/5 px-4 py-5"
                          >
                            <div className="bg-muted h-4 w-20 rounded" />
                            <div className="bg-muted mt-2 h-3 w-14 rounded" />
                          </div>
                        ))
                      : (seasonsQuery.data?.seasons ?? []).map((season) => (
                          <button
                            key={season.season_number}
                            type="button"
                            onClick={() => handleSelectSeason(season.season_number)}
                            className="group bg-surface hover:bg-surface-hover flex items-center justify-between rounded-xl border border-white/5 px-4 py-4 text-left transition-all duration-200 hover:border-white/15"
                          >
                            <div>
                              <div className="text-sm font-semibold">
                                {season.is_specials ? "Specials" : `Season ${season.season_number}`}
                              </div>
                              <div className="text-muted-foreground mt-0.5 text-xs">
                                {season.episode_count} episode
                                {season.episode_count !== 1 ? "s" : ""}
                              </div>
                            </div>
                            <ChevronRight className="text-muted-foreground size-4 transition-transform group-hover:translate-x-0.5" />
                          </button>
                        ))}
                  </div>
                </div>
              ) : null}

              {searchStep.stage === "episodes" ? (
                <div>
                  <div className="mb-4 flex items-center gap-3">
                    {searchStep.series.poster_url && (
                      <img
                        src={searchStep.series.poster_url}
                        alt=""
                        className="h-16 w-11 shrink-0 rounded-lg object-cover"
                      />
                    )}
                    <div>
                      <h3 className="text-lg font-semibold tracking-tight">
                        {searchStep.series.title}
                      </h3>
                      <div className="text-muted-foreground text-sm">
                        Season {searchStep.seasonNumber}
                      </div>
                    </div>
                  </div>

                  <div className="grid gap-1.5">
                    {episodesQuery.isLoading
                      ? Array.from({ length: 6 }).map((_, i) => (
                          <div
                            key={i}
                            className="bg-surface animate-pulse rounded-lg border border-white/5 px-4 py-4"
                          >
                            <div className="flex items-center gap-3">
                              <div className="bg-muted h-5 w-6 rounded" />
                              <div className="bg-muted h-4 w-40 rounded" />
                            </div>
                          </div>
                        ))
                      : (episodesQuery.data?.episodes ?? [])
                          .filter((ep) => ep.files.length > 0)
                          .map((episode) => (
                            <button
                              key={episode.content_id}
                              type="button"
                              onClick={() => handleSelectEpisode(episode)}
                              className="group flex items-start gap-4 rounded-lg border border-white/5 px-4 py-3.5 text-left transition-all duration-200 hover:border-white/15 hover:bg-white/[0.03]"
                            >
                              <span className="text-muted-foreground mt-0.5 w-6 shrink-0 text-center text-sm font-semibold tabular-nums">
                                {episode.episode_number}
                              </span>
                              <div className="min-w-0 flex-1">
                                <div className="text-sm font-medium group-hover:text-white">
                                  {episode.title}
                                </div>
                                {episode.overview && (
                                  <div className="text-muted-foreground mt-1 line-clamp-2 text-xs leading-relaxed">
                                    {episode.overview}
                                  </div>
                                )}
                              </div>
                              <Play className="text-muted-foreground mt-1 size-3.5 shrink-0 opacity-0 transition-opacity group-hover:opacity-100" />
                            </button>
                          ))}
                    {!episodesQuery.isLoading &&
                    (episodesQuery.data?.episodes ?? []).filter((ep) => ep.files.length > 0)
                      .length === 0 ? (
                      <div className="text-muted-foreground rounded-lg border border-white/5 px-4 py-8 text-center text-sm">
                        No playable episodes in this season.
                      </div>
                    ) : null}
                  </div>
                </div>
              ) : null}
            </div>
          ) : null}

          {/* Search results poster grid */}
          {showSearchResults ? (
            <div className="mt-5">
              {searchQuery.isFetching ? (
                <div className="grid grid-cols-3 gap-3 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-6">
                  {Array.from({ length: 6 }).map((_, i) => (
                    <div key={i}>
                      <div className="bg-surface aspect-[2/3] animate-pulse rounded-xl" />
                      <div className="bg-surface mt-2.5 h-3.5 w-3/4 animate-pulse rounded" />
                      <div className="bg-surface mt-1.5 h-3 w-1/2 animate-pulse rounded" />
                    </div>
                  ))}
                </div>
              ) : searchResults.length > 0 ? (
                <div className="grid grid-cols-3 gap-3 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-6">
                  {searchResults.map((item, index) => (
                    <SearchPosterCard
                      key={item.content_id}
                      item={item}
                      selected={candidate?.content_id === item.content_id}
                      onClick={() => handleSelectSearchResult(item)}
                      index={index}
                    />
                  ))}
                </div>
              ) : (
                <div className="text-muted-foreground py-12 text-center text-sm">
                  No matches found. Try a different search.
                </div>
              )}
            </div>
          ) : null}

          {/* Empty state — no search yet */}
          {!hasSearchQuery && !showDrillDown && !candidate ? (
            <div className="text-muted-foreground mt-8 flex flex-col items-center gap-2 py-8 text-center">
              <Search className="size-8 opacity-30" />
              <p className="text-sm">
                {isPlaying
                  ? "Search to switch what everyone is watching."
                  : isVoteMode
                    ? "Search for something to suggest to the room."
                    : "Find something for everyone to watch."}
              </p>
            </div>
          ) : null}
        </section>
      ) : (
        /* Guest waiting state */
        <section className="flex flex-col items-center gap-3 py-16 text-center">
          <div className="bg-surface flex size-16 items-center justify-center rounded-2xl border border-white/10">
            <Users className="text-muted-foreground size-7" />
          </div>
          <h2 className="mt-2 text-lg font-semibold">Waiting for the host</h2>
          <p className="text-muted-foreground max-w-sm text-sm">
            {isPlaying
              ? "The host already started playback. You'll enter together."
              : "The host will choose a movie or episode for the room."}
          </p>
        </section>
      )}

      {/* ─── Suggestions (vote mode) ─── */}
      {isVoteMode && roomId && roomToken ? (
        <>
          <div className="border-border/50 border-t" />
          <WatchTogetherSuggestionPanel
            suggestions={roomConnection.suggestions}
            isHost={isHost}
            currentProfileId={storage.get(storage.KEYS.PROFILE_ID) ?? ""}
            onVote={(id: string) => roomConnection.vote(id)}
            onUnvote={(id: string) => roomConnection.unvote(id)}
            onDelete={(id: string) => roomConnection.deleteSuggestion(id)}
            onPromote={async (id: string) => {
              await roomConnection.promoteSuggestion(id);
            }}
            onOpenSearch={() => {
              searchInputRef.current?.focus();
            }}
          />
        </>
      ) : null}
    </div>
  );
}
