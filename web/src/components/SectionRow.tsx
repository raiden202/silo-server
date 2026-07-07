import ContinueWatchingCard from "@/components/ContinueWatchingCard";
import MediaCarousel from "@/components/MediaCarousel";
import SectionItemCard from "@/components/SectionItemCard";
import { useViewTransitionNavigate } from "@/hooks/useViewTransition";
import { useToggleSidebarPin } from "@/hooks/queries/sidebarPins";
import { useOverlayPrefs } from "@/hooks/useOverlayPrefs";
import { buildSectionCatalogHref, isSectionBrowseSupported } from "@/pages/catalogSearchParams";
import type { ResolvedSection } from "@/api/types";
import type { SectionCardImageStyle } from "@/api/types";
import { Pin, PinOff } from "lucide-react";

interface SectionRowProps {
  section: ResolvedSection;
  /** Library ID this section belongs to (enables pin-to-sidebar) */
  libraryId?: number;
  cardImageStyle?: SectionCardImageStyle;
}

function SectionPinButton({
  sectionId,
  sectionTitle,
  libraryId,
}: {
  sectionId: string;
  sectionTitle: string;
  libraryId: number;
}) {
  const { togglePin, isPinned } = useToggleSidebarPin();
  const pinned = isPinned(libraryId, "section", sectionId);

  return (
    <button
      onClick={() => togglePin(libraryId, { type: "section", id: sectionId, label: sectionTitle })}
      className={`rounded-full p-1.5 transition-colors ${
        pinned
          ? "text-primary hover:text-primary/80"
          : "text-muted-foreground/40 hover:bg-accent/60 hover:text-muted-foreground opacity-0 group-hover/carousel:opacity-100"
      }`}
      title={pinned ? "Unpin from sidebar" : "Pin to sidebar"}
    >
      {pinned ? <PinOff className="h-4 w-4" /> : <Pin className="h-4 w-4" />}
    </button>
  );
}

export default function SectionRow({ section, libraryId, cardImageStyle }: SectionRowProps) {
  const navigate = useViewTransitionNavigate();
  const browseSupported = isSectionBrowseSupported(section.section_type);
  const { prefs: overlayPrefs } = useOverlayPrefs();
  const configuredImageStyle = cardImageStyle ?? section.card_image_style;

  const handleViewAll = () => {
    if (!browseSupported) {
      return;
    }

    navigate(
      libraryId
        ? buildSectionCatalogHref({
            scope: "library",
            libraryId: libraryId,
            sectionId: section.id,
            title: section.title,
          })
        : buildSectionCatalogHref({
            scope: "home",
            sectionId: section.id,
            title: section.title,
          }),
    );
  };

  if (section.items.length === 0) return null;

  const headerActions =
    libraryId && browseSupported ? (
      <SectionPinButton sectionId={section.id} sectionTitle={section.title} libraryId={libraryId} />
    ) : undefined;

  return (
    <MediaCarousel
      title={section.title}
      onViewAll={
        browseSupported && section.total_count > section.item_limit ? handleViewAll : undefined
      }
      headerActions={headerActions}
    >
      {section.section_type === "continue_watching" || section.section_type === "next_up"
        ? (() => {
            const configuredVariant =
              configuredImageStyle === "portrait"
                ? "poster"
                : configuredImageStyle === "landscape"
                  ? "wide"
                  : undefined;
            // Continue Watching with only movies, audiobooks, and/or ebooks
            // looks better as upright covers (audiobook covers render square
            // within the poster variant; ebook covers are naturally 2:3).
            // Episodes (incl. Next Up) keep the wide variant so stills are
            // shown with their natural 16:9 framing.
            const cardVariant: "wide" | "poster" =
              configuredVariant ??
              (section.section_type === "continue_watching" &&
              section.items.length > 0 &&
              section.items.every(
                (it) => it.type === "movie" || it.type === "audiobook" || it.type === "ebook",
              )
                ? "poster"
                : "wide");
            return section.items.map((item) => (
              <ContinueWatchingCard
                key={item.content_id}
                sectionItem={item}
                libraryId={libraryId}
                overlayPrefs={overlayPrefs}
                variant={cardVariant}
              />
            ));
          })()
        : (() => {
            const imageStyle = configuredImageStyle ?? "portrait";
            const cardWidth =
              imageStyle === "landscape"
                ? "w-[260px] shrink-0 sm:w-[315px]"
                : "w-[140px] shrink-0 sm:w-[160px] lg:w-[185px]";
            return section.items.map((item) => (
              <div key={item.content_id} className={cardWidth} role="listitem">
                <SectionItemCard item={item} libraryId={libraryId} imageStyle={imageStyle} />
              </div>
            ));
          })()}
    </MediaCarousel>
  );
}
