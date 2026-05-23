import { useEffect, useRef } from "react";
import type JASSUB from "jassub";
import type { PlayerSubtitleInfo } from "../types";
import { isASSCodec } from "../utils/assSubtitles";

/**
 * Manages client-side ASS/SSA subtitle rendering via JASSUB (libass WASM).
 *
 * When an ASS-codec subtitle track is active, this hook lazy-loads JASSUB,
 * creates an instance attached to the video element, and renders styled
 * subtitles onto a canvas overlay. When a non-ASS track is selected (or
 * subtitles are turned off), the JASSUB instance is destroyed.
 *
 * The existing VTT subtitle pipeline (useSubtitleTracks) handles SRT/VTT;
 * this hook handles ASS/SSA. The two are coordinated by the `isActive`
 * return value — when true, the VTT overlay should be suppressed.
 */
export function useASSSubtitles(
  videoRef: React.RefObject<HTMLVideoElement | null>,
  subtitleUrls: PlayerSubtitleInfo[],
  activeSubtitleIndex: number | null,
  isDetached: boolean,
  streamOriginSeconds: number,
  subtitleDelayMs: number,
): { isActive: boolean } {
  const jassubRef = useRef<JASSUB | null>(null);
  const jassubImportRef = useRef<Promise<typeof JASSUB> | null>(null);
  // Effective JASSUB time offset. `streamOriginSeconds` accounts for HLS
  // PTS rebasing; the user-facing delay (ms → s) adds on top. Positive
  // delay = subtitles shown later, matching VTTCue semantics.
  const effectiveOffset = streamOriginSeconds + subtitleDelayMs / 1000;
  const streamOriginRef = useRef(effectiveOffset);
  streamOriginRef.current = effectiveOffset;

  // Resolve the active subtitle track.
  const activeSub =
    activeSubtitleIndex !== null
      ? (subtitleUrls.find((s) => s.index === activeSubtitleIndex) ?? null)
      : null;

  const isASS = activeSub !== null && isASSCodec(activeSub.codec);
  const activeUrl = isASS ? activeSub.url : null;

  // Main effect: create/destroy JASSUB based on active track.
  useEffect(() => {
    const video = videoRef.current;

    // Destroy JASSUB if the active track is not ASS, or player is detached,
    // or no video element is available.
    if (!activeUrl || !video || isDetached) {
      if (jassubRef.current) {
        jassubRef.current.destroy();
        jassubRef.current = null;
      }
      return;
    }

    let cancelled = false;

    async function initJASSUB() {
      if (!video || cancelled) return;

      // Lazy-load JASSUB module (only once).
      if (!jassubImportRef.current) {
        jassubImportRef.current = import("jassub").then((m) => m.default);
      }

      const JASSUBClass = await jassubImportRef.current;
      if (cancelled) return;

      // If an existing instance is already attached, reuse it by switching
      // the track URL. Snapshot to a local variable to avoid null deref if
      // the ref is cleared between awaits.
      const existing = jassubRef.current;
      if (existing) {
        existing.timeOffset = streamOriginRef.current;
        await existing.ready;
        if (!cancelled) {
          await existing.renderer.setTrackByUrl(activeUrl!);
        }
        return;
      }

      if (cancelled) return;

      const instance = new JASSUBClass({
        video,
        subUrl: activeUrl!,
        timeOffset: streamOriginRef.current,
      });

      // Guard against the effect being cleaned up while the constructor ran.
      if (cancelled) {
        instance.destroy();
        return;
      }

      jassubRef.current = instance;
    }

    initJASSUB();

    return () => {
      cancelled = true;
      // Destroy the current instance if the effect is being torn down
      // (e.g. track switch or unmount). This covers the common case where
      // initJASSUB has already completed and stored the instance.
      if (jassubRef.current) {
        jassubRef.current.destroy();
        jassubRef.current = null;
      }
    };
    // videoRef is a stable ref object. streamOriginSeconds is read from
    // streamOriginRef inside the async function to always get the latest value.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeUrl, isDetached]);

  // Update JASSUB's time offset when either the media timeline remaps or
  // the user nudges subtitle sync. Avoids destroying and recreating the
  // instance for offset-only changes.
  useEffect(() => {
    const instance = jassubRef.current;
    if (!instance || !activeUrl) return;

    instance.timeOffset = effectiveOffset;
    void instance.resize(true);
  }, [effectiveOffset, activeUrl]);

  // Cleanup on unmount.
  useEffect(() => {
    return () => {
      if (jassubRef.current) {
        jassubRef.current.destroy();
        jassubRef.current = null;
      }
    };
  }, []);

  return { isActive: isASS && !isDetached };
}
