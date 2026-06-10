import type { ReactNode } from "react";
import type { AdminUser } from "@/api/types";
import { useAdminLibraries } from "@/hooks/queries/admin/libraries";
import { Badge } from "@/components/ui/badge";
import { formatPlaybackQualityPreset } from "@/lib/playback-quality";
import { PERMISSION_ADMIN, permissionLabel } from "@/lib/permissions";

function formatUnlimited(value: number) {
  return value === 0 ? "Unlimited" : String(value);
}

/**
 * Read-only summary of a user's effective access (permissions, library
 * access, and limits) derived from group membership.
 */
export function EffectiveAccessSummary({ user }: { user: AdminUser }) {
  const { data: libraries = [] } = useAdminLibraries();

  const libraryNames =
    user.library_ids === null
      ? "All libraries"
      : user.library_ids.length === 0
        ? "None"
        : user.library_ids
            .map((id) => libraries.find((l) => l.id === id)?.name ?? `#${id}`)
            .join(", ");

  return (
    <div>
      <div className="divide-border divide-y">
        <SummaryRow label="Permissions">
          {user.permissions.length === 0 ? (
            <span className="text-muted-foreground text-sm">None</span>
          ) : (
            <div className="flex flex-wrap justify-end gap-1">
              {user.permissions.map((permission) => (
                <Badge
                  key={permission}
                  variant={permission === PERMISSION_ADMIN ? "default" : "outline"}
                >
                  {permissionLabel(permission)}
                </Badge>
              ))}
            </div>
          )}
        </SummaryRow>
        <SummaryRow label="Library Access">{libraryNames}</SummaryRow>
        <SummaryRow label="Max Playback Quality">
          {formatPlaybackQualityPreset(user.max_playback_quality)}
        </SummaryRow>
        <SummaryRow label="Max Streams">{formatUnlimited(user.max_streams)}</SummaryRow>
        <SummaryRow label="Max Transcodes">{formatUnlimited(user.max_transcodes)}</SummaryRow>
        <SummaryRow label="Max Profiles">{formatUnlimited(user.max_profiles)}</SummaryRow>
        <SummaryRow label="Downloads">
          {user.download_allowed ? "Allowed" : "Not allowed"}
        </SummaryRow>
        <SummaryRow label="Download Transcode">
          {user.download_transcode_allowed ? "Allowed" : "Not allowed"}
        </SummaryRow>
      </div>
      <p className="text-muted-foreground px-4 py-2.5 text-xs">Derived from group membership.</p>
    </div>
  );
}

function SummaryRow({ label, children }: { label: string; children: ReactNode }) {
  return (
    <div className="flex items-center justify-between gap-4 px-4 py-2.5">
      <span className="text-muted-foreground shrink-0 text-sm">{label}</span>
      {typeof children === "string" ? (
        <span className="text-right text-sm font-medium">{children}</span>
      ) : (
        children
      )}
    </div>
  );
}
