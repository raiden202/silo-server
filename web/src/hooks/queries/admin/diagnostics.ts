import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toast } from "sonner";

import { api, apiResponse } from "@/api/client";
import type {
  DiagnosticDownloadResponse,
  DiagnosticReport,
  DiagnosticReportListResponse,
  DiagnosticStatus,
} from "@/api/types";
import { adminKeys } from "@/hooks/queries/keys";

export interface AdminDiagnosticsQuery {
  user_id?: number | string;
  platform?: string;
  report_type?: string;
  from?: string;
  to?: string;
  short_id?: string;
  limit?: number;
  cursor?: string;
}

function toQueryString(params: AdminDiagnosticsQuery) {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params)) {
    if (value === undefined || value === null || value === "") continue;
    search.set(key, String(value));
  }
  return search.toString();
}

export function useDiagnosticsStatus() {
  return useQuery({
    queryKey: adminKeys.diagnosticStatus(),
    queryFn: () => api<DiagnosticStatus>("/diagnostics/status"),
    staleTime: 30_000,
  });
}

export function useDiagnosticReports(params: AdminDiagnosticsQuery) {
  const query = toQueryString(params);
  return useQuery({
    queryKey: adminKeys.diagnosticReports({ ...params }),
    queryFn: () =>
      api<DiagnosticReportListResponse>(`/admin/diagnostics/reports${query ? `?${query}` : ""}`),
    staleTime: 5_000,
  });
}

export function useDiagnosticReport(id?: string) {
  return useQuery({
    queryKey: adminKeys.diagnosticReport(id),
    queryFn: () => api<DiagnosticReport>(`/admin/diagnostics/reports/${encodeURIComponent(id!)}`),
    enabled: Boolean(id),
  });
}

export function useDeleteDiagnosticReport() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) =>
      api<void>(`/admin/diagnostics/reports/${encodeURIComponent(id)}`, {
        method: "DELETE",
      }),
    onSuccess: (_result, id) => {
      queryClient.removeQueries({ queryKey: adminKeys.diagnosticReport(id) });
      void queryClient.invalidateQueries({
        queryKey: ["admin", "diagnostics", "reports"],
      });
      toast.success("Diagnostic report deleted");
    },
    onError: (error) => {
      toast.error(error instanceof Error ? error.message : "Failed to delete diagnostic report");
    },
  });
}

export async function downloadDiagnosticReport(report: DiagnosticReport) {
  const downloadWindow = window.open("about:blank", "_blank");
  if (downloadWindow) {
    downloadWindow.opener = null;
  }

  try {
    const response = await apiResponse(
      `/admin/diagnostics/reports/${encodeURIComponent(report.id)}/download`,
    );
    const contentType = response.headers
      .get("Content-Type")
      ?.split(";", 1)[0]
      ?.trim()
      .toLowerCase();

    if (contentType === "application/json") {
      const payload = (await response.json()) as Partial<DiagnosticDownloadResponse>;
      if (typeof payload.download_url !== "string" || !payload.download_url.trim()) {
        throw new Error("The server returned an invalid diagnostic download response.");
      }
      if (!downloadWindow) {
        throw new Error("The browser blocked the diagnostic download window.");
      }
      downloadWindow.location.href = payload.download_url;
      return;
    }

    downloadWindow?.close();
    const blob = await response.blob();
    const url = URL.createObjectURL(blob);
    const anchor = document.createElement("a");
    anchor.href = url;
    anchor.download = `silo-diagnostics-${report.short_id || report.id}.tar.gz`;
    anchor.style.display = "none";
    document.body.appendChild(anchor);
    anchor.click();
    anchor.remove();
    window.setTimeout(() => URL.revokeObjectURL(url), 0);
  } catch (error) {
    downloadWindow?.close();
    throw error;
  }
}
