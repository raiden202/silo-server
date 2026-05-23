import { useQuery } from "@tanstack/react-query";
import type { ScanRun } from "@/api/types";
import { adminKeys } from "../keys";

/**
 * Active scans are delivered exclusively via the WebSocket `scans` channel —
 * there is no REST endpoint. The queryFn returns an empty array as a fallback;
 * the real data arrives through RealtimeEventsProvider's `hydrateScans`.
 */
export function useActiveScans() {
  return useQuery({
    queryKey: adminKeys.activeScans(),
    queryFn: () => Promise.resolve([] as ScanRun[]),
    staleTime: Infinity,
  });
}
