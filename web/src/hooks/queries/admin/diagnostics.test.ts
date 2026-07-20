import { beforeEach, describe, expect, it, vi } from "vitest";

import type { DiagnosticReport } from "@/api/types";

const mocks = vi.hoisted(() => ({
  apiResponse: vi.fn(),
}));

vi.mock("@/api/client", () => ({
  api: vi.fn(),
  apiResponse: mocks.apiResponse,
}));

import { downloadDiagnosticReport } from "./diagnostics";

const report = {
  id: "83fd3186-bd4f-42e1-8285-58107c503685",
  short_id: "ABCDEF123456",
} as DiagnosticReport;

describe("downloadDiagnosticReport", () => {
  beforeEach(() => {
    mocks.apiResponse.mockReset();
  });

  it("opens a presigned download URL returned as JSON", async () => {
    const downloadWindow = {
      close: vi.fn(),
      location: { href: "about:blank" },
      opener: window,
    } as unknown as Window;
    const open = vi.spyOn(window, "open").mockReturnValue(downloadWindow);
    mocks.apiResponse.mockResolvedValue(
      new Response(
        JSON.stringify({
          download_url: "https://storage.example/report.tar.gz?signature=test",
          expires_at: "2026-07-20T18:00:00Z",
        }),
        { headers: { "Content-Type": "application/json; charset=utf-8" } },
      ),
    );

    await downloadDiagnosticReport(report);

    expect(mocks.apiResponse).toHaveBeenCalledWith(
      "/admin/diagnostics/reports/83fd3186-bd4f-42e1-8285-58107c503685/download",
    );
    expect(open).toHaveBeenCalledWith("about:blank", "_blank");
    expect(downloadWindow.opener).toBeNull();
    expect(downloadWindow.location.href).toBe(
      "https://storage.example/report.tar.gz?signature=test",
    );
    open.mockRestore();
  });

  it("downloads a streamed gzip fallback as a file", async () => {
    const close = vi.fn();
    const open = vi.spyOn(window, "open").mockReturnValue({ close } as unknown as Window);
    const objectURL = vi.spyOn(URL, "createObjectURL").mockReturnValue("blob:diagnostic");
    const revokeObjectURL = vi.spyOn(URL, "revokeObjectURL").mockImplementation(() => undefined);
    const click = vi
      .spyOn(HTMLAnchorElement.prototype, "click")
      .mockImplementation(() => undefined);
    mocks.apiResponse.mockResolvedValue(
      new Response(new Blob(["bundle"]), { headers: { "Content-Type": "application/gzip" } }),
    );

    await downloadDiagnosticReport(report);

    expect(click).toHaveBeenCalledOnce();
    expect(objectURL).toHaveBeenCalledOnce();
    expect(close).toHaveBeenCalledOnce();
    expect(document.querySelector('a[download="silo-diagnostics-ABCDEF123456.tar.gz"]')).toBeNull();

    await new Promise((resolve) => window.setTimeout(resolve, 0));
    expect(revokeObjectURL).toHaveBeenCalledWith("blob:diagnostic");
    objectURL.mockRestore();
    revokeObjectURL.mockRestore();
    click.mockRestore();
    open.mockRestore();
  });

  it("reports when a presigned download window is blocked", async () => {
    const open = vi.spyOn(window, "open").mockReturnValue(null);
    mocks.apiResponse.mockResolvedValue(
      new Response(JSON.stringify({ download_url: "https://storage.example/report.tar.gz" }), {
        headers: { "Content-Type": "application/json" },
      }),
    );

    await expect(downloadDiagnosticReport(report)).rejects.toThrow(
      "The browser blocked the diagnostic download window.",
    );
    open.mockRestore();
  });
});
