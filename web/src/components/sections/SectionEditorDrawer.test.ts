import { describe, expect, it } from "vitest";
import { buildAdminSectionPayload, buildProfileSectionSaveEntry } from "./SectionEditorDrawer";
import { queryDefinitionFromSectionConfig } from "@/api/types";

describe("SectionEditorDrawer payload builders", () => {
  it("preserves continue listening config for admin sections", () => {
    const payload = buildAdminSectionPayload({
      section: null,
      scope: "home",
      currentLibraryId: null,
      sectionType: "continue_watching",
      title: "Continue Listening",
      itemLimit: 20,
      featured: false,
      enabled: true,
      queryDefinition: queryDefinitionFromSectionConfig(),
      selectedCollectionId: "",
      recipeParams: { continue_type: "listening" },
    });

    expect(payload).toMatchObject({
      section_type: "continue_watching",
      title: "Continue Listening",
      config: { continue_type: "listening" },
    });
  });

  it("preserves continue listening config for profile sections", () => {
    const entry = buildProfileSectionSaveEntry({
      section: null,
      sectionType: "continue_watching",
      title: "Continue Listening",
      itemLimit: 20,
      featured: false,
      queryDefinition: queryDefinitionFromSectionConfig(),
      selectedCollectionId: "",
      recipeParams: { continue_type: "listening" },
    });

    expect(entry).toMatchObject({
      section_type: "continue_watching",
      title: "Continue Listening",
      is_custom: true,
      config: { continue_type: "listening" },
    });
  });

  it("adds row image style to admin section config", () => {
    const payload = buildAdminSectionPayload({
      section: null,
      scope: "home",
      currentLibraryId: null,
      sectionType: "recently_added",
      title: "Wide Recents",
      itemLimit: 20,
      featured: false,
      cardImageStyle: "landscape",
      enabled: true,
      queryDefinition: queryDefinitionFromSectionConfig(),
      selectedCollectionId: "",
      recipeParams: {},
    });

    expect(payload.config).toMatchObject({ card_image_style: "landscape" });
  });

  it("omits automatic row image style from profile section config", () => {
    const entry = buildProfileSectionSaveEntry({
      section: {
        id: "section-1",
        section_type: "recently_added",
        title: "Recently Added",
        featured: false,
        item_limit: 20,
        hidden: false,
        is_custom: true,
        customized: true,
        position: 0,
        config: { card_image_style: "landscape", min_rating: 7 },
      },
      sectionType: "recently_added",
      title: "Recently Added",
      itemLimit: 20,
      featured: false,
      cardImageStyle: "auto",
      queryDefinition: queryDefinitionFromSectionConfig(),
      selectedCollectionId: "",
      recipeParams: { min_rating: 7, card_image_style: "landscape" },
    });

    expect(entry.config).toEqual({ min_rating: 7 });
  });
});
