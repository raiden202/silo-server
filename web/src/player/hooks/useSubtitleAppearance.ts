import { useMemo } from "react";
import { useCurrentProfile } from "@/hooks/useCurrentProfile";
import { useEffectiveSettings } from "@/hooks/queries/settings";
import { parseSubtitleAppearance, computeSubtitleStyles } from "@/lib/subtitleAppearance";
import type { SubtitleAppearance, SubtitleStyles } from "@/lib/subtitleAppearance";

export function useSubtitleAppearance(): SubtitleStyles & { settings: SubtitleAppearance } {
  const { profile } = useCurrentProfile();
  const { data } = useEffectiveSettings(profile?.id, ["subtitle_appearance"]);

  const settings = useMemo(
    () => parseSubtitleAppearance(data?.subtitle_appearance?.effective_value ?? null),
    [data],
  );

  const styles = useMemo(() => computeSubtitleStyles(settings), [settings]);

  return { settings, ...styles };
}
