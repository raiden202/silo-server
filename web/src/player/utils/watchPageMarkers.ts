import type { PlayerFileVersion, PlayerTimeRange } from "../types";

export function patchVersionMarkers(
  versions: PlayerFileVersion[],
  fileId: number,
  intro?: PlayerTimeRange | null,
  credits?: PlayerTimeRange | null,
): PlayerFileVersion[] {
  if (intro === undefined && credits === undefined) {
    return versions;
  }

  let changed = false;
  const nextVersions = versions.map((version) => {
    if (version.file_id !== fileId) {
      return version;
    }

    const nextIntro = intro === undefined ? version.intro : intro;
    const nextCredits = credits === undefined ? version.credits : credits;
    const introUnchanged =
      version.intro?.start === nextIntro?.start && version.intro?.end === nextIntro?.end;
    const creditsUnchanged =
      version.credits?.start === nextCredits?.start && version.credits?.end === nextCredits?.end;
    if (introUnchanged && creditsUnchanged) {
      return version;
    }

    changed = true;
    return {
      ...version,
      intro: nextIntro,
      credits: nextCredits,
    };
  });

  return changed ? nextVersions : versions;
}

export function resolveActiveVersionMarkers(
  version: Pick<PlayerFileVersion, "intro" | "credits"> | null | undefined,
): {
  intro: PlayerTimeRange | null;
  credits: PlayerTimeRange | null;
} {
  return {
    intro: version?.intro ?? null,
    credits: version?.credits ?? null,
  };
}
