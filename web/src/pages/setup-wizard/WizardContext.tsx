import { createContext, useCallback, useContext, useState } from "react";
import type { ReactNode } from "react";
import { useQuery } from "@tanstack/react-query";
import { api } from "@/api/client";
import type { Library, Profile } from "@/api/types";
import { useAuth } from "@/hooks/useAuth";

const SKIPPABLE_STEPS = [
  "library",
  "server",
  "integrations",
  "downloads",
  "recommendations",
] as const;
export type SkippableStep = (typeof SKIPPABLE_STEPS)[number];

const STORAGE_KEYS: Record<SkippableStep, string> = {
  library: "setup_wizard_skip_library",
  server: "setup_wizard_server_done",
  integrations: "setup_wizard_integrations_done",
  downloads: "setup_wizard_downloads_done",
  recommendations: "setup_wizard_recommendations_done",
};

function readFlag(step: SkippableStep): boolean {
  if (typeof window === "undefined") return false;
  return window.localStorage.getItem(STORAGE_KEYS[step]) === "true";
}

function writeFlag(step: SkippableStep, value: boolean) {
  if (value) {
    window.localStorage.setItem(STORAGE_KEYS[step], "true");
  } else {
    window.localStorage.removeItem(STORAGE_KEYS[step]);
  }
}

interface WizardContextValue {
  // Auth-derived
  user: ReturnType<typeof useAuth>["user"];
  profile: ReturnType<typeof useAuth>["profile"];
  setupRequired: boolean;
  setupInitialUser: ReturnType<typeof useAuth>["setupInitialUser"];
  selectProfile: ReturnType<typeof useAuth>["selectProfile"];

  // Queries
  profiles: Profile[];
  profilesLoading: boolean;
  refetchProfiles: () => void;
  libraries: Library[];
  librariesLoading: boolean;
  refetchLibraries: () => void;

  // Step completion flags
  stepDone: Record<SkippableStep, boolean>;
  markDone: (step: SkippableStep) => void;
}

const WizardCtx = createContext<WizardContextValue | null>(null);

export function useWizardContext() {
  const ctx = useContext(WizardCtx);
  if (!ctx) throw new Error("useWizardContext must be used within WizardProvider");
  return ctx;
}

function shouldRetrySetupQuery(failureCount: number, error: unknown) {
  if (error instanceof Error && "status" in error && (error as { status: number }).status === 429) {
    return false;
  }
  return failureCount < 1;
}

export function WizardProvider({ children }: { children: ReactNode }) {
  const { user, profile, setupRequired, setupInitialUser, selectProfile } = useAuth();
  const isAdmin = user?.role === "admin";

  const [stepDone, setStepDone] = useState<Record<SkippableStep, boolean>>(() => {
    const flags = {} as Record<SkippableStep, boolean>;
    for (const step of SKIPPABLE_STEPS) {
      flags[step] = readFlag(step);
    }
    return flags;
  });

  const profilesQuery = useQuery({
    queryKey: ["setup-wizard", "profiles"],
    queryFn: () => api<{ profiles: Profile[] }>("/profiles").then((d) => d.profiles ?? []),
    enabled: !!user,
    retry: shouldRetrySetupQuery,
  });

  const profileComplete = (profilesQuery.data ?? []).length > 0;

  const librariesQuery = useQuery({
    queryKey: ["setup-wizard", "libraries"],
    queryFn: () => api<Library[]>("/libraries").then((d) => d ?? []),
    enabled: isAdmin && profileComplete,
    retry: shouldRetrySetupQuery,
  });

  const markDone = useCallback((step: SkippableStep) => {
    writeFlag(step, true);
    setStepDone((prev) => ({ ...prev, [step]: true }));
  }, []);

  return (
    <WizardCtx.Provider
      value={{
        user,
        profile,
        setupRequired,
        setupInitialUser,
        selectProfile,
        profiles: profilesQuery.data ?? [],
        profilesLoading: !!user && profilesQuery.isPending,
        refetchProfiles: () => void profilesQuery.refetch(),
        libraries: librariesQuery.data ?? [],
        librariesLoading: isAdmin && profileComplete && librariesQuery.isPending,
        refetchLibraries: () => void librariesQuery.refetch(),
        stepDone,
        markDone,
      }}
    >
      {children}
    </WizardCtx.Provider>
  );
}
