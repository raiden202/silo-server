import { useMemo } from "react";
import { GripVertical, Trash2 } from "lucide-react";

import { DndContext } from "@dnd-kit/core";
import {
  SortableContext,
  verticalListSortingStrategy,
  useSortable,
} from "@dnd-kit/sortable";
import { CSS } from "@dnd-kit/utilities";

import type { CollectionItem } from "@/api/types";
import { Button } from "@/components/ui/button";
import { Skeleton } from "@/components/ui/skeleton";
import {
  useCollectionItems,
  useRemoveCollectionItem,
  useReorderCollectionItems,
} from "@/hooks/queries/collections";
import { useSortableList } from "@/hooks/useSortableList";

interface ManualCollectionItemsEditorProps {
  collectionId: string;
  readOnly?: boolean;
}

export function ManualCollectionItemsEditor({
  collectionId,
  readOnly = false,
}: ManualCollectionItemsEditorProps) {
  const { data, isLoading } = useCollectionItems(collectionId);
  const items = useMemo(() => data ?? [], [data]);
  const reorderMutation = useReorderCollectionItems(collectionId);
  const removeMutation = useRemoveCollectionItem(collectionId);

  const { sensors, collisionDetection, handleDragEnd } = useSortableList(
    items,
    (item) => item.media_item_id,
    (orderedIds) => reorderMutation.mutate(orderedIds),
  );

  if (isLoading) {
    return (
      <div className="space-y-2">
        {Array.from({ length: 3 }).map((_, i) => (
          <Skeleton key={i} className="h-12 rounded-lg" />
        ))}
      </div>
    );
  }

  if (items.length === 0) {
    return (
      <div className="text-muted-foreground rounded-lg border border-dashed px-4 py-5 text-sm">
        No items yet. Items added to this collection will appear here.
      </div>
    );
  }

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={collisionDetection}
      onDragEnd={readOnly ? undefined : handleDragEnd}
    >
      <SortableContext
        items={items.map((item) => item.media_item_id)}
        strategy={verticalListSortingStrategy}
      >
        <div className="space-y-2">
          {items.map((item, index) => (
            <SortableItemRow
              key={item.media_item_id}
              item={item}
              index={index}
              readOnly={readOnly}
              onRemove={() => removeMutation.mutate(item.media_item_id)}
            />
          ))}
        </div>
      </SortableContext>
    </DndContext>
  );
}

function SortableItemRow({
  item,
  index,
  readOnly,
  onRemove,
}: {
  item: CollectionItem;
  index: number;
  readOnly: boolean;
  onRemove: () => void;
}) {
  const { attributes, listeners, setNodeRef, transform, transition, isDragging } = useSortable({
    id: item.media_item_id,
    disabled: readOnly,
  });
  const style: React.CSSProperties = {
    transform: CSS.Transform.toString(transform),
    transition,
    opacity: isDragging ? 0.4 : 1,
  };

  return (
    <div
      ref={setNodeRef}
      style={style}
      className="surface-panel-subtle flex items-center gap-3 rounded-xl px-3 py-2"
    >
      {!readOnly ? (
        <button
          type="button"
          aria-label={`Drag item ${item.media_item_id}`}
          className="hover:bg-surface-hover cursor-grab touch-none rounded-md p-1 transition-colors"
          {...attributes}
          {...listeners}
        >
          <GripVertical className="text-muted-foreground h-4 w-4" />
        </button>
      ) : null}
      <span className="text-muted-foreground w-8 shrink-0 text-right text-xs tabular-nums">
        {index + 1}
      </span>
      <code className="text-muted-foreground min-w-0 flex-1 truncate text-xs">
        {item.media_item_id}
      </code>
      {!readOnly ? (
        <Button
          variant="ghost"
          size="icon"
          className="text-destructive hover:bg-destructive/10 hover:text-destructive h-7 w-7"
          aria-label={`Remove item ${item.media_item_id}`}
          onClick={onRemove}
        >
          <Trash2 className="h-3 w-3" />
        </Button>
      ) : null}
    </div>
  );
}
