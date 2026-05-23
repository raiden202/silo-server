/** Codecs that indicate ASS/SSA format subtitles with rich styling support. */
const ASS_CODECS = new Set(["ass", "ssa"]);

/** Returns true if the given subtitle codec is ASS/SSA format. */
export function isASSCodec(codec: string | undefined): boolean {
  if (!codec) return false;
  return ASS_CODECS.has(codec.toLowerCase());
}

/**
 * Returns a human-readable format label for display in the subtitle menu,
 * or null if the codec is unknown/unset.
 */
export function getSubtitleFormatLabel(codec: string | undefined): string | null {
  if (!codec) return null;
  switch (codec.toLowerCase()) {
    case "ass":
    case "ssa":
      return "ASS";
    case "srt":
    case "subrip":
      return "SRT";
    case "vtt":
    case "webvtt":
      return "VTT";
    case "pgs":
    case "hdmv_pgs_subtitle":
      return "PGS";
    case "dvd_subtitle":
      return "DVD";
    case "dvb_subtitle":
      return "DVB";
    default:
      return null;
  }
}
