import { describe, expect, it } from "vitest";

import { mediaKindLabel, type CollectionTemplate } from "./collectionTemplates";

describe("collectionTemplates lib", () => {
  it("returns human labels for every media kind", () => {
    expect(mediaKindLabel("movie")).toBe("Movies");
    expect(mediaKindLabel("tv")).toBe("TV");
    expect(mediaKindLabel("mixed")).toBe("Movies + TV");
  });

  it("template type accepts all source variants", () => {
    const templates: CollectionTemplate[] = [
      {
        id: "tmdb-x",
        title: "TMDB",
        description: "",
        icon: "🔥",
        category: "trending",
        source: "tmdb",
        media_kind: "movie",
        tmdb: { preset: "trending", media_type: "movie", time_window: "day" },
      },
      {
        id: "trakt-x",
        title: "Trakt",
        description: "",
        icon: "📈",
        category: "trending",
        source: "trakt",
        media_kind: "tv",
        trakt: { preset: "trending", media_type: "tv" },
      },
      {
        id: "mdblist-x",
        title: "MDBList",
        description: "",
        icon: "📋",
        category: "custom",
        source: "mdblist",
        media_kind: "mixed",
        mdblist: { url: "https://mdblist.com/lists/x/json" },
      },
    ];
    expect(templates.map((t) => t.source)).toEqual(["tmdb", "trakt", "mdblist"]);
  });
});
