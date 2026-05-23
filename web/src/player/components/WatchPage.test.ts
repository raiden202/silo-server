import { describe, expect, it } from "vitest";

import { derivePersistedSubtitleMode } from "../utils/subtitleMode";

describe("derivePersistedSubtitleMode", () => {
  it("persists an enabled mode when a subtitle track is selected", () => {
    expect(derivePersistedSubtitleMode(3)).toBe("always");
  });

  it("persists off when subtitles are disabled", () => {
    expect(derivePersistedSubtitleMode(null)).toBe("off");
  });
});
