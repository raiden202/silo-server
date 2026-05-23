import type { QueryClient } from "@tanstack/react-query";
import { catalogKeys, ratingKeys, recKeys, sectionKeys } from "./keys";

export async function invalidateRatingSurfaceQueries(queryClient: QueryClient, itemId: string) {
  await Promise.all([
    queryClient.invalidateQueries({ queryKey: ratingKeys.item(itemId) }),
    queryClient.invalidateQueries({ queryKey: ["catalog", "items", itemId, "detail"] }),
    queryClient.invalidateQueries({ queryKey: catalogKeys.all }),
    queryClient.invalidateQueries({ queryKey: recKeys.all }),
    queryClient.invalidateQueries({ queryKey: sectionKeys.all }),
  ]);
}
