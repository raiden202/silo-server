import { useId, useMemo, useState } from "react";
import type { FormEvent } from "react";
import type { AdminGroup, AdminUser, CreateUserRequest, UpdateUserRequest } from "@/api/types";
import { useAdminGroups } from "@/hooks/queries/admin/groups";
import { Skeleton } from "@/components/ui/skeleton";
import { useCreateUser, useUpdateUser } from "@/hooks/queries/admin/users";
import { EffectiveAccessSummary } from "@/components/admin/EffectiveAccessSummary";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Switch } from "@/components/ui/switch";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { formatLibraryAccess, formatLimit } from "@/lib/group-policy";
import { formatPlaybackQualityPreset } from "@/lib/playback-quality";
import { permissionLabel } from "@/lib/permissions";

/** Slug of the built-in group preselected for new users. */
const USERS_GROUP_SLUG = "users";

const GROUP_UNION_HINT =
  "Users in multiple groups get the most permissive combination of their groups.";

function groupPolicySummary(group: AdminGroup): string {
  const parts: string[] = [];
  parts.push(
    group.permissions.length === 0
      ? "No permissions"
      : group.permissions.map(permissionLabel).join(", "),
  );
  parts.push(formatLibraryAccess(group.library_ids));
  parts.push(`${formatLimit(group.max_streams)} streams`);
  parts.push(`${formatPlaybackQualityPreset(group.max_playback_quality)} quality`);
  parts.push(group.download_allowed ? "Downloads" : "No downloads");
  return parts.join(" · ");
}

function GroupMembershipSelector({
  groups,
  selectedIds,
  onChange,
}: {
  groups: AdminGroup[];
  selectedIds: number[];
  onChange: (ids: number[]) => void;
}) {
  if (groups.length === 0) {
    return <p className="text-muted-foreground text-sm">No groups available.</p>;
  }

  return (
    <div className="space-y-2">
      {groups.map((group) => (
        <div
          key={group.id}
          className="border-border flex items-center justify-between gap-3 rounded-md border px-3 py-2"
        >
          <div className="min-w-0">
            <div className="flex flex-wrap items-center gap-2">
              <span className="text-sm font-medium">{group.name}</span>
              {group.built_in && <Badge variant="secondary">Built-in</Badge>}
            </div>
            <p className="text-muted-foreground text-xs">{groupPolicySummary(group)}</p>
          </div>
          <Switch
            checked={selectedIds.includes(group.id)}
            aria-label={`Member of ${group.name}`}
            onCheckedChange={(checked) =>
              onChange(
                checked ? [...selectedIds, group.id] : selectedIds.filter((id) => id !== group.id),
              )
            }
          />
        </div>
      ))}
    </div>
  );
}

/**
 * Create/edit form for admin users. Identity fields are edited directly;
 * permissions, library access, and limits are managed via group membership.
 */
export function AdminUserForm({ user, onClose }: { user: AdminUser | null; onClose: () => void }) {
  const { data: groups, isLoading: groupsLoading, isError: groupsError } = useAdminGroups();
  const groupsLoaded = !groupsLoading && !groupsError && groups !== undefined;
  const [username, setUsername] = useState(user?.username ?? "");
  const [email, setEmail] = useState(user?.email ?? "");
  const [password, setPassword] = useState("");
  const [enabled, setEnabled] = useState(user?.enabled ?? true);
  // null = untouched; new users fall back to the built-in default group once
  // the groups list loads.
  const [groupIds, setGroupIds] = useState<number[] | null>(
    user ? user.groups.map((g) => g.id) : null,
  );
  const defaultGroupIds = useMemo(() => {
    const usersGroup = groups?.find((g) => g.slug === USERS_GROUP_SLUG);
    return usersGroup ? [usersGroup.id] : [];
  }, [groups]);
  const selectedGroupIds = groupIds ?? defaultGroupIds;
  const usernameId = useId();
  const emailId = useId();
  const passwordId = useId();
  const enabledId = useId();
  const createMutation = useCreateUser();
  const updateMutation = useUpdateUser();
  const isPending = createMutation.isPending || updateMutation.isPending;

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    if (user) {
      const body: UpdateUserRequest = {
        username,
        email,
        enabled,
        group_ids: selectedGroupIds,
      };
      if (password) body.password = password;
      updateMutation.mutate({ id: user.id, body }, { onSuccess: onClose });
    } else {
      // Only include group_ids when the groups list has successfully loaded OR
      // the admin explicitly touched the selector. If groups are still loading
      // or errored and the selector was never touched, omit the field entirely
      // so the backend applies its own default-group provisioning.
      const includeGroupIds = groupsLoaded || groupIds !== null;
      const body: CreateUserRequest = {
        username,
        email,
        password,
        create_default_profile: true,
        ...(includeGroupIds && { group_ids: selectedGroupIds }),
      };
      createMutation.mutate(body, { onSuccess: onClose });
    }
  }

  return (
    <form onSubmit={handleSubmit} className="flex max-h-[70vh] flex-col">
      <Tabs defaultValue="account" className="min-h-0 flex-1">
        <TabsList variant="line" className="border-border mb-4 w-full justify-start border-b pb-1">
          <TabsTrigger value="account" className="flex-none px-1">
            Account
          </TabsTrigger>
          <TabsTrigger value="groups" className="flex-none px-1">
            Groups
          </TabsTrigger>
        </TabsList>

        <div className="min-h-0 flex-1 overflow-y-auto pr-1">
          <TabsContent value="account" className="mt-0 space-y-4">
            <div className="grid gap-3 sm:grid-cols-2">
              <div className="space-y-2">
                <Label htmlFor={usernameId}>Username</Label>
                <Input
                  id={usernameId}
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                  required
                />
              </div>
              <div className="space-y-2">
                <Label htmlFor={emailId}>Email</Label>
                <Input
                  id={emailId}
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  required
                />
              </div>
              <div className="space-y-2 sm:col-span-2">
                <Label htmlFor={passwordId}>
                  Password {user && "(leave blank to keep current)"}
                </Label>
                <Input
                  id={passwordId}
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required={!user}
                />
              </div>
            </div>
            {user && (
              <div className="border-border flex items-center justify-between rounded-md border px-3 py-2">
                <div>
                  <div className="text-sm font-medium">Account status</div>
                  <div className="text-muted-foreground text-xs">
                    Disable access without deleting the user.
                  </div>
                </div>
                <div className="flex items-center gap-2">
                  <Label htmlFor={enabledId} className="text-xs">
                    Enabled
                  </Label>
                  <Switch id={enabledId} checked={enabled} onCheckedChange={setEnabled} />
                </div>
              </div>
            )}
          </TabsContent>

          <TabsContent value="groups" className="mt-0 space-y-4">
            {groupsLoading ? (
              <div className="space-y-2">
                <Skeleton className="h-[52px] w-full rounded-md" />
                <Skeleton className="h-[52px] w-full rounded-md" />
                <Skeleton className="h-[52px] w-full rounded-md" />
              </div>
            ) : groupsError ? (
              <p className="text-muted-foreground text-sm">
                {"Couldn't load groups — the user will be created in the default group."}
              </p>
            ) : (
              <GroupMembershipSelector
                groups={groups ?? []}
                selectedIds={selectedGroupIds}
                onChange={setGroupIds}
              />
            )}
            {!groupsError && <p className="text-muted-foreground text-xs">{GROUP_UNION_HINT}</p>}
            {user && (
              <div className="space-y-2">
                <Label>Effective access</Label>
                <div className="border-border rounded-md border">
                  <EffectiveAccessSummary user={user} />
                </div>
              </div>
            )}
          </TabsContent>
        </div>
      </Tabs>

      <div className="border-border mt-4 border-t pt-4">
        <Button type="submit" className="w-full" disabled={isPending}>
          {isPending ? "Saving..." : "Save"}
        </Button>
      </div>
    </form>
  );
}
