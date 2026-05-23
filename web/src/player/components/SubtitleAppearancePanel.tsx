import { useCallback, useMemo } from "react";
import {
  useDeleteDeviceSetting,
  useEffectiveSettings,
  useSetDeviceSetting,
} from "@/hooks/queries/settings";
import { useCurrentProfile } from "@/hooks/useCurrentProfile";
import { parseSubtitleAppearance, type SubtitleAppearance } from "@/lib/subtitleAppearance";
import { SubtitleAppearancePanelView } from "@/components/settings/SubtitleAppearancePanelView";

const SETTINGS_KEY = "subtitle_appearance";

interface SubtitleAppearancePanelProps {
  open: boolean;
  onClose: () => void;
}

/**
 * In-player subtitle styling panel modelled on Plex's "Playback Settings"
 * sheet — lets a viewer tune font size, colour, position, background, and
 * outline without leaving the player. Every change writes to the current
 * profile + device subtitle appearance override so the rendered subtitles
 * update live via the shared effective-settings cache.
 *
 * The visual chrome lives in {@link SubtitleAppearancePanelView}; this
 * wrapper is responsible solely for binding the panel to the current
 * profile/device and the device-setting mutations.
 */
export function SubtitleAppearancePanel({ open, onClose }: SubtitleAppearancePanelProps) {
  const { profile } = useCurrentProfile();
  const { data } = useEffectiveSettings(profile?.id, [SETTINGS_KEY]);
  const { mutate: save } = useSetDeviceSetting();
  const { mutate: reset } = useDeleteDeviceSetting();

  const settings = useMemo(
    () => parseSubtitleAppearance(data?.[SETTINGS_KEY]?.effective_value ?? null),
    [data],
  );

  const update = useCallback(
    (patch: Partial<SubtitleAppearance>) => {
      const next = { ...settings, ...patch };
      save({ key: SETTINGS_KEY, value: JSON.stringify(next) });
    },
    [settings, save],
  );

  const resetDefaults = useCallback(() => {
    reset({ key: SETTINGS_KEY });
  }, [reset]);

  return (
    <SubtitleAppearancePanelView
      open={open}
      value={settings}
      onChange={update}
      onClose={onClose}
      onReset={resetDefaults}
    />
  );
}
