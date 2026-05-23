import { describe, expect, it } from "vitest";
import { buildEventsUrl } from "./RealtimeEventsProvider";

describe("buildEventsUrl", () => {
  it("includes auth token and websocket scheme", () => {
    expect(
      buildEventsUrl("token-123", {
        protocol: "https:",
        host: "example.com",
      }),
    ).toBe("wss://example.com/api/v1/events/ws?token=token-123");
  });

  it("omits the query string when no token is available", () => {
    expect(
      buildEventsUrl(null, {
        protocol: "http:",
        host: "localhost:5173",
      }),
    ).toBe("ws://localhost:5173/api/v1/events/ws");
  });
});
