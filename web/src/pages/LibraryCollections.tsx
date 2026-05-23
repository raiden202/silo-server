import { useState } from "react";
import type { LibraryTabCollection, LibraryTabGroup, LibraryTabUngrouped } from "@/api/types";
import { useLibraryCollections } from "@/hooks/queries/libraryCollections";
import { useToggleSidebarPin } from "@/hooks/queries/sidebarPins";
import { useViewTransitionNavigate } from "@/hooks/useViewTransition";
import {
  buildLibraryCollectionCatalogHref,
  buildUserCollectionCatalogHref,
} from "@/pages/catalogSearchParams";
import { Card, CardContent } from "@/components/ui/card";
import { Skeleton } from "@/components/ui/skeleton";
import { Pin, PinOff, User } from "lucide-react";

interface LibraryCollectionsProps {
  libraryId: number;
}

const GRID_CLASSES =
  "grid grid-cols-3 sm:grid-cols-4 md:grid-cols-5 lg:grid-cols-7 xl:grid-cols-8 gap-3";

export default function LibraryCollections({ libraryId }: LibraryCollectionsProps) {
  const { data, isLoading } = useLibraryCollections(libraryId);

  if (isLoading) {
    return (
      <div className="page-shell py-6 sm:py-8">
        <div className={GRID_CLASSES}>
          {Array.from({ length: 24 }, (_, i) => (
            <div key={i}>
              <Skeleton className="aspect-[2/3] rounded-lg" />
              <Skeleton className="mt-2 h-4 w-3/4" />
            </div>
          ))}
        </div>
      </div>
    );
  }

  const groups = data?.groups ?? [];
  const ungroupedData = data?.ungrouped ?? null;
  const ungrouped = ungroupedData?.collections ?? [];

  if (groups.length === 0 && ungrouped.length === 0) {
    return (
      <div className="page-shell py-6 sm:py-8">
        <Card className="surface-panel overflow-hidden rounded-[2rem] border-0 shadow-none">
          <CardContent className="py-10 text-center">
            <p className="text-lg font-semibold">No collections yet</p>
            <p className="text-muted-foreground mt-2 text-sm">
              Create library collections from the admin area to feature curated shelves here.
            </p>
          </CardContent>
        </Card>
      </div>
    );
  }

  return (
    <div className="page-shell space-y-6 py-6 sm:py-8">
      <div className="page-header gap-5">
        <div className="space-y-3">
          <h1 className="page-title text-[clamp(2rem,4vw,3rem)]">Collections</h1>
          <p className="page-subtitle text-sm sm:text-base">
            Browse hand-picked shelves and smart lists created for this library.
          </p>
        </div>
      </div>
      <div className="space-y-8">
        {buildRenderOrder(groups, ungroupedData).map((item) =>
          item.kind === "ungrouped" ? (
            <UngroupedGroupSection
              key="ungrouped"
              collections={item.collections}
              libraryId={libraryId}
            />
          ) : (
            <GroupSection key={item.group.id} group={item.group} libraryId={libraryId} />
          ),
        )}
      </div>
    </div>
  );
}

// Build a unified render order from groups + (optional) ungrouped, sorted by
// each item's effective sort position.
type RenderItem =
  | { kind: "group"; group: LibraryTabGroup }
  | { kind: "ungrouped"; collections: LibraryTabCollection[] };

function buildRenderOrder(
  groups: LibraryTabGroup[],
  ungroupedData: LibraryTabUngrouped | null,
): RenderItem[] {
  type Slot = { order: number; item: RenderItem };
  const slots: Slot[] = groups.map((g) => ({
    order: g.sort_order,
    item: { kind: "group" as const, group: g },
  }));
  if (ungroupedData && ungroupedData.collections.length > 0) {
    slots.push({
      order: ungroupedData.sort_order,
      item: { kind: "ungrouped" as const, collections: ungroupedData.collections },
    });
  }
  slots.sort((a, b) => a.order - b.order);
  return slots.map((s) => s.item);
}

function UngroupedGroupSection({
  collections,
  libraryId,
}: {
  collections: LibraryTabCollection[];
  libraryId: number;
}) {
  return (
    <section>
      <div className={GRID_CLASSES}>
        {collections.map((c) => (
          <CollectionPosterCard key={c.id} collection={c} kind="regular" libraryId={libraryId} />
        ))}
      </div>
    </section>
  );
}

function GroupSection({ group, libraryId }: { group: LibraryTabGroup; libraryId: number }) {
  return (
    <section>
      <h2 className="mb-3 text-lg font-semibold">{group.name}</h2>
      <div className={GRID_CLASSES}>
        {group.collections.map((c) => (
          <CollectionPosterCard key={c.id} collection={c} kind={group.kind} libraryId={libraryId} />
        ))}
      </div>
    </section>
  );
}

function CollectionPosterCard({
  collection,
  kind,
  libraryId,
}: {
  collection: LibraryTabCollection;
  kind: "regular" | "user_collections";
  libraryId: number;
}) {
  const [loaded, setLoaded] = useState(false);
  const navigate = useViewTransitionNavigate();
  const { togglePin, isPinned } = useToggleSidebarPin();
  const pinned = isPinned(libraryId, "collection", collection.id);

  const isUserCollection = kind === "user_collections";
  const href = isUserCollection
    ? buildUserCollectionCatalogHref(collection.id, collection.title)
    : buildLibraryCollectionCatalogHref(collection.id, collection.title);

  return (
    <div className="group/card relative w-full text-left">
      <button type="button" onClick={() => navigate(href)} className="block w-full text-left">
        <div className="media-card-image relative aspect-[2/3] overflow-hidden rounded-xl">
          {collection.poster_url ? (
            <img
              src={collection.poster_url}
              alt={collection.title}
              className={`h-full w-full object-cover transition-opacity duration-300 ${
                loaded ? "opacity-100" : "opacity-0"
              }`}
              loading="lazy"
              onLoad={() => setLoaded(true)}
            />
          ) : (
            <div className="text-muted-foreground flex h-full w-full flex-col items-center justify-center gap-1 p-3 text-center text-sm">
              {isUserCollection && <User className="h-5 w-5 opacity-60" />}
              <span className="line-clamp-3 font-medium">{collection.title}</span>
            </div>
          )}
          <span className="bg-background/60 text-foreground absolute right-2 bottom-2 rounded-md px-2 py-0.5 text-[11px] font-bold backdrop-blur-sm">
            {collection.item_count}
          </span>
        </div>
        <div className="px-0.5 pt-2.5">
          <div className="truncate text-[13px] font-semibold">{collection.title}</div>
          {isUserCollection && <div className="text-muted-foreground text-xs">User collection</div>}
        </div>
      </button>
      {/* Pin to sidebar button — only for library collections */}
      {!isUserCollection && (
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            togglePin(libraryId, {
              type: "collection",
              id: collection.id,
              label: collection.title,
            });
          }}
          className={`absolute top-2 left-2 rounded-lg p-1.5 backdrop-blur-sm transition-all ${
            pinned
              ? "bg-primary/90 text-primary-foreground"
              : "bg-background/50 text-foreground opacity-0 group-hover/card:opacity-100"
          }`}
          title={pinned ? "Unpin from sidebar" : "Pin to sidebar"}
        >
          {pinned ? <PinOff className="h-3.5 w-3.5" /> : <Pin className="h-3.5 w-3.5" />}
        </button>
      )}
    </div>
  );
}
