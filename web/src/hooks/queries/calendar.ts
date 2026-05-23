import { useQuery } from "@tanstack/react-query";
import { api } from "@/api/client";
import { calendarKeys } from "./keys";
import { addDays } from "@/lib/calendarWeek";

export interface CalendarEvent {
  content_id: string;
  type: "movie" | "episode" | "season_premiere";
  title: string;
  episode_title?: string;
  series_id?: string;
  season_number?: number;
  episode_number?: number;
  air_date: string;
  air_time?: string;
  poster_url?: string;
  poster_thumbhash?: string;
  badges: string[];
}

export interface CalendarDay {
  date: string;
  items: CalendarEvent[];
}

interface CalendarResponse {
  events: CalendarDay[];
}

export function useCalendarWeek(weekStart: string, params: { filter: string; libraryId?: number }) {
  return useQuery({
    queryKey: calendarKeys.week(weekStart, params.filter, params.libraryId),
    queryFn: () => {
      const sp = new URLSearchParams({
        start: weekStart,
        end: addDays(weekStart, 6),
        filter: params.filter,
      });
      if (params.libraryId) sp.set("library_id", String(params.libraryId));
      return api<CalendarResponse>(`/calendar?${sp}`).then((d) => d.events ?? []);
    },
    staleTime: 10 * 60 * 1000,
  });
}
