import type { User } from "@/api/types";

export const PERMISSION_ADMIN = "admin";
export const PERMISSION_METADATA_CURATION = "metadata_curation";
export const PERMISSION_MARKER_EDIT = "marker_edit";

const PERMISSION_LABELS: Record<string, string> = {
  [PERMISSION_ADMIN]: "Administrator",
  [PERMISSION_MARKER_EDIT]: "Marker Editing",
  [PERMISSION_METADATA_CURATION]: "Metadata Curation",
};

export function permissionLabel(permission: string) {
  return PERMISSION_LABELS[permission] ?? permission;
}

export function hasPermission(
  user: Pick<User, "role" | "permissions"> | null | undefined,
  permission: string,
) {
  if (!user) return false;
  if (user.role === "admin") return true;
  return Array.isArray(user.permissions) && user.permissions.includes(permission);
}

export function canCurateMetadata(user: Pick<User, "role" | "permissions"> | null | undefined) {
  return hasPermission(user, PERMISSION_METADATA_CURATION);
}

export function canEditMarkers(user: Pick<User, "role" | "permissions"> | null | undefined) {
  return hasPermission(user, PERMISSION_MARKER_EDIT);
}

export function hasAssignedPermission(permissions: string[] | undefined, permission: string) {
  return Array.isArray(permissions) && permissions.includes(permission);
}

export function setAssignedPermission(permissions: string[], permission: string, enabled: boolean) {
  const next = new Set(permissions);
  if (enabled) {
    next.add(permission);
  } else {
    next.delete(permission);
  }
  return Array.from(next).sort();
}
