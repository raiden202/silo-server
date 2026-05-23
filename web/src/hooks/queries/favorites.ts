import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { api } from "@/api/client";
import type { BrowseItem } from "@/api/types";
import { favoriteKeys } from "./keys";
import { toast } from "sonner";
import { invalidateMediaSurfaceQueries, updateCatalogItemDetail } from "./mediaSurfaceRefresh";
import { bumpHomeRefreshSignal } from "@/pages/homeSurfaceRefresh";

export function useFavorites() {
  return useQuery({
    queryKey: favoriteKeys.list(),
    queryFn: () => api<{ items: BrowseItem[] }>("/favorites").then((d) => d.items ?? []),
  });
}

export function useToggleFavorite(itemId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (currentlyFavorite: boolean) =>
      api(`/favorites/${itemId}`, {
        method: currentlyFavorite ? "DELETE" : "PUT",
      }),
    onMutate: async (currentlyFavorite: boolean) => {
      await queryClient.cancelQueries({ queryKey: ["catalog", "items", itemId, "detail"] });
      const previous = queryClient.getQueriesData({
        predicate: (query) =>
          Array.isArray(query.queryKey) &&
          query.queryKey[0] === "catalog" &&
          query.queryKey[1] === "items" &&
          query.queryKey[2] === itemId &&
          query.queryKey[3] === "detail",
      });
      updateCatalogItemDetail(queryClient, itemId, (detail) => ({
        ...detail,
        user_state: {
          played: detail.user_state?.played ?? false,
          is_favorite: !currentlyFavorite,
          in_watchlist: detail.user_state?.in_watchlist ?? false,
        },
      }));
      return { previous };
    },
    onError: (_err, _vars, context) => {
      for (const [queryKey, value] of context?.previous ?? []) {
        queryClient.setQueryData(queryKey, value);
      }
      toast.error("Failed to update favorites");
    },
    onSuccess: (_data, currentlyFavorite) => {
      toast.success(currentlyFavorite ? "Removed from favorites" : "Added to favorites");
    },
    onSettled: async () => {
      await invalidateMediaSurfaceQueries(queryClient, { itemId });
      bumpHomeRefreshSignal(queryClient);
    },
  });
}
