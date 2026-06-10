import { useMemo, useState, useId } from "react";
import type { FormEvent } from "react";
import type { AdminGroup, CreateGroupRequest, UpdateGroupRequest } from "@/api/types";
import {
  useAdminGroups,
  useGroupMembers,
  useCreateGroup,
  useUpdateGroup,
  useDeleteGroup,
  useAddGroupMember,
  useRemoveGroupMember,
} from "@/hooks/queries/admin/groups";
import { useAdminUsers } from "@/hooks/queries/admin/users";
import { useAdminLibraries } from "@/hooks/queries/admin/libraries";
import { LibraryAccessSelector } from "@/components/LibraryAccessSelector";
import { ConfirmDialog } from "@/components/ConfirmDialog";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Badge } from "@/components/ui/badge";
import { Skeleton } from "@/components/ui/skeleton";
import { Switch } from "@/components/ui/switch";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs";
import { Tooltip, TooltipContent, TooltipProvider, TooltipTrigger } from "@/components/ui/tooltip";
import { Check, Minus, Pencil, Plus, Search, Trash2, UserPlus, X } from "lucide-react";
import {
  PLAYBACK_QUALITY_OPTIONS,
  formatPlaybackQualityPreset,
  playbackQualityPresetFromValue,
  playbackQualityValueFromPreset,
  type PlaybackQualityPreset,
} from "@/lib/playback-quality";
import {
  PERMISSION_ADMIN,
  PERMISSION_MARKER_EDIT,
  PERMISSION_METADATA_CURATION,
  hasAssignedPermission,
  setAssignedPermission,
} from "@/lib/permissions";

const ADMINISTRATORS_SLUG = "administrators";
const MEMBERS_PAGE_SIZE = 50;

const PERMISSION_OPTIONS: Array<{ id: string; label: string; description: string }> = [
  {
    id: PERMISSION_ADMIN,
    label: "Administrator",
    description: "Full access to all features and settings.",
  },
  {
    id: PERMISSION_MARKER_EDIT,
    label: "Marker Editing",
    description: "Edit intro, recap, credits, and preview markers within assigned libraries.",
  },
  {
    id: PERMISSION_METADATA_CURATION,
    label: "Metadata Curation",
    description: "Edit, refresh, and rematch metadata within assigned libraries.",
  },
];

function permissionLabel(permission: string) {
  return PERMISSION_OPTIONS.find((option) => option.id === permission)?.label ?? permission;
}

export default function AdminGroups() {
  const { data: groups = [], isLoading } = useAdminGroups();
  const deleteMutation = useDeleteGroup();
  const [dialogOpen, setDialogOpen] = useState(false);
  const [editingGroup, setEditingGroup] = useState<AdminGroup | null>(null);
  const [confirmDeleteGroup, setConfirmDeleteGroup] = useState<AdminGroup | null>(null);

  if (isLoading)
    return (
      <div className="space-y-3">
        <Skeleton className="h-10 w-full rounded-lg" />
        {Array.from({ length: 5 }).map((_, i) => (
          <Skeleton key={i} className="h-12 w-full rounded-lg" />
        ))}
      </div>
    );

  return (
    <div className="space-y-6">
      <ConfirmDialog
        open={confirmDeleteGroup !== null}
        onOpenChange={(open) => {
          if (!open) setConfirmDeleteGroup(null);
        }}
        title="Delete group"
        description={`Delete group "${confirmDeleteGroup?.name}"? Members lose this group's permissions and limits. This action cannot be undone.`}
        confirmLabel="Delete"
        variant="destructive"
        onConfirm={() => {
          if (confirmDeleteGroup) deleteMutation.mutate(confirmDeleteGroup.id);
          setConfirmDeleteGroup(null);
        }}
      />
      <div className="page-header">
        <div className="space-y-3">
          <h1 className="page-title text-[clamp(2rem,4vw,3rem)]">Groups</h1>
          <p className="page-subtitle text-sm sm:text-base">
            Define permissions, library access, and playback limits shared by their members.
          </p>
        </div>
        <Dialog
          open={dialogOpen}
          onOpenChange={(open) => {
            setDialogOpen(open);
            if (!open) setEditingGroup(null);
          }}
        >
          <DialogTrigger asChild>
            <Button size="sm">
              <Plus className="mr-1 h-4 w-4" /> Add Group
            </Button>
          </DialogTrigger>
          <DialogContent className="sm:max-w-2xl">
            <DialogHeader>
              <DialogTitle>{editingGroup ? "Edit Group" : "Create Group"}</DialogTitle>
            </DialogHeader>
            <GroupForm
              group={editingGroup}
              onClose={() => {
                setDialogOpen(false);
                setEditingGroup(null);
              }}
            />
          </DialogContent>
        </Dialog>
      </div>

      <TooltipProvider>
        <div className="surface-panel overflow-x-auto rounded-2xl border-0">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Name</TableHead>
                <TableHead>Members</TableHead>
                <TableHead>Permissions</TableHead>
                <TableHead>Libraries</TableHead>
                <TableHead>Streams / Transcodes</TableHead>
                <TableHead>Quality</TableHead>
                <TableHead>Downloads</TableHead>
                <TableHead className="w-20">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {groups.map((group) => (
                <TableRow key={group.id}>
                  <TableCell>
                    <div className="flex items-center gap-2">
                      <span className="font-medium">{group.name}</span>
                      {group.built_in && <Badge variant="secondary">Built-in</Badge>}
                    </div>
                    {group.description && (
                      <div className="text-muted-foreground text-xs">{group.description}</div>
                    )}
                  </TableCell>
                  <TableCell>{group.member_count}</TableCell>
                  <TableCell>
                    {group.permissions.length === 0 ? (
                      <span className="text-muted-foreground text-xs">None</span>
                    ) : (
                      <div className="flex flex-wrap gap-1">
                        {group.permissions.map((permission) => (
                          <Badge
                            key={permission}
                            variant={permission === PERMISSION_ADMIN ? "default" : "outline"}
                          >
                            {permissionLabel(permission)}
                          </Badge>
                        ))}
                      </div>
                    )}
                  </TableCell>
                  <TableCell>
                    {group.library_ids === null
                      ? "All libraries"
                      : `${group.library_ids.length} ${
                          group.library_ids.length === 1 ? "library" : "libraries"
                        }`}
                  </TableCell>
                  <TableCell>
                    {formatLimit(group.max_streams)} / {formatLimit(group.max_transcodes)}
                  </TableCell>
                  <TableCell>{formatPlaybackQualityPreset(group.max_playback_quality)}</TableCell>
                  <TableCell>
                    {group.download_allowed ? (
                      <Check className="h-4 w-4 text-green-500" aria-label="Downloads allowed" />
                    ) : (
                      <Minus
                        className="text-muted-foreground h-4 w-4"
                        aria-label="Downloads not allowed"
                      />
                    )}
                  </TableCell>
                  <TableCell>
                    <div className="flex gap-1">
                      <Button
                        variant="ghost"
                        size="icon"
                        className="h-7 w-7"
                        aria-label={`Edit ${group.name}`}
                        onClick={() => {
                          setEditingGroup(group);
                          setDialogOpen(true);
                        }}
                      >
                        <Pencil className="h-3 w-3" aria-hidden="true" />
                      </Button>
                      {group.built_in ? (
                        <Tooltip>
                          <TooltipTrigger asChild>
                            <span tabIndex={0}>
                              <Button
                                variant="ghost"
                                size="icon"
                                className="h-7 w-7"
                                aria-label={`Delete ${group.name}`}
                                disabled
                              >
                                <Trash2 className="h-3 w-3" aria-hidden="true" />
                              </Button>
                            </span>
                          </TooltipTrigger>
                          <TooltipContent>Built-in groups cannot be deleted.</TooltipContent>
                        </Tooltip>
                      ) : (
                        <Button
                          variant="ghost"
                          size="icon"
                          className="h-7 w-7"
                          aria-label={`Delete ${group.name}`}
                          onClick={() => setConfirmDeleteGroup(group)}
                        >
                          <Trash2 className="h-3 w-3" aria-hidden="true" />
                        </Button>
                      )}
                    </div>
                  </TableCell>
                </TableRow>
              ))}
              {groups.length === 0 && (
                <TableRow>
                  <TableCell colSpan={8} className="text-muted-foreground py-8 text-center">
                    No groups yet. Create one to assign permissions and limits.
                  </TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </div>
      </TooltipProvider>
    </div>
  );
}

function formatLimit(value: number) {
  return value === 0 ? "∞" : String(value);
}

function GroupForm({ group, onClose }: { group: AdminGroup | null; onClose: () => void }) {
  const { data: libraries = [] } = useAdminLibraries();
  const isAdministrators = group?.slug === ADMINISTRATORS_SLUG;
  const [name, setName] = useState(group?.name ?? "");
  const [description, setDescription] = useState(group?.description ?? "");
  const [permissions, setPermissions] = useState<string[]>(group?.permissions ?? []);
  const [libraryIDs, setLibraryIDs] = useState<number[] | null>(group?.library_ids ?? null);
  const [maxStreams, setMaxStreams] = useState<number>(group?.max_streams ?? 6);
  const [maxTranscodes, setMaxTranscodes] = useState<number>(group?.max_transcodes ?? 2);
  const [maxProfiles, setMaxProfiles] = useState<number>(group?.max_profiles ?? 5);
  const [maxPlaybackQualityPreset, setMaxPlaybackQualityPreset] = useState<PlaybackQualityPreset>(
    playbackQualityPresetFromValue(group?.max_playback_quality),
  );
  const [downloadAllowed, setDownloadAllowed] = useState(group?.download_allowed ?? true);
  const [downloadTranscodeAllowed, setDownloadTranscodeAllowed] = useState(
    group?.download_transcode_allowed ?? false,
  );
  const nameId = useId();
  const descriptionId = useId();
  const maxStreamsId = useId();
  const maxTranscodesId = useId();
  const maxProfilesId = useId();
  const maxPlaybackQualityId = useId();
  const downloadAllowedId = useId();
  const downloadTranscodeAllowedId = useId();
  const createMutation = useCreateGroup();
  const updateMutation = useUpdateGroup();
  const isPending = createMutation.isPending || updateMutation.isPending;

  function handleSubmit(e: FormEvent) {
    e.preventDefault();
    const body: CreateGroupRequest & UpdateGroupRequest = {
      name,
      description,
      permissions,
      library_ids: libraryIDs,
      max_streams: maxStreams,
      max_transcodes: maxTranscodes,
      max_profiles: maxProfiles,
      max_playback_quality: playbackQualityValueFromPreset(maxPlaybackQualityPreset),
      download_allowed: downloadAllowed,
      download_transcode_allowed: downloadTranscodeAllowed,
    };
    if (group) {
      updateMutation.mutate({ id: group.id, body }, { onSuccess: onClose });
    } else {
      createMutation.mutate(body, { onSuccess: onClose });
    }
  }

  return (
    <form onSubmit={handleSubmit} className="flex max-h-[70vh] flex-col">
      <Tabs defaultValue="general" className="min-h-0 flex-1">
        <TabsList variant="line" className="border-border mb-4 w-full justify-start border-b pb-1">
          <TabsTrigger value="general" className="flex-none px-1">
            General
          </TabsTrigger>
          <TabsTrigger value="access" className="flex-none px-1">
            Access
          </TabsTrigger>
          <TabsTrigger value="limits" className="flex-none px-1">
            Limits
          </TabsTrigger>
          {group && (
            <TabsTrigger value="members" className="flex-none px-1">
              Members ({group.member_count})
            </TabsTrigger>
          )}
        </TabsList>

        <div className="min-h-0 flex-1 overflow-y-auto pr-1">
          <TabsContent value="general" className="mt-0 space-y-4">
            <div className="space-y-2">
              <Label htmlFor={nameId}>Name</Label>
              <Input id={nameId} value={name} onChange={(e) => setName(e.target.value)} required />
            </div>
            <div className="space-y-2">
              <Label htmlFor={descriptionId}>Description</Label>
              <Input
                id={descriptionId}
                value={description}
                onChange={(e) => setDescription(e.target.value)}
              />
            </div>
            <div className="space-y-2">
              <Label>Permissions</Label>
              {PERMISSION_OPTIONS.map((option) => {
                const locked = option.id === PERMISSION_ADMIN && isAdministrators;
                return (
                  <div
                    key={option.id}
                    className="border-border flex items-center justify-between gap-3 rounded-md border px-3 py-2"
                  >
                    <div>
                      <div className="text-sm font-medium">{option.label}</div>
                      <p className="text-muted-foreground text-xs">{option.description}</p>
                    </div>
                    <Switch
                      checked={locked || hasAssignedPermission(permissions, option.id)}
                      disabled={locked}
                      aria-label={option.label}
                      onCheckedChange={(checked) =>
                        setPermissions((current) =>
                          setAssignedPermission(current, option.id, checked),
                        )
                      }
                    />
                  </div>
                );
              })}
              {isAdministrators && (
                <p className="text-muted-foreground text-xs">
                  The administrators group always keeps the Administrator permission.
                </p>
              )}
            </div>
            <p className="text-muted-foreground text-xs">
              Users in multiple groups get the most permissive combination of their groups.
            </p>
          </TabsContent>

          <TabsContent value="access" className="mt-0 space-y-4">
            <LibraryAccessSelector
              libraries={libraries}
              value={libraryIDs}
              onChange={setLibraryIDs}
            />
          </TabsContent>

          <TabsContent value="limits" className="mt-0 space-y-4">
            <div className="grid gap-3 sm:grid-cols-2">
              <div className="space-y-1">
                <Label htmlFor={maxStreamsId}>Max Streams</Label>
                <Input
                  id={maxStreamsId}
                  type="number"
                  min={0}
                  value={maxStreams}
                  onChange={(e) => setMaxStreams(Number(e.target.value))}
                />
                <p className="text-muted-foreground text-xs">0 = unlimited</p>
              </div>
              <div className="space-y-1">
                <Label htmlFor={maxTranscodesId}>Max Transcodes</Label>
                <Input
                  id={maxTranscodesId}
                  type="number"
                  min={0}
                  value={maxTranscodes}
                  onChange={(e) => setMaxTranscodes(Number(e.target.value))}
                />
                <p className="text-muted-foreground text-xs">0 = unlimited</p>
              </div>
              <div className="space-y-1">
                <Label htmlFor={maxProfilesId}>Max Profiles</Label>
                <Input
                  id={maxProfilesId}
                  type="number"
                  min={1}
                  value={maxProfiles}
                  onChange={(e) => setMaxProfiles(Number(e.target.value))}
                />
              </div>
              <div className="space-y-1 sm:col-span-2">
                <Label htmlFor={maxPlaybackQualityId}>Max Playback Quality</Label>
                <Select
                  value={maxPlaybackQualityPreset}
                  onValueChange={(value) =>
                    setMaxPlaybackQualityPreset(value as PlaybackQualityPreset)
                  }
                >
                  <SelectTrigger id={maxPlaybackQualityId}>
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {PLAYBACK_QUALITY_OPTIONS.map((option) => (
                      <SelectItem key={option.value} value={option.value}>
                        {option.label}
                      </SelectItem>
                    ))}
                  </SelectContent>
                </Select>
                <p className="text-muted-foreground text-xs">
                  {
                    PLAYBACK_QUALITY_OPTIONS.find(
                      (option) => option.value === maxPlaybackQualityPreset,
                    )?.description
                  }
                </p>
              </div>
            </div>
            <div className="grid gap-2 sm:grid-cols-2">
              <div className="border-border flex items-center justify-between rounded-md border px-3 py-2">
                <Label htmlFor={downloadAllowedId}>Downloads Allowed</Label>
                <Switch
                  id={downloadAllowedId}
                  checked={downloadAllowed}
                  onCheckedChange={setDownloadAllowed}
                />
              </div>
              <div className="border-border flex items-center justify-between rounded-md border px-3 py-2">
                <Label htmlFor={downloadTranscodeAllowedId}>Download Transcode Allowed</Label>
                <Switch
                  id={downloadTranscodeAllowedId}
                  checked={downloadTranscodeAllowed}
                  onCheckedChange={setDownloadTranscodeAllowed}
                />
              </div>
            </div>
            <p className="text-muted-foreground text-xs">
              Users in multiple groups get the most permissive combination of their groups.
            </p>
          </TabsContent>

          {group && (
            <TabsContent value="members" className="mt-0 space-y-4">
              <GroupMembersSection group={group} />
            </TabsContent>
          )}
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

function GroupMembersSection({ group }: { group: AdminGroup }) {
  const [offset, setOffset] = useState(0);
  const { data, isLoading } = useGroupMembers(group.id, offset, MEMBERS_PAGE_SIZE);
  const { data: users = [] } = useAdminUsers();
  const addMutation = useAddGroupMember();
  const removeMutation = useRemoveGroupMember();
  const [search, setSearch] = useState("");

  const members = data?.members ?? [];
  const total = data?.total ?? 0;

  const candidates = useMemo(() => {
    const q = search.trim().toLowerCase();
    if (!q) return [];
    return users
      .filter(
        (u) =>
          !u.groups.some((g) => g.id === group.id) &&
          (u.username?.toLowerCase().includes(q) || u.email?.toLowerCase().includes(q)),
      )
      .slice(0, 8);
  }, [users, search, group.id]);

  function handleRemove(userId: number) {
    removeMutation.mutate(
      { groupId: group.id, userId },
      {
        onSuccess: () => {
          // Step back a page when removing the last member on a later page.
          if (members.length === 1 && offset > 0) {
            setOffset(offset - MEMBERS_PAGE_SIZE);
          }
        },
      },
    );
  }

  return (
    <div className="space-y-4">
      <div className="space-y-2">
        <div className="relative">
          <Search className="text-muted-foreground absolute top-1/2 left-3 h-4 w-4 -translate-y-1/2" />
          <Input
            placeholder="Search users to add..."
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            className="pr-9 pl-9"
          />
          {search && (
            <button
              type="button"
              aria-label="Clear search"
              onClick={() => setSearch("")}
              className="text-muted-foreground hover:text-foreground absolute top-1/2 right-3 -translate-y-1/2"
            >
              <X className="h-4 w-4" />
            </button>
          )}
        </div>
        {search.trim() && (
          <div className="grid gap-1.5">
            {candidates.length === 0 ? (
              <p className="text-muted-foreground text-xs">No matching users to add.</p>
            ) : (
              candidates.map((u) => (
                <div
                  key={u.id}
                  className="border-border flex items-center justify-between gap-3 rounded-md border px-3 py-1.5"
                >
                  <div className="min-w-0">
                    <span className="text-sm font-medium">{u.username}</span>
                    <span className="text-muted-foreground ml-2 text-xs">{u.email}</span>
                  </div>
                  <Button
                    type="button"
                    variant="outline"
                    size="sm"
                    disabled={addMutation.isPending}
                    onClick={() =>
                      addMutation.mutate(
                        { groupId: group.id, userId: u.id },
                        { onSuccess: () => setSearch("") },
                      )
                    }
                  >
                    <UserPlus className="mr-1 h-3 w-3" aria-hidden="true" /> Add
                  </Button>
                </div>
              ))
            )}
          </div>
        )}
      </div>

      {isLoading ? (
        <div className="space-y-2">
          {Array.from({ length: 3 }).map((_, i) => (
            <Skeleton key={i} className="h-10 w-full rounded-lg" />
          ))}
        </div>
      ) : members.length === 0 ? (
        <p className="text-muted-foreground text-sm">This group has no members.</p>
      ) : (
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Username</TableHead>
              <TableHead>Email</TableHead>
              <TableHead>Status</TableHead>
              <TableHead className="w-12">
                <span className="sr-only">Actions</span>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {members.map((member) => (
              <TableRow key={member.user_id}>
                <TableCell className="font-medium">{member.username}</TableCell>
                <TableCell>{member.email}</TableCell>
                <TableCell>
                  <Badge variant={member.enabled ? "outline" : "destructive"}>
                    {member.enabled ? "Active" : "Disabled"}
                  </Badge>
                </TableCell>
                <TableCell>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    className="h-7 w-7"
                    aria-label={`Remove ${member.username} from group`}
                    disabled={removeMutation.isPending}
                    onClick={() => handleRemove(member.user_id)}
                  >
                    <Trash2 className="h-3 w-3" aria-hidden="true" />
                  </Button>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      )}

      {total > MEMBERS_PAGE_SIZE && (
        <div className="flex items-center justify-between">
          <span className="text-muted-foreground text-sm">
            Showing {offset + 1}-{Math.min(offset + MEMBERS_PAGE_SIZE, total)} of {total}
          </span>
          <div className="flex gap-2">
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => setOffset((o) => Math.max(0, o - MEMBERS_PAGE_SIZE))}
              disabled={offset === 0}
            >
              Previous
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              onClick={() => setOffset((o) => o + MEMBERS_PAGE_SIZE)}
              disabled={offset + MEMBERS_PAGE_SIZE >= total}
            >
              Next
            </Button>
          </div>
        </div>
      )}
    </div>
  );
}
