import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type MutableRefObject,
  type RefObject,
} from "react";
import { getProfileToken } from "@/api/client";
import {
  closeWatchTogetherRoom,
  type GuestControlPolicy,
  type WatchTogetherRoomSnapshot,
  updateWatchTogetherRoomPolicy,
} from "@/lib/watchTogether";
import { usePlayerConfig } from "../context/PlayerConfigContext";
import { toMediaTime } from "../utils/mediaTimeline";

type ConnectionState = "disconnected" | "connecting" | "connected";

interface UseWatchTogetherRoomOptions {
  roomId?: string | null;
  roomToken?: string | null;
  sessionId?: string | null;
  videoRef: RefObject<HTMLVideoElement | null>;
  streamOriginRef: MutableRefObject<number>;
  playbackRealtimeConnected?: boolean;
}

interface UseWatchTogetherRoomResult {
  connectionState: ConnectionState;
  room: WatchTogetherRoomSnapshot | null;
  closedReason: string | null;
  requestTransport: (
    action: "play" | "pause" | "seek",
    positionSeconds: number,
    isPaused: boolean,
  ) => void;
  updatePolicy: (policy: GuestControlPolicy) => Promise<WatchTogetherRoomSnapshot | null>;
  closeRoom: () => Promise<void>;
}

const reconnectDelays = [500, 1_000, 2_000, 5_000];
const stateReportIntervalMs = 1_500;

function buildRoomWebSocketUrl(
  apiBaseUrl: string,
  roomId: string,
  roomToken: string,
  accessToken: string | null,
  profileId: string | null,
  profileToken: string | null,
) {
  if (typeof window === "undefined") {
    return "";
  }

  const apiBase = apiBaseUrl.startsWith("http")
    ? apiBaseUrl
    : new URL(apiBaseUrl, window.location.origin).toString();
  const wsBase = apiBase.replace(/^http/, "ws");
  const url = new URL(`${wsBase}/watch-together/rooms/${roomId}/ws`);
  url.searchParams.set("room_token", roomToken);
  if (accessToken) {
    url.searchParams.set("token", accessToken);
  }
  if (profileId) {
    url.searchParams.set("profile_id", profileId);
  }
  if (profileToken) {
    url.searchParams.set("profile_token", profileToken);
  }
  return url.toString();
}

export function useWatchTogetherRoom({
  roomId,
  roomToken,
  sessionId,
  videoRef,
  streamOriginRef,
  playbackRealtimeConnected,
}: UseWatchTogetherRoomOptions): UseWatchTogetherRoomResult {
  const config = usePlayerConfig();
  const stateRoomId = roomId ?? null;
  const [activeRoomId, setActiveRoomId] = useState<string | null>(stateRoomId);
  const [connectionStateValue, setConnectionState] = useState<ConnectionState>("disconnected");
  const [roomValue, setRoom] = useState<WatchTogetherRoomSnapshot | null>(null);
  const [closedReasonValue, setClosedReason] = useState<string | null>(null);
  const socketRef = useRef<WebSocket | null>(null);
  const roomRef = useRef<WatchTogetherRoomSnapshot | null>(null);
  const closedReasonRef = useRef<string | null>(null);
  const playbackRealtimeConnectedRef = useRef(playbackRealtimeConnected ?? true);
  const connectionState = activeRoomId === stateRoomId ? connectionStateValue : "disconnected";
  const room = activeRoomId === stateRoomId ? roomValue : null;
  const closedReason = activeRoomId === stateRoomId ? closedReasonValue : null;

  useEffect(() => {
    roomRef.current = room;
  }, [room]);

  useEffect(() => {
    closedReasonRef.current = closedReason;
  }, [closedReason]);

  useEffect(() => {
    playbackRealtimeConnectedRef.current = playbackRealtimeConnected ?? true;
  }, [playbackRealtimeConnected]);

  const websocketUrl = useMemo(() => {
    if (!roomId || !roomToken) {
      return null;
    }
    return buildRoomWebSocketUrl(
      config.apiBaseUrl,
      roomId,
      roomToken,
      config.getAccessToken(),
      config.getProfileId(),
      getProfileToken(),
    );
  }, [config, roomId, roomToken]);

  const sendMessage = useCallback((message: Record<string, unknown>) => {
    const socket = socketRef.current;
    if (!socket || socket.readyState !== WebSocket.OPEN) {
      return false;
    }
    socket.send(JSON.stringify(message));
    return true;
  }, []);

  useEffect(() => {
    if (!roomId || !roomToken || !websocketUrl) {
      return;
    }

    let disposed = false;
    let attempt = 0;
    let reconnectTimer: number | null = null;

    const scheduleReconnect = () => {
      if (disposed || closedReasonRef.current) {
        return;
      }
      const delay = reconnectDelays[Math.min(attempt, reconnectDelays.length - 1)];
      attempt += 1;
      reconnectTimer = window.setTimeout(connect, delay);
    };

    const connect = () => {
      if (disposed) {
        return;
      }

      setConnectionState("connecting");

      let socket: WebSocket;
      try {
        socket = new WebSocket(websocketUrl);
      } catch {
        scheduleReconnect();
        return;
      }

      socketRef.current = socket;

      socket.addEventListener("open", () => {
        if (disposed) {
          socket.close();
          return;
        }
        attempt = 0;
        setActiveRoomId(stateRoomId);
        setConnectionState("connected");
        if (sessionId && playbackRealtimeConnectedRef.current) {
          socket.send(JSON.stringify({ type: "attach_session", session_id: sessionId }));
        }
      });

      socket.addEventListener("message", (event) => {
        let message: Record<string, unknown>;
        try {
          message = JSON.parse(String(event.data)) as Record<string, unknown>;
        } catch {
          return;
        }

        switch (message.type) {
          case "snapshot": {
            const payload = message.room as WatchTogetherRoomSnapshot | undefined;
            if (payload) {
              setActiveRoomId(stateRoomId);
              roomRef.current = payload;
              setRoom(payload);
            }
            return;
          }
          case "room_closed":
            setActiveRoomId(stateRoomId);
            roomRef.current = null;
            closedReasonRef.current =
              typeof message.reason === "string" ? message.reason : "room_closed";
            setRoom(null);
            setClosedReason(closedReasonRef.current);
            socket.close();
            return;
          case "pong":
            return;
          default:
            if (message.type === "error") {
              console.warn("[watch-together]", message.code ?? "error", message.message ?? "");
            }
        }
      });

      socket.addEventListener("close", () => {
        setActiveRoomId(stateRoomId);
        setConnectionState("disconnected");
        if (socketRef.current === socket) {
          socketRef.current = null;
        }
        scheduleReconnect();
      });

      socket.addEventListener("error", () => {
        socket.close();
      });
    };

    connect();

    return () => {
      disposed = true;
      if (reconnectTimer !== null) {
        window.clearTimeout(reconnectTimer);
      }
      if (socketRef.current) {
        const socket = socketRef.current;
        socketRef.current = null;
        if (socket.readyState === WebSocket.OPEN || socket.readyState === WebSocket.CONNECTING) {
          socket.close();
        }
      }
    };
  }, [roomId, roomToken, sessionId, stateRoomId, websocketUrl]);

  useEffect(() => {
    if (
      !roomId ||
      !sessionId ||
      connectionState !== "connected" ||
      playbackRealtimeConnected === false
    ) {
      return;
    }

    sendMessage({ type: "attach_session", session_id: sessionId });
  }, [connectionState, playbackRealtimeConnected, roomId, sendMessage, sessionId]);

  useEffect(() => {
    if (
      !roomId ||
      !sessionId ||
      connectionState !== "connected" ||
      playbackRealtimeConnected === false
    ) {
      return;
    }

    const intervalId = window.setInterval(() => {
      const currentRoom = roomRef.current;
      const video = videoRef.current;
      if (!video || currentRoom?.attached_session_id !== sessionId) {
        return;
      }

      sendMessage({
        type: "state_report",
        session_id: sessionId,
        position_seconds: toMediaTime(video.currentTime, streamOriginRef.current),
        is_paused: video.paused,
      });
    }, stateReportIntervalMs);

    return () => {
      window.clearInterval(intervalId);
    };
  }, [
    connectionState,
    playbackRealtimeConnected,
    roomId,
    sendMessage,
    sessionId,
    streamOriginRef,
    videoRef,
  ]);

  const requestTransport = useCallback(
    (action: "play" | "pause" | "seek", positionSeconds: number, isPaused: boolean) => {
      if (!roomId) {
        return;
      }

      sendMessage({
        type: "transport_request",
        action,
        position_seconds: positionSeconds,
        is_paused: isPaused,
      });
    },
    [roomId, sendMessage],
  );

  const updatePolicy = useCallback(
    async (policy: GuestControlPolicy) => {
      if (!roomId) {
        return null;
      }

      const response = await updateWatchTogetherRoomPolicy(roomId, policy);
      setRoom(response.room);
      return response.room;
    },
    [roomId],
  );

  const closeRoom = useCallback(async () => {
    if (!roomId) {
      return;
    }

    await closeWatchTogetherRoom(roomId);
  }, [roomId]);

  return {
    connectionState,
    room,
    closedReason,
    requestTransport,
    updatePolicy,
    closeRoom,
  };
}
