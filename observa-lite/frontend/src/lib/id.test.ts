import { expect, it, vi } from "vitest";
import { newID } from "./id";
it("creates distinct valid IDs without randomUUID on HTTP origins", () => {
  vi.stubGlobal("crypto", {
    getRandomValues: (b: Uint8Array) => {
      for (let i = 0; i < b.length; i++) b[i] = Math.floor(Math.random() * 256);
      return b;
    },
  });
  try {
    const ids = new Set(Array.from({ length: 1000 }, () => newID()));
    expect(ids.size).toBe(1000);
    for (const id of ids)
      expect(id).toMatch(
        /^[a-f0-9]{8}-[a-f0-9]{4}-4[a-f0-9]{3}-[89ab][a-f0-9]{3}-[a-f0-9]{12}$/,
      );
  } finally {
    vi.unstubAllGlobals();
  }
});
