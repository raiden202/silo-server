interface Props {
  kind: "server-default" | "yours";
  title: string;
  sectionType: string;
  hidden: boolean;
  onHide: () => void;
  onShow: () => void;
  onEdit: () => void;
  onDelete: () => void;
}

export default function ProfileSectionRow({
  kind,
  title,
  sectionType,
  hidden,
  onHide,
  onShow,
  onEdit,
  onDelete,
}: Props) {
  return (
    <div
      className={`grid grid-cols-[24px_1fr_120px_80px_80px] items-center gap-3 border-b border-white/5 px-2 py-2 text-sm ${hidden ? "opacity-55" : ""}`}
    >
      <span className="cursor-grab text-white/30">⋮⋮</span>
      <div>
        <div className="font-semibold">{title}</div>
        <div className="font-mono text-[10px] opacity-55">· {sectionType}</div>
      </div>
      <span
        className={`rounded px-1.5 py-0.5 text-[10px] ${kind === "yours" ? "bg-emerald-500/15 text-emerald-200" : "bg-indigo-500/15 text-indigo-200"}`}
      >
        {kind === "yours" ? "Yours" : "Server default"}
      </span>
      <span className="text-[10px] opacity-65">{hidden ? "hidden" : "on"}</span>
      <div className="flex justify-end gap-1">
        {kind === "server-default" ? (
          hidden ? (
            <button
              type="button"
              onClick={onShow}
              className="rounded border border-white/15 px-2 py-0.5 text-[11px]"
            >
              Show
            </button>
          ) : (
            <button
              type="button"
              onClick={onHide}
              className="rounded border border-white/15 px-2 py-0.5 text-[11px]"
            >
              Hide
            </button>
          )
        ) : (
          <>
            <button
              type="button"
              onClick={onEdit}
              className="rounded border border-white/15 px-2 py-0.5 text-[11px]"
            >
              Edit
            </button>
            <button
              type="button"
              onClick={onDelete}
              className="rounded border border-white/15 px-2 py-0.5 text-[11px]"
            >
              Delete
            </button>
          </>
        )}
      </div>
    </div>
  );
}
