import { describe, it, expect } from "vitest";
import { newRUMApplication, replayPages, vitalHealth } from "./rum";
describe("Digital experience metrics", () => {
  it("starts with consent required and replay disabled", () => {
    const app = newRUMApplication();
    expect(app.require_consent).toBe(true);
    expect(app.replay_enabled).toBe(false);
    expect(app.capture_path).toBe(false);
    expect(app.identify_users).toBe(false);
  });
  it("keeps missing data distinct from good measurements", () => {
    expect(vitalHealth("INP", null)).toBe("unknown");
    expect(vitalHealth("INP", 200)).toBe("good");
    expect(vitalHealth("INP", 201)).toBe("needs-improvement");
    expect(vitalHealth("LCP", 4001)).toBe("poor");
    expect(vitalHealth("CLS", 0.25)).toBe("needs-improvement");
  });
  it("rejects incomplete pages and preserves page boundaries", () => {
    const pages = replayPages([
      {
        id: "c1",
        app_id: "a",
        session_id: "s",
        page_id: "one",
        at: "",
        events: [
          { type: 4, timestamp: 1, data: {} },
          { type: 2, timestamp: 2, data: {} },
          { type: 3, timestamp: 3, data: {} },
        ],
      },
      {
        id: "c2",
        app_id: "a",
        session_id: "s",
        page_id: "two",
        at: "",
        events: [{ type: 3, timestamp: 4, data: {} }],
      },
    ]);
    expect(pages.size).toBe(1);
    expect(pages.get("one")?.map((e) => e.type)).toEqual([4, 2, 3]);
  });
});
