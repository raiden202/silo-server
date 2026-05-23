import { useMutation, useQueryClient } from "@tanstack/react-query";
import { api, getAccessToken } from "@/api/client";
import { downloadKeys } from "./keys";
import { toast } from "sonner";

interface DownloadResponse {
  id: string;
  content_id: string;
  episode_id?: string;
  batch_id?: string;
  media_file_id: number;
  file_size: number;
  bytes_sent: number;
  kind: string;
  status: string;
  created_at: string;
  completed_at?: string;
}

interface CreateDownloadRequest {
  content_id: string;
  episode_id?: string;
  file_id?: number;
  series?: boolean;
}

export function useCreateDownload() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (req: CreateDownloadRequest) =>
      api<DownloadResponse | { downloads: DownloadResponse[] }>("/downloads", {
        method: "POST",
        body: JSON.stringify(req),
      }),
    onSuccess: (_data, req) => {
      toast.success(req.series ? "Series download queued" : "Download queued");
      qc.invalidateQueries({ queryKey: downloadKeys.all });
    },
    onError: () => {
      toast.error("Failed to start download");
    },
  });
}

export function useDeleteDownload() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => api(`/downloads/${id}`, { method: "DELETE" }),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: downloadKeys.all });
    },
  });
}

/**
 * Build a direct download URL for a specific file.
 * Uses a token query param since browsers can't set auth headers on navigations.
 */
export function buildDirectDownloadUrl(fileId: number): string {
  const token = getAccessToken();
  const params = new URLSearchParams({ file_id: String(fileId) });
  if (token) params.set("token", token);
  return `/api/v1/direct-download?${params.toString()}`;
}
