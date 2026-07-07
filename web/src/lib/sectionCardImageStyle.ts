import type { SectionCardImageStyle } from "@/api/types";

export type SectionCardImageStyleSetting = SectionCardImageStyle | "auto";

const CONFIG_KEY = "card_image_style";

export function getSectionCardImageStyleSetting(
  config?: Record<string, unknown>,
): SectionCardImageStyleSetting {
  const value = config?.[CONFIG_KEY];
  if (value === "portrait" || value === "landscape") {
    return value;
  }
  return "auto";
}

export function applySectionCardImageStyle(
  config: Record<string, unknown> | undefined,
  style: SectionCardImageStyleSetting | undefined,
): Record<string, unknown> {
  const next = { ...(config ?? {}) };
  if (style === undefined) {
    return next;
  }

  delete next[CONFIG_KEY];

  if (style === "portrait" || style === "landscape") {
    next[CONFIG_KEY] = style;
  }

  return next;
}

export function sectionCardImageStyleLabel(style: SectionCardImageStyleSetting): string | null {
  if (style === "portrait") return "Portrait";
  if (style === "landscape") return "Landscape";
  return null;
}
