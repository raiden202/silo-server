import type { FilterChipModel } from "@/lib/filterEasyMode";

const TEMPLATES: { name: string; desc: string; chips: FilterChipModel[] }[] = [
  {
    name: "Highly rated",
    desc: "rating ≥ 7.5",
    chips: [{ field: "rating_imdb", op: "gte", value: 7.5 }],
  },
  {
    name: "By decade",
    desc: "year range",
    chips: [{ field: "year", op: "between", value: [1990, 1999] }],
  },
  {
    name: "Genre + decade",
    desc: "two filters",
    chips: [
      { field: "genre", op: "contains", value: "Sci-Fi" },
      { field: "year", op: "between", value: [1980, 1989] },
    ],
  },
  {
    name: "Cast / crew",
    desc: "person filter",
    chips: [{ field: "cast", op: "is", value: "" }],
  },
  {
    name: "Watchlist + unwatched",
    desc: "profile-aware",
    chips: [{ field: "watched", op: "is", value: false }],
  },
  {
    name: "Empty",
    desc: "start blank",
    chips: [],
  },
];

interface Props {
  onPick: (chips: FilterChipModel[]) => void;
}

export default function QuickStartTemplates({ onPick }: Props) {
  return (
    <div className="flex flex-wrap gap-2">
      {TEMPLATES.map((t) => (
        <button
          key={t.name}
          type="button"
          onClick={() => onPick(t.chips)}
          className="rounded-md border border-white/10 bg-white/5 px-3 py-2 text-left text-xs hover:border-indigo-500"
        >
          <div className="font-semibold">{t.name}</div>
          <div className="text-[10px] opacity-55">{t.desc}</div>
        </button>
      ))}
    </div>
  );
}
