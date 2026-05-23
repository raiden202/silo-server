import { Link } from "react-router";
import { ChevronLeft, ChevronRight } from "lucide-react";

interface BreadcrumbSegment {
  label: string;
  href?: string;
}

interface DetailBreadcrumbProps {
  segments: BreadcrumbSegment[];
}

export default function DetailBreadcrumb({ segments }: DetailBreadcrumbProps) {
  if (segments.length === 0) return null;

  const backSegment = [...segments].reverse().find((segment) => segment.href);

  return (
    <nav aria-label="Breadcrumb">
      <div className="flex items-center gap-2.5">
        {backSegment?.href && (
          <Link
            to={backSegment.href}
            className="glass-subtle text-muted-foreground hover:text-foreground flex items-center justify-center rounded-full p-1.5 transition-colors"
          >
            <ChevronLeft className="size-5" />
          </Link>
        )}
        <div className="flex items-center gap-1.5">
          {segments.map((segment, i) => {
            const isLast = i === segments.length - 1;
            return (
              <span key={i} className="flex items-center gap-1.5">
                {i > 0 && <ChevronRight className="text-muted-foreground/40 size-4" />}
                {segment.href && !isLast ? (
                  <Link
                    to={segment.href}
                    className="text-muted-foreground hover:text-foreground text-[13px] transition-colors"
                  >
                    {segment.label}
                  </Link>
                ) : (
                  <span
                    className="text-muted-foreground/60 text-[13px]"
                    {...(isLast ? { "aria-current": "page" as const } : {})}
                  >
                    {segment.label}
                  </span>
                )}
              </span>
            );
          })}
        </div>
      </div>
    </nav>
  );
}
