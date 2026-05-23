import { describe, expect, it } from "vitest";
import type { AdminSession } from "@/api/types";
import { formatVideoDetail, formatVideoSummary } from "./adminActivityPresentation";

function makeSession(overrides: Partial<AdminSession> = {}): AdminSession {
  return {
    session_id: overrides.session_id ?? "session-1",
    user_id: overrides.user_id ?? 1,
    username: overrides.username ?? "admin",
    profile_id: overrides.profile_id ?? "default",
    media_file_id: overrides.media_file_id ?? 2,
    requested_media_file_id: overrides.requested_media_file_id ?? 2,
    media_title: overrides.media_title ?? "Example",
    media_type: overrides.media_type ?? "movie",
    play_method: overrides.play_method ?? "transcode",
    reporting_node: overrides.reporting_node ?? "local",
    file_duration: overrides.file_duration ?? 3600,
    started_at: overrides.started_at ?? new Date().toISOString(),
    updated_at: overrides.updated_at ?? new Date().toISOString(),
    is_paused: overrides.is_paused ?? false,
    audio_track_index: overrides.audio_track_index ?? 0,
    transcode_audio: overrides.transcode_audio ?? true,
    stream_bitrate_kbps: overrides.stream_bitrate_kbps ?? 8000,
    target_bitrate_kbps: overrides.target_bitrate_kbps ?? 8000,
    source_bitrate_kbps: overrides.source_bitrate_kbps ?? 9000,
    source_video_codec: overrides.source_video_codec ?? "h264",
    source_video_resolution: overrides.source_video_resolution ?? "1080p",
    target_video_codec: overrides.target_video_codec ?? "h264",
    target_resolution: overrides.target_resolution ?? "1080p",
    source_audio_codec: overrides.source_audio_codec ?? "aac",
    source_audio_channels: overrides.source_audio_channels ?? 2,
    requested_video_codec: overrides.requested_video_codec ?? "hevc",
    requested_video_resolution: overrides.requested_video_resolution ?? "2160p",
  };
}

describe("adminActivityPresentation", () => {
  it("uses the effective source as the primary video summary", () => {
    expect(formatVideoSummary(makeSession())).toBe("H.264 · 1080p");
  });

  it("explains auto-switched requested sources in the detail line", () => {
    expect(
      formatVideoDetail(
        makeSession({
          media_file_id: 2,
          requested_media_file_id: 1,
        }),
      ),
    ).toBe("Auto-switched from HEVC · 2160p · Output → H.264 · 1080p");
  });
});
