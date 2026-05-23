import { useCallback, useMemo, useRef, useState } from "react";
import type { PlayerChapter } from "../types";

interface SeekBarProps {
  currentTime: number;
  duration: number;
  buffered: TimeRanges | null;
  chapters?: PlayerChapter[];
  introRegion?: { start: number; end: number } | null;
  creditsRegion?: { start: number; end: number } | null;
  onSeek: (seconds: number) => void;
}

function findChapterAtTime(chapters: PlayerChapter[], time: number): PlayerChapter | null {
  for (const chapter of chapters) {
    if (time >= chapter.start_seconds && time < chapter.end_seconds) {
      return chapter;
    }
  }
  return chapters.length > 0 ? (chapters[chapters.length - 1] ?? null) : null;
}

function formatTime(seconds: number): string {
  const s = Math.floor(seconds);
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const sec = s % 60;
  if (h > 0) {
    return `${h}:${m.toString().padStart(2, "0")}:${sec.toString().padStart(2, "0")}`;
  }
  return `${m}:${sec.toString().padStart(2, "0")}`;
}

export function SeekBar({
  currentTime,
  duration,
  buffered,
  chapters = [],
  introRegion,
  creditsRegion,
  onSeek,
}: SeekBarProps) {
  const barRef = useRef<HTMLDivElement>(null);
  const [hoverTime, setHoverTime] = useState<number | null>(null);
  const [dragging, setDragging] = useState(false);

  const getTimeFromEvent = useCallback(
    (e: React.MouseEvent | MouseEvent) => {
      const bar = barRef.current;
      if (!bar || duration <= 0) return 0;
      const rect = bar.getBoundingClientRect();
      const fraction = Math.max(0, Math.min(1, (e.clientX - rect.left) / rect.width));
      return fraction * duration;
    },
    [duration],
  );

  const getTimeFromTouch = useCallback(
    (e: React.TouchEvent<HTMLDivElement>) => {
      const bar = barRef.current;
      const touch = e.touches[0] ?? e.changedTouches[0];
      if (!bar || !touch || duration <= 0) return 0;
      const rect = bar.getBoundingClientRect();
      const fraction = Math.max(0, Math.min(1, (touch.clientX - rect.left) / rect.width));
      return fraction * duration;
    },
    [duration],
  );

  const [dragTime, setDragTime] = useState<number | null>(null);
  const hoverChapter = useMemo(
    () => (hoverTime === null ? null : findChapterAtTime(chapters, hoverTime)),
    [chapters, hoverTime],
  );

  const handleMouseDown = useCallback(
    (e: React.MouseEvent) => {
      setDragging(true);
      const time = getTimeFromEvent(e);
      setDragTime(time);

      const handleMouseMove = (ev: MouseEvent) => {
        setDragTime(getTimeFromEvent(ev));
      };
      const handleMouseUp = (ev: MouseEvent) => {
        const finalTime = getTimeFromEvent(ev);
        onSeek(finalTime);
        setDragging(false);
        setDragTime(null);
        document.removeEventListener("mousemove", handleMouseMove);
        document.removeEventListener("mouseup", handleMouseUp);
      };
      document.addEventListener("mousemove", handleMouseMove);
      document.addEventListener("mouseup", handleMouseUp);
    },
    [getTimeFromEvent, onSeek],
  );

  const handleMouseMove = useCallback(
    (e: React.MouseEvent) => {
      if (!dragging) {
        setHoverTime(getTimeFromEvent(e));
      }
    },
    [dragging, getTimeFromEvent],
  );

  const handleTouchStart = useCallback(
    (e: React.TouchEvent<HTMLDivElement>) => {
      setDragging(true);
      const time = getTimeFromTouch(e);
      setDragTime(time);
    },
    [getTimeFromTouch],
  );

  const handleTouchMove = useCallback(
    (e: React.TouchEvent<HTMLDivElement>) => {
      if (!dragging) return;
      e.preventDefault();
      const time = getTimeFromTouch(e);
      setDragTime(time);
    },
    [dragging, getTimeFromTouch],
  );

  const handleTouchEnd = useCallback(
    (e: React.TouchEvent<HTMLDivElement>) => {
      if (!dragging) return;
      setDragging(false);
      const time = getTimeFromTouch(e);
      onSeek(time);
      setDragTime(null);
      setHoverTime(null);
    },
    [dragging, getTimeFromTouch, onSeek],
  );

  const displayTime = dragTime ?? currentTime;
  const playedPercent = duration > 0 ? (displayTime / duration) * 100 : 0;

  const formatAriaValueText = useCallback((seconds: number): string => {
    const s = Math.floor(seconds);
    const m = Math.floor(s / 60);
    const sec = s % 60;
    if (m > 0 && sec > 0) return `${m} minutes ${sec} seconds`;
    if (m > 0) return `${m} minutes`;
    return `${sec} seconds`;
  }, []);

  const handleKeyDown = useCallback(
    (e: React.KeyboardEvent) => {
      let newTime: number | null = null;
      switch (e.key) {
        case "ArrowRight":
          newTime = Math.min(duration, displayTime + 5);
          break;
        case "ArrowLeft":
          newTime = Math.max(0, displayTime - 5);
          break;
        case "Home":
          newTime = 0;
          break;
        case "End":
          newTime = duration;
          break;
        default:
          return;
      }
      e.preventDefault();
      onSeek(newTime);
    },
    [duration, displayTime, onSeek],
  );

  // Calculate all buffered ranges as percentages.
  const bufferedRanges: Array<{ startPercent: number; widthPercent: number }> = [];
  if (buffered && duration > 0) {
    for (let i = 0; i < buffered.length; i++) {
      const startPercent = (buffered.start(i) / duration) * 100;
      const endPercent = (buffered.end(i) / duration) * 100;
      bufferedRanges.push({ startPercent, widthPercent: endPercent - startPercent });
    }
  }

  return (
    <div className="player-seekbar group/seek relative w-full px-2">
      {/* Hover time preview */}
      {hoverTime !== null && !dragging && (
        <div
          className="pointer-events-none absolute z-10 flex flex-col items-center"
          style={{
            left: `clamp(24px, ${(hoverTime / (duration || 1)) * 100}%, calc(100% - 24px))`,
            bottom: "calc(100% + 8px)",
            transform: "translateX(-50%)",
          }}
        >
          <div
            className={[
              "overflow-hidden rounded-lg border border-white/[0.08] bg-neutral-900/95 text-white shadow-2xl backdrop-blur-md",
              hoverChapter ? "w-40" : "",
            ].join(" ")}
          >
            {/* Thumbnail or chapter placeholder */}
            {hoverChapter &&
              (hoverChapter.thumbnail_url ? (
                <img
                  src={hoverChapter.thumbnail_url}
                  alt={hoverChapter.title}
                  className="aspect-video w-full object-cover"
                />
              ) : (
                <div className="flex aspect-video w-full items-center justify-center bg-gradient-to-b from-white/[0.06] to-white/[0.02]">
                  <svg
                    className="h-5 w-5 text-white/[0.12]"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="1.5"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  >
                    <rect x="2" y="2" width="20" height="20" rx="2.18" ry="2.18" />
                    <path d="m7 2 0 20M17 2v20M2 12h20M2 7h5M2 17h5M17 17h5M17 7h5" />
                  </svg>
                </div>
              ))}
            <div className="px-2.5 py-1.5">
              <div className="text-xs font-semibold text-white tabular-nums">
                {formatTime(hoverTime)}
              </div>
              {hoverChapter && (
                <div className="mt-0.5 truncate text-[11px] leading-tight text-white/50">
                  {hoverChapter.title}
                </div>
              )}
            </div>
          </div>
          {/* Caret */}
          <div className="relative -mt-px h-2 w-4 overflow-hidden">
            <div className="absolute top-0 left-1/2 h-2.5 w-2.5 -translate-x-1/2 rotate-45 border-r border-b border-white/[0.08] bg-neutral-900/95" />
          </div>
        </div>
      )}

      {/* Touch-friendly hit area (min 44px) with visual bar centered */}
      <div
        ref={barRef}
        role="slider"
        tabIndex={0}
        aria-label="Seek"
        aria-valuemin={0}
        aria-valuemax={duration}
        aria-valuenow={displayTime}
        aria-valuetext={formatAriaValueText(displayTime)}
        className="relative flex min-h-[44px] w-full cursor-pointer touch-none items-center rounded focus-visible:ring-2 focus-visible:ring-white/70 focus-visible:outline-none"
        onMouseDown={handleMouseDown}
        onMouseMove={handleMouseMove}
        onMouseLeave={() => setHoverTime(null)}
        onTouchStart={handleTouchStart}
        onTouchMove={handleTouchMove}
        onTouchEnd={handleTouchEnd}
        onKeyDown={handleKeyDown}
      >
        <div className="relative h-[3px] w-full rounded-full bg-white/15 transition-[height] duration-200 ease-out group-hover/seek:h-[5px]">
          {duration > 0 &&
            chapters
              .filter((chapter) => chapter.start_seconds > 0 && chapter.start_seconds < duration)
              .map((chapter) => (
                <div
                  key={chapter.index}
                  className="absolute top-1/2 z-[1] h-[10px] w-px -translate-x-1/2 -translate-y-1/2 bg-white/55"
                  style={{ left: `${(chapter.start_seconds / duration) * 100}%` }}
                />
              ))}
          {/* Buffered ranges */}
          {bufferedRanges.map((range, i) => (
            <div
              key={i}
              className="absolute inset-y-0 rounded-full bg-white/30"
              style={{ left: `${range.startPercent}%`, width: `${range.widthPercent}%` }}
            />
          ))}
          {/* Intro region */}
          {introRegion && duration > 0 && (
            <div
              className="absolute top-0 h-full rounded-full bg-sky-300/40"
              style={{
                left: `${(introRegion.start / duration) * 100}%`,
                width: `${((introRegion.end - introRegion.start) / duration) * 100}%`,
              }}
            />
          )}
          {/* Credits region */}
          {creditsRegion && duration > 0 && (
            <div
              className="absolute top-0 h-full rounded-full bg-amber-300/45"
              style={{
                left: `${(creditsRegion.start / duration) * 100}%`,
                width: `${((creditsRegion.end - creditsRegion.start) / duration) * 100}%`,
              }}
            />
          )}
          {/* Played — warm white with a subtle amber kiss at the leading edge */}
          <div
            className="absolute inset-y-0 left-0 rounded-full bg-white shadow-[0_0_10px_-1px_rgb(255_255_255/0.35)]"
            style={{ width: `${playedPercent}%` }}
          />
          {/* Thumb */}
          <div
            className="absolute top-1/2 h-3.5 w-3.5 -translate-x-1/2 -translate-y-1/2 rounded-full bg-white opacity-0 shadow-[0_4px_14px_rgb(0_0_0/0.45)] ring-1 ring-black/10 transition-all duration-200 group-hover/seek:opacity-100"
            style={{ left: `${playedPercent}%` }}
          />
        </div>
      </div>
    </div>
  );
}

export { formatTime };
