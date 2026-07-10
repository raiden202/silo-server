import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";
import QualityBadges from "./QualityBadges";

describe("QualityBadges", () => {
  it("renders the selected media summary without re-ranking versions", () => {
    const markup = renderToStaticMarkup(
      <QualityBadges
        summary={{
          durationMinutes: 60,
          resolution: "4K",
          videoRangeLabel: "DV HDR10",
          audioLabel: "DD+ Atmos",
        }}
      />,
    );

    expect(markup).toContain(">4K<");
    expect(markup).toContain(">DV HDR10<");
    expect(markup).toContain(">DD+ Atmos<");
  });

  it("returns no markup when the summary is empty", () => {
    const markup = renderToStaticMarkup(
      <QualityBadges
        summary={{ durationMinutes: 0, resolution: "", videoRangeLabel: "", audioLabel: "" }}
      />,
    );

    expect(markup).toBe("");
  });
});
