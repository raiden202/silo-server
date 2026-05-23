import { useQuery } from "@tanstack/react-query";
import { api } from "@/api/client";
import type {
  BrowseItem,
  LibraryTabCollection,
  LibraryTabResponse,
  ServerVisibleUserCollection,
} from "@/api/types";
import { libraryCollectionKeys } from "./keys";

export function useLibraryCollections(libraryId: number) {
  return useQuery({
    queryKey: libraryCollectionKeys.list(libraryId),
    queryFn: () => api<LibraryTabResponse>(`/library/${libraryId}/collections`),
    enabled: Number.isFinite(libraryId) && libraryId > 0,
  });
}

export function flattenLibraryCollections(
  resp: LibraryTabResponse | undefined,
): LibraryTabCollection[] {
  if (!resp) return [];
  const out: LibraryTabCollection[] = [];
  for (const group of resp.groups ?? []) {
    out.push(...(group.collections ?? []));
  }
  out.push(...(resp.ungrouped?.collections ?? []));
  return out;
}

export function useLibraryUserCollections(libraryId: number) {
  return useQuery({
    queryKey: libraryCollectionKeys.userContributed(libraryId),
    queryFn: () =>
      api<{ collections: ServerVisibleUserCollection[] }>(
        `/library/${libraryId}/user-collections`,
      ).then((data) => data.collections ?? []),
    enabled: Number.isFinite(libraryId) && libraryId > 0,
  });
}

export function useLibraryCollectionItems(libraryId: number, collectionId: string | null) {
  return useQuery({
    queryKey: libraryCollectionKeys.items(libraryId, collectionId ?? ""),
    queryFn: () =>
      api<{ items: BrowseItem[] }>(`/library/${libraryId}/collections/${collectionId}/items`).then(
        (data) => data.items ?? [],
      ),
    enabled:
      Number.isFinite(libraryId) &&
      libraryId > 0 &&
      collectionId !== null &&
      collectionId.length > 0,
  });
}
