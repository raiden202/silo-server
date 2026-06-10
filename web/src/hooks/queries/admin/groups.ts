import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { api } from "@/api/client";
import type {
  AdminGroup,
  CreateGroupRequest,
  GroupMembersPage,
  UpdateGroupRequest,
} from "@/api/types";
import { adminKeys } from "../keys";
import { toast } from "sonner";

const ADMIN_STALE_TIME = 30_000;

interface AdminGroupsResponse {
  groups: AdminGroup[];
}

// Group mutations also invalidate the admin users caches: membership and
// policy changes alter the effective fields on admin user rows.
function invalidateGroupCaches(queryClient: ReturnType<typeof useQueryClient>) {
  queryClient.invalidateQueries({ queryKey: adminKeys.groups() });
  queryClient.invalidateQueries({ queryKey: adminKeys.users() });
}

export function useAdminGroups() {
  return useQuery({
    queryKey: adminKeys.groups(),
    queryFn: () => api<AdminGroupsResponse>("/admin/groups").then((d) => d?.groups ?? []),
    staleTime: ADMIN_STALE_TIME,
  });
}

export function useAdminGroup(id: number) {
  return useQuery({
    queryKey: adminKeys.groupDetail(id),
    queryFn: () => api<AdminGroup>(`/admin/groups/${id}`),
    enabled: id > 0,
    staleTime: ADMIN_STALE_TIME,
  });
}

export function useGroupMembers(id: number, offset = 0, limit = 50) {
  return useQuery({
    queryKey: adminKeys.groupMembers(id, offset, limit),
    queryFn: () =>
      api<GroupMembersPage>(`/admin/groups/${id}/members?offset=${offset}&limit=${limit}`),
    enabled: id > 0,
    staleTime: ADMIN_STALE_TIME,
  });
}

export function useCreateGroup() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (body: CreateGroupRequest) =>
      api<AdminGroup>("/admin/groups", {
        method: "POST",
        body: JSON.stringify(body),
      }),
    onSuccess: () => {
      toast.success("Group created");
      invalidateGroupCaches(queryClient);
    },
    onError: (err) => {
      toast.error(err instanceof Error ? err.message : "Failed to save");
    },
  });
}

export function useUpdateGroup() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ id, body }: { id: number; body: UpdateGroupRequest }) =>
      api<AdminGroup>(`/admin/groups/${id}`, {
        method: "PATCH",
        body: JSON.stringify(body),
      }),
    onSuccess: () => {
      toast.success("Group updated");
      invalidateGroupCaches(queryClient);
    },
    onError: (err) => {
      toast.error(err instanceof Error ? err.message : "Failed to save");
    },
  });
}

export function useDeleteGroup() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: number) => api(`/admin/groups/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      toast.success("Group deleted");
      invalidateGroupCaches(queryClient);
    },
    onError: (err) => {
      toast.error(err instanceof Error ? err.message : "Failed to delete");
    },
  });
}

export function useAddGroupMember() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ groupId, userId }: { groupId: number; userId: number }) =>
      api(`/admin/groups/${groupId}/members/${userId}`, { method: "PUT" }),
    onSuccess: () => {
      toast.success("Member added");
      invalidateGroupCaches(queryClient);
    },
    onError: (err) => {
      toast.error(err instanceof Error ? err.message : "Failed to add member");
    },
  });
}

export function useRemoveGroupMember() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: ({ groupId, userId }: { groupId: number; userId: number }) =>
      api(`/admin/groups/${groupId}/members/${userId}`, { method: "DELETE" }),
    onSuccess: () => {
      toast.success("Member removed");
      invalidateGroupCaches(queryClient);
    },
    onError: (err) => {
      toast.error(err instanceof Error ? err.message : "Failed to remove member");
    },
  });
}
