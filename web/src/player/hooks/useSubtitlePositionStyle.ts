import { useEffect, useMemo, useState, type CSSProperties, type RefObject } from "react";
import { computeSubtitlePositionStyle } from "@/lib/subtitleAppearance";
import type { SubtitleAppearance } from "@/lib/subtitleAppearance";

/**
 * Tracks the player container size and the video's intrinsic aspect ratio,
 * then produces a position CSS style that anchors subtitles to a 16:9
 * reference frame centered on the actually-rendered video area (object-fit:
 * contain) rather than the player window. Falls back to container-relative
 * percentages until measurements are available.
 */
export function useSubtitlePositionStyle(
  containerRef: RefObject<HTMLElement | null>,
  videoRef: RefObject<HTMLVideoElement | null>,
  position: SubtitleAppearance["position"],
): CSSProperties {
  const [playerSize, setPlayerSize] = useState<{ w: number; h: number }>({ w: 0, h: 0 });
  const [videoAspect, setVideoAspect] = useState(0);

  useEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const update = () => setPlayerSize({ w: el.clientWidth, h: el.clientHeight });
    update();
    const ro = new ResizeObserver(update);
    ro.observe(el);
    return () => ro.disconnect();
  }, [containerRef]);

  useEffect(() => {
    const video = videoRef.current;
    if (!video) return;
    const update = () => {
      if (video.videoWidth > 0 && video.videoHeight > 0) {
        setVideoAspect(video.videoWidth / video.videoHeight);
      }
    };
    update();
    video.addEventListener("loadedmetadata", update);
    video.addEventListener("resize", update);
    return () => {
      video.removeEventListener("loadedmetadata", update);
      video.removeEventListener("resize", update);
    };
  }, [videoRef]);

  return useMemo(
    () => computeSubtitlePositionStyle(position, playerSize.w, playerSize.h, videoAspect),
    [position, playerSize.w, playerSize.h, videoAspect],
  );
}
