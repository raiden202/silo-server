import type { ReactNode } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

import SectionItemCard from "./SectionItemCard";

vi.mock("@/components/ViewTransitionLink", () => ({
  default: ({ children }: { children: ReactNode }) => <div>{children}</div>,
}));

vi.mock("@/components/MediaItemMenu", () => ({
  default: () => null,
}));

vi.mock("@/components/overlays/CardOverlays", () => ({
  default: () => null,
}));

vi.mock("@/hooks/useOverlayPrefs", () => ({
  useOverlayPrefs: () => ({ prefs: null }),
}));

describe("SectionItemCard", () => {
  it("renders premiere metadata when an upcoming event is present", () => {
    const markup = renderToStaticMarkup(
      <SectionItemCard
        item={{
          content_id: "series-1",
          type: "series",
          title: "Series A",
          year: 2024,
          genres: ["Drama"],
          status: "matched",
          rating_imdb: 8.2,
          overview: "Overview",
          poster_url: "",
          poster_thumbhash: "",
          backdrop_url: "",
          backdrop_thumbhash: "",
          logo_url: "",
          upcoming_event: {
            type: "season_premiere",
            air_date: "2026-04-08",
            air_time: "20:00",
            episode_title: "Back Again",
            season_number: 2,
            badges: ["season_premiere"],
          },
        }}
      />,
    );

    expect(markup).toContain("Series A");
    expect(markup).toContain("Season Premiere");
    expect(markup).toContain("Season 2 · Back Again");
    expect(markup).toContain("Wed, Apr 8");
    expect(markup).toContain("8:00 PM");
  });
});
