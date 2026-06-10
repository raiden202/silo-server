/**
 * Shared formatters for group access-policy values in admin views.
 */

/** Formats a numeric limit where 0 means unlimited. */
export function formatLimit(value: number): string {
  return value === 0 ? "∞" : String(value);
}

/** Formats library access where null means unrestricted. */
export function formatLibraryAccess(ids: number[] | null): string {
  if (ids === null) return "All libraries";
  return `${ids.length} ${ids.length === 1 ? "library" : "libraries"}`;
}
