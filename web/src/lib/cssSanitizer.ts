/**
 * CSS sanitizer that strips external resource loading from raw CSS.
 *
 * Blocks: @import, external url(), external @font-face src
 * Allows: url(data:...), url(/...), url(#...), all other CSS
 */

/** Matches @import rules (with url() or bare string) */
const AT_IMPORT_RE = /@import\s+(?:url\(.*?\)|['"].*?['"])[^;]*;?/gi;

/** Check if a single url() value is safe (data:, local path, or fragment) */
function isSafeUrl(urlContent: string): boolean {
  const trimmed = urlContent.trim().replace(/^['"]|['"]$/g, "");
  if (!trimmed) return true;
  if (trimmed.startsWith("data:")) return true;
  if (trimmed.startsWith("/") && !trimmed.startsWith("//")) return true;
  if (trimmed.startsWith("#")) return true;
  // Relative paths (no scheme) are fine — they resolve to the same origin
  if (!/^[a-z][a-z0-9+.-]*:/i.test(trimmed) && !trimmed.startsWith("//")) return true;
  return false;
}

/**
 * Sanitize raw CSS by stripping external resource references.
 * Returns the cleaned CSS string.
 */
export function sanitizeCss(css: string): string {
  let result = css;

  // Strip @import rules entirely
  result = result.replace(AT_IMPORT_RE, "/* [blocked @import] */");

  // Replace external url() with empty url()
  result = result.replace(
    /url\(\s*(['"]?)([\s\S]*?)\1\s*\)/gi,
    (_match, _quote, content: string) => {
      if (isSafeUrl(content)) return _match;
      return "/* [blocked external url] */";
    },
  );

  return result;
}

/** Quick check if CSS contains patterns that would be sanitized. */
export function hasUnsafeCss(css: string): boolean {
  return css !== sanitizeCss(css);
}
