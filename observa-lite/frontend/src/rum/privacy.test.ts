// @vitest-environment jsdom
import { describe, it, expect } from "vitest";
import {
  excluded,
  publicText,
  safeURL,
  scrubReplay,
  type PrivacyPolicy,
} from "./privacy";
const policy: PrivacyPolicy = {
  masking: "all",
  public_text_selectors: [],
  block_selectors: [],
  exclude_paths: ["/login"],
  capture_path: false,
};
describe("Capture privacy", () => {
  it("drops credentials, query strings, fragments and paths by default", () => {
    expect(
      safeURL(
        "https://user:password@shop.test/private?token=SECRET#secret",
        false,
      ),
    ).toBe("https://shop.test/");
    expect(safeURL("https://shop.test/orders?token=SECRET#secret", true)).toBe(
      "https://shop.test/orders",
    );
    expect(safeURL("javascript:alert(1)", true)).toBe("");
    expect(excluded("https://shop.test/login/step", policy)).toBe(true);
  });
  it("never unmasks editable values or explicitly private regions", () => {
    document.body.innerHTML =
      '<div data-public><p>Public label</p><input value="secret"><span data-private>Private</span></div>';
    const p = {
      ...policy,
      masking: "allowlist" as const,
      public_text_selectors: ["[data-public]"],
    };
    expect(publicText(document.querySelector("p"), p)).toBe(true);
    expect(publicText(document.querySelector("input"), p)).toBe(false);
    expect(publicText(document.querySelector("span"), p)).toBe(false);
  });
  it("strips secrets from DOM, attributes, mutation values and inputs before transport", () => {
    document.body.innerHTML =
      '<p id="secret">EMAIL_SECRET</p><input value="PASSWORD_SECRET">';
    const node = document.querySelector("p")!;
    const resolve = (id: number) =>
      id === 2 ? node.firstChild : id === 1 ? node : null;
    const raw = {
      type: 2,
      timestamp: 1,
      data: {
        node: {
          id: 1,
          type: 2,
          tagName: "p",
          attributes: { title: "TITLE_SECRET", onclick: "SCRIPT_SECRET" },
          childNodes: [{ id: 2, type: 3, textContent: "EMAIL_SECRET" }],
        },
        initialOffset: { top: 0, left: 0 },
      },
    };
    const safe = scrubReplay(raw, policy, resolve);
    expect(JSON.stringify(safe)).not.toMatch(
      /EMAIL_SECRET|TITLE_SECRET|SCRIPT_SECRET/,
    );
    expect(safe.data.node.childNodes[0].textContent).toBe("***");
    const input = scrubReplay(
      {
        type: 3,
        timestamp: 2,
        data: { source: 5, id: 9, text: "PASSWORD_SECRET", isChecked: true },
      },
      policy,
      resolve,
    );
    expect(input.data.text).toBe("***");
    expect(input.data.isChecked).toBe(false);
  });
});
