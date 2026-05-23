import type { QueryClient } from "@tanstack/react-query";
import { sectionKeys } from "@/hooks/queries/keys";

export function bumpHomeRefreshSignal(queryClient: QueryClient) {
  queryClient.setQueryData<number>(
    sectionKeys.homeRefreshSignal(),
    (current) => (current ?? 0) + 1,
  );
}
