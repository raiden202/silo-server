// Catalog filter values for `original_language` are ISO 639-1 codes ("en",
// "fr", "ja"). Surfaces show friendly names ("English", "French", "Japanese")
// while keeping the underlying value untouched so query rules stay valid.

let cachedDisplayNames: Intl.DisplayNames | null | undefined;

function getDisplayNames(): Intl.DisplayNames | null {
  if (cachedDisplayNames !== undefined) {
    return cachedDisplayNames;
  }
  try {
    cachedDisplayNames = new Intl.DisplayNames(["en"], { type: "language" });
  } catch {
    cachedDisplayNames = null;
  }
  return cachedDisplayNames;
}

export function formatLanguage(code: string): string {
  const trimmed = code.trim();
  if (!trimmed) return "";
  const dn = getDisplayNames();
  if (!dn) return trimmed.toUpperCase();
  try {
    const name = dn.of(trimmed);
    if (name && name !== trimmed) {
      // Intl.DisplayNames returns lowercase for some locales — capitalize.
      return name.charAt(0).toUpperCase() + name.slice(1);
    }
  } catch {
    // Fall through to fallback.
  }
  return trimmed.toUpperCase();
}
