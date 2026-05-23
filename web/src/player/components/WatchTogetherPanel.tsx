import type { GuestControlPolicy, WatchTogetherRoomSnapshot } from "@/lib/watchTogether";

interface WatchTogetherPanelProps {
  room: WatchTogetherRoomSnapshot | null;
  connectionState: "disconnected" | "connecting" | "connected";
  visible: boolean;
  onCopyInvite: () => void;
  onToggleGuestControl: (policy: GuestControlPolicy) => void;
  onEndRoom: () => void;
}

function connectionLabel(connectionState: WatchTogetherPanelProps["connectionState"]) {
  switch (connectionState) {
    case "connected":
      return "Live";
    case "connecting":
      return "Connecting";
    default:
      return "Offline";
  }
}

export function WatchTogetherPanel({
  room,
  connectionState,
  visible,
  onCopyInvite,
  onToggleGuestControl,
  onEndRoom,
}: WatchTogetherPanelProps) {
  const isHost = room?.self_can_manage_room === true;
  const policy = room?.guest_control_policy ?? "host_only";

  const policyLabel = isHost
    ? policy === "guest_play_pause"
      ? "Guests can pause & resume"
      : "Only you control playback"
    : room?.self_can_control_transport
      ? "You can pause & resume"
      : "Host controls playback";

  return (
    <div
      className={`absolute top-4 right-4 z-50 w-56 rounded-xl border border-white/10 bg-black/80 p-3 text-white shadow-2xl backdrop-blur-xl transition-opacity duration-300 ${
        visible ? "opacity-100" : "pointer-events-none opacity-0"
      }`}
    >
      {/* Label + status */}
      <div className="flex items-center gap-2 text-[10px] font-semibold tracking-[0.16em] text-white/60 uppercase">
        <span>Watch Party</span>
        <span
          className={`size-1.5 rounded-full ${
            connectionState === "connected"
              ? "bg-emerald-400"
              : connectionState === "connecting"
                ? "bg-amber-300"
                : "bg-white/35"
          }`}
        />
        <span>{connectionLabel(connectionState)}</span>
      </div>

      {/* Code + viewers */}
      <div className="mt-2 flex items-baseline gap-2">
        <span className="font-mono text-lg leading-none font-semibold tracking-wider">
          {room?.code ?? "..."}
        </span>
        {room ? (
          <span className="text-xs text-white/50">
            {room.member_count} {room.member_count === 1 ? "viewer" : "viewers"}
          </span>
        ) : null}
      </div>

      {/* Policy */}
      {room ? (
        <div className="mt-1.5 text-[11px] leading-snug text-white/50">{policyLabel}</div>
      ) : (
        <div className="mt-1.5 text-[11px] leading-snug text-white/50">Syncing room state</div>
      )}

      {/* Actions */}
      {isHost ? (
        <div className="mt-3 flex items-center gap-1 border-t border-white/8 pt-2.5">
          <button
            type="button"
            onClick={onCopyInvite}
            className="rounded-md bg-white/12 px-2.5 py-1 text-[11px] font-medium text-white/90 transition-colors hover:bg-white/20"
          >
            Invite
          </button>
          <button
            type="button"
            onClick={() =>
              onToggleGuestControl(policy === "guest_play_pause" ? "host_only" : "guest_play_pause")
            }
            className="rounded-md bg-white/12 px-2.5 py-1 text-[11px] font-medium text-white/90 transition-colors hover:bg-white/20"
          >
            {policy === "guest_play_pause" ? "Host Only" : "Allow Pause"}
          </button>
          <button
            type="button"
            onClick={onEndRoom}
            className="ml-auto rounded-md bg-red-500/25 px-2.5 py-1 text-[11px] font-medium text-red-100 transition-colors hover:bg-red-500/35"
          >
            End
          </button>
        </div>
      ) : null}
    </div>
  );
}
