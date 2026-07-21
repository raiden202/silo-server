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

  it("streams the bundle through the proxy download path", async () => {
    const objectURL = vi.spyOn(URL, "createObjectURL").mockReturnValue("blob:diagnostic");
    const revokeObjectURL = vi.spyOn(URL, "revokeObjectURL").mockImplementation(() => undefined);
    const click = vi
      .spyOn(HTMLAnchorElement.prototype, "click")
      .mockImplementation(() => undefined);
    mocks.apiResponse.mockResolvedValue(
      new Response(new Blob(["bundle"]), { headers: { "Content-Type": "application/gzip" } }),
    );

    await downloadDiagnosticReport(report);

    expect(mocks.apiResponse).toHaveBeenCalledWith(
      "/admin/diagnostics/reports/83fd3186-bd4f-42e1-8285-58107c503685/download?proxy=1",
    );
    expect(click).toHaveBeenCalledOnce();
    expect(objectURL).toHaveBeenCalledOnce();
    // The click spy records `this` as the anchor the download helper clicked;
    // assert it carried the blob URL and expected filename before cleanup.
    const clickedAnchor = click.mock.contexts[0] as HTMLAnchorElement;
    expect(clickedAnchor.href).toBe("blob:diagnostic");
    expect(clickedAnchor.download).toBe("silo-diagnostics-ABCDEF123456.tar.gz");
    expect(document.querySelector('a[download="silo-diagnostics-ABCDEF123456.tar.gz"]')).toBeNull();

    await new Promise((resolve) => window.setTimeout(resolve, 0));
    expect(revokeObjectURL).toHaveBeenCalledWith("blob:diagnostic");
    objectURL.mockRestore();
    revokeObjectURL.mockRestore();
    click.mockRestore();
  });

  it("propagates request failures to the caller", async () => {
    mocks.apiResponse.mockRejectedValue(new Error("Diagnostic report bundle not found"));

    await expect(downloadDiagnosticReport(report)).rejects.toThrow(
      "Diagnostic report bundle not found",
    );
  });
});
