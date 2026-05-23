import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";

import { api } from "@/api/client";
import type {
  DownloadedSubtitle,
  SubtitleDownloadRequest,
  SubtitleSearchRequest,
  SubtitleSearchResponse,
} from "@/api/types";

import { subtitleKeys } from "./keys";

interface DownloadSubtitleResponse {
  subtitle: DownloadedSubtitle;
}

export async function fetchDownloadedSubtitles(
  mediaFileId: number,
  options?: RequestInit,
): Promise<DownloadedSubtitle[]> {
  const response = await api<{ subtitles: DownloadedSubtitle[] }>(
    `/subtitles/${mediaFileId}`,
    options,
  );
  return response.subtitles ?? [];
}

export async function searchSubtitles(
  request: SubtitleSearchRequest,
  options?: RequestInit,
): Promise<SubtitleSearchResponse> {
  return api<SubtitleSearchResponse>("/subtitles/search", {
    ...options,
    method: "POST",
    body: JSON.stringify(request),
  });
}

export async function downloadSubtitle(
  request: SubtitleDownloadRequest,
  options?: RequestInit,
): Promise<DownloadSubtitleResponse> {
  return api<DownloadSubtitleResponse>("/subtitles/download", {
    ...options,
    method: "POST",
    body: JSON.stringify(request),
  });
}

export function useDownloadedSubtitles(mediaFileId: number | undefined) {
  return useQuery({
    queryKey: mediaFileId != null ? subtitleKeys.downloaded(mediaFileId) : subtitleKeys.all,
    queryFn: () => fetchDownloadedSubtitles(mediaFileId!),
    enabled: mediaFileId != null,
  });
}

export function useDownloadSubtitle() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (request: SubtitleDownloadRequest) => downloadSubtitle(request),
    onSuccess: async (_response, request) => {
      toast.success("Subtitle downloaded");
      await queryClient.invalidateQueries({
        queryKey: subtitleKeys.downloaded(request.media_file_id),
      });
    },
    onError: (err) => {
      toast.error(err instanceof Error ? err.message : "Failed to download subtitle");
    },
  });
}
