import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

import { ServerStorageStep } from "./ServerStorageStep";

const useSettingsFormMock = vi.fn();
const useWizardContextMock = vi.fn();
const useCheckAdminSettingsConnectionMock = vi.fn();
const useQueryMock = vi.fn();

vi.mock("@tanstack/react-query", () => ({
  useQuery: (...args: unknown[]) => useQueryMock(...args),
}));

vi.mock("@/hooks/useSettingsForm", () => ({
  useSettingsForm: (...args: unknown[]) => useSettingsFormMock(...args),
}));

vi.mock("../WizardContext", () => ({
  useWizardContext: (...args: unknown[]) => useWizardContextMock(...args),
}));

vi.mock("@/hooks/queries/admin/settings", () => ({
  useCheckAdminSettingsConnection: (...args: unknown[]) =>
    useCheckAdminSettingsConnectionMock(...args),
}));

describe("ServerStorageStep", () => {
  it("renders connection check actions for Redis and public/private S3 storage", () => {
    useWizardContextMock.mockReturnValue({ markDone: vi.fn() });
    useQueryMock.mockReturnValue({ data: null });
    useCheckAdminSettingsConnectionMock.mockReturnValue({
      isPending: false,
      mutateAsync: vi.fn(),
    });
    useSettingsFormMock.mockReturnValue({
      isLoading: false,
      getValue: (key: string) => {
        if (key === "s3.public_url_auth") return "presigned";
        return "";
      },
      setValue: vi.fn(),
      dirtyCount: 0,
      save: vi.fn(),
      discard: vi.fn(),
      isSaving: false,
      restartRequired: false,
      sensitiveConfigured: [],
      buildConnectionCheckRequest: vi.fn(),
    });

    const markup = renderToStaticMarkup(<ServerStorageStep />);

    expect(markup).toContain("Public Assets Storage");
    expect(markup).toContain("Private Internal Storage");
    expect(markup).toContain("Check Connection");
  });
});
