import { useState } from "react";
import { useSearchParams } from "react-router";
import { CalendarDays } from "lucide-react";
import { Button } from "@/components/ui/button";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import MediaCarousel from "@/components/MediaCarousel";
import { useDocumentTitle } from "@/hooks/useDocumentTitle";
import { useCalendarWeek } from "@/hooks/queries/calendar";
import { useUserLibraries } from "@/hooks/queries/libraries";
import WeekNavigator from "@/components/calendar/WeekNavigator";
import DayGroup from "@/components/calendar/DayGroup";
import { addWeeks, formatDayHeading, getWeekDays, getWeekStart } from "@/lib/calendarWeek";

type CalendarFilter = "all" | "favorites" | "watchlist";

const FILTER_OPTIONS: { value: CalendarFilter; label: string }[] = [
  { value: "all", label: "All" },
  { value: "favorites", label: "Favorites" },
  { value: "watchlist", label: "Watchlist" },
];

/** Skeleton rows mirror a full week; enough slides per row to fill wide viewports. */
const CALENDAR_SKELETON_DAY_ROWS = 7;
const CALENDAR_SKELETON_ITEMS_PER_ROW = 18;

function parseCalendarParams(searchParams: URLSearchParams) {
  const weekRaw = searchParams.get("week");
  const weekStart =
    weekRaw && /^\d{4}-\d{2}-\d{2}$/.test(weekRaw) ? weekRaw : getWeekStart(new Date());
  const rawFilter = searchParams.get("filter");
  const filter: CalendarFilter =
    rawFilter === "favorites" || rawFilter === "watchlist" ? rawFilter : "all";
  const libraryIdRaw = searchParams.get("library");
  const libraryId = libraryIdRaw ? Number(libraryIdRaw) : undefined;
  return { weekStart, filter, libraryId };
}

export default function Calendar() {
  useDocumentTitle("Calendar");
  const [searchParams, setSearchParams] = useSearchParams();
  const { weekStart, filter, libraryId } = parseCalendarParams(searchParams);
  const libraries = useUserLibraries();

  const queryParams = { filter, libraryId };
  const prevWeek = addWeeks(weekStart, -1);
  const nextWeek = addWeeks(weekStart, 1);

  const currentQuery = useCalendarWeek(weekStart, queryParams);

  // Selected day in the week navigator. Drives the highlight in the day strip
  // and is used to scroll to the matching DayGroup when the user taps a day.
  // Derived `activeSelectedDay` ignores stale selections from a different week
  // so we don't need a reset effect when the user navigates weeks.
  const [selectedDay, setSelectedDay] = useState<string | null>(null);
  const visibleWeekDays = getWeekDays(weekStart);
  const activeSelectedDay =
    selectedDay && visibleWeekDays.includes(selectedDay) ? selectedDay : null;

  function setParams(updates: Record<string, string | undefined>) {
    setSearchParams(
      (prev) => {
        const next = new URLSearchParams(prev);
        for (const [k, v] of Object.entries(updates)) {
          if (v == null || v === "") next.delete(k);
          else next.set(k, v);
        }
        return next;
      },
      { replace: true },
    );
  }

  const goToWeek = (w: string) => setParams({ week: w });
  const setFilter = (f: string) => setParams({ filter: f === "all" ? undefined : f });
  const setLibrary = (id: string) => setParams({ library: id === "all" ? undefined : id });

  const onSelectDay = (date: string) => {
    setSelectedDay(date);
    // Defer to next frame so the highlighted state renders before we scroll.
    requestAnimationFrame(() => {
      const el = document.getElementById(`day-${date}`);
      if (el) el.scrollIntoView({ behavior: "smooth", block: "start" });
    });
  };

  const days = currentQuery.data ?? [];
  const datesWithEvents = new Set(days.map((d) => d.date));
  const showSelectedEmpty =
    !!activeSelectedDay && !datesWithEvents.has(activeSelectedDay) && !currentQuery.isLoading;

  return (
    <div className="space-y-2 py-4 pb-10 lg:py-8">
      {/* Same horizontal rhythm as Recommendations: one page gutter; MediaCarousel row padding aligns with it */}
      <div className="px-4 pt-4 pb-4 sm:px-6 sm:pt-6 lg:px-10 xl:px-12">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h1 className="text-2xl font-bold tracking-tight sm:text-3xl">Calendar</h1>
          <div className="flex flex-wrap items-center gap-2">
            {/* Filter toggle */}
            <div
              role="group"
              aria-label="Filter"
              className="surface-panel-subtle flex items-center gap-0.5 rounded-full p-1"
            >
              {FILTER_OPTIONS.map((opt) => (
                <button
                  key={opt.value}
                  type="button"
                  aria-pressed={filter === opt.value}
                  onClick={() => setFilter(opt.value)}
                  className={`rounded-full px-3 py-1 text-[12px] font-semibold transition-all duration-150 sm:px-4 sm:py-1.5 sm:text-[13px] ${
                    filter === opt.value
                      ? "bg-primary text-primary-foreground shadow-sm"
                      : "text-muted-foreground hover:bg-surface-hover hover:text-foreground"
                  }`}
                >
                  {opt.label}
                </button>
              ))}
            </div>

            {/* Library filter */}
            {libraries.data && libraries.data.length > 1 && (
              <Select value={libraryId ? String(libraryId) : "all"} onValueChange={setLibrary}>
                <SelectTrigger className="border-border/50 bg-surface/60 h-9 w-auto min-w-[120px] rounded-full text-[12px] font-semibold backdrop-blur-sm sm:min-w-[140px] sm:text-[13px]">
                  <SelectValue placeholder="All Libraries" />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="all">All Libraries</SelectItem>
                  {libraries.data.map((lib) => (
                    <SelectItem key={lib.id} value={String(lib.id)}>
                      {lib.name}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            )}
          </div>
        </div>
      </div>

      {/* Sticky week navigator — anchors to the page-level container so the
          containing block spans every DayGroup, keeping it pinned through
          full-page scroll. top-20 clears the mobile header; when the mobile
          header auto-hides on scroll-down (Layout sets data-mobile-header-hidden
          on <html>) we pin closer to the viewport top so the gap stays tight. */}
      <div className="sticky top-20 z-20 px-4 pb-3 transition-[top] duration-200 ease-out sm:px-6 lg:top-2 lg:px-10 xl:px-12 [html[data-mobile-header-hidden=true]_&]:top-2">
        <WeekNavigator
          weekStart={weekStart}
          days={days}
          selectedDay={activeSelectedDay}
          onSelectDay={onSelectDay}
          onPrev={() => goToWeek(prevWeek)}
          onNext={() => goToWeek(nextWeek)}
          onToday={() => goToWeek(getWeekStart(new Date()))}
        />
      </div>

      {showSelectedEmpty && (
        <div className="px-4 sm:px-6 lg:px-10 xl:px-12">
          <div
            className="surface-panel-subtle text-muted-foreground rounded-2xl px-4 py-3 text-[13px]"
            role="status"
          >
            Nothing scheduled for{" "}
            <span className="text-foreground font-semibold">
              {formatDayHeading(activeSelectedDay!)}
            </span>
            .
          </div>
        </div>
      )}

      {currentQuery.isLoading ? (
        <CalendarSkeleton />
      ) : days.length > 0 ? (
        <div className="space-y-6">
          {days.map((day) => (
            <DayGroup key={day.date} day={day} isSelected={activeSelectedDay === day.date} />
          ))}
        </div>
      ) : (
        <div className="px-4 sm:px-6 lg:px-10 xl:px-12">
          <CalendarEmpty filter={filter} onClearFilter={() => setFilter("all")} />
        </div>
      )}
    </div>
  );
}

function CalendarSkeleton() {
  return (
    <div className="space-y-6">
      {Array.from({ length: CALENDAR_SKELETON_DAY_ROWS }).map((_, i) => (
        <MediaCarousel
          key={i}
          title="Loading..."
          loading
          skeletonCount={CALENDAR_SKELETON_ITEMS_PER_ROW}
        >
          {null}
        </MediaCarousel>
      ))}
    </div>
  );
}

function CalendarEmpty({ filter, onClearFilter }: { filter: string; onClearFilter: () => void }) {
  return (
    <div className="surface-panel flex min-h-[300px] flex-col items-center justify-center gap-3 rounded-[1.8rem] border-0 px-6 py-16 text-center">
      <CalendarDays className="text-muted-foreground h-10 w-10" strokeWidth={1.5} />
      <p className="text-muted-foreground text-sm">
        {filter !== "all"
          ? `No events this week matching your ${filter} filter.`
          : "Nothing scheduled this week."}
      </p>
      {filter !== "all" && (
        <Button variant="link" size="sm" className="text-primary text-sm" onClick={onClearFilter}>
          Show all
        </Button>
      )}
    </div>
  );
}
