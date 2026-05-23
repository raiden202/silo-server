export function toMediaTime(playerTimeSeconds: number, streamOriginSeconds = 0): number {
  return Math.max(0, playerTimeSeconds + streamOriginSeconds);
}

export function toPlayerTime(mediaTimeSeconds: number, streamOriginSeconds = 0): number {
  return Math.max(0, mediaTimeSeconds - streamOriginSeconds);
}
