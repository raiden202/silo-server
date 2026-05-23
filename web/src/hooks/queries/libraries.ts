import { useQuery } from "@tanstack/react-query";
import { api } from "@/api/client";
import type { UserLibrary } from "@/api/types";
import { useAuth } from "@/hooks/useAuth";
import { libraryKeys } from "./keys";
import { useSettings } from "./settings";

export const DISABLED_LIBRARY_IDS_SETTING_KEY = "disabled_library_ids";
export const LIBRARY_ORDER_SETTING_KEY = "library_order";

function normalizeLibraryIDs(ids: number[]) {
  return [...new Set(ids.filter((id) => Number.isInteger(id) && id > 0))];
}

export function parseDisabledLibraryIDs(value: string | null | undefined) {
  if (!value) return [];

  try {
    const parsed = JSON.parse(value);
    if (!Array.isArray(parsed)) return [];
    return normalizeLibraryIDs(
      parsed.map((entry) => (typeof entry === "number" ? entry : Number.NaN)),
    );
  } catch {
    return [];
  }
}

export function serializeDisabledLibraryIDs(ids: number[]) {
  return JSON.stringify(normalizeLibraryIDs(ids));
}

export function parseLibraryOrder(value: string | null | undefined): number[] {
  if (!value) return [];

  try {
    const parsed = JSON.parse(value);
    if (!Array.isArray(parsed)) return [];
    return normalizeLibraryIDs(
      parsed.map((entry) => (typeof entry === "number" ? entry : Number.NaN)),
    );
  } catch {
    return [];
  }
}

export function serializeLibraryOrder(ids: number[]) {
  return JSON.stringify(normalizeLibraryIDs(ids));
}

export function applyLibraryOrder(libraries: UserLibrary[], orderIDs: number[]): UserLibrary[] {
  if (orderIDs.length === 0) return libraries;
  const pos = new Map(orderIDs.map((id, i) => [id, i]));
  const ordered: UserLibrary[] = [];
  const tail: UserLibrary[] = [];
  for (const lib of libraries) {
    if (pos.has(lib.id)) {
      ordered.push(lib);
    } else {
      tail.push(lib);
    }
  }
  ordered.sort((a, b) => (pos.get(a.id) ?? 0) - (pos.get(b.id) ?? 0));
  return [...ordered, ...tail];
}

export function filterVisibleLibraries(libraries: UserLibrary[], disabledLibraryIDs: number[]) {
  if (disabledLibraryIDs.length === 0) return libraries;
  const disabled = new Set(disabledLibraryIDs);
  return libraries.filter((library) => !disabled.has(library.id));
}

export function useAvailableUserLibraries() {
  const { profile } = useAuth();

  return useQuery({
    queryKey: libraryKeys.user(profile?.id),
    queryFn: () => api<UserLibrary[]>("/user/libraries"),
    staleTime: 5 * 60 * 1000,
  });
}

export function useUserLibraries() {
  const librariesQuery = useAvailableUserLibraries();
  const settingsQuery = useSettings();
  const disabledLibraryIDs = settingsQuery.isLoading
    ? null
    : parseDisabledLibraryIDs(settingsQuery.data?.[DISABLED_LIBRARY_IDS_SETTING_KEY]);
  const libraryOrder = settingsQuery.isLoading
    ? null
    : parseLibraryOrder(settingsQuery.data?.[LIBRARY_ORDER_SETTING_KEY]);

  let data = librariesQuery.data;
  if (data != null && disabledLibraryIDs != null) {
    data = filterVisibleLibraries(data, disabledLibraryIDs);
  }
  if (data != null && libraryOrder != null) {
    data = applyLibraryOrder(data, libraryOrder);
  }

  return {
    ...librariesQuery,
    data,
    isLoading: librariesQuery.isLoading || settingsQuery.isLoading,
    isFetching: librariesQuery.isFetching || settingsQuery.isFetching,
    error: librariesQuery.error ?? settingsQuery.error,
  };
}
