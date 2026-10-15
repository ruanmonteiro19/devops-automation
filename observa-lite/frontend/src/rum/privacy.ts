export type PrivacyPolicy = {
  masking: "all" | "allowlist";
  public_text_selectors: string[];
  block_selectors: string[];
  exclude_paths: string[];
  capture_path: boolean;
};
export const blockedElements =
  "script,style,link,meta,base,iframe,object,embed,svg,math,img,video,audio,canvas,[data-vetra-block],[data-private],[data-dtrum-mask],.rr-block";
export const layoutProperties =
  "display position top left right bottom width height min-width max-width min-height max-height box-sizing margin-top margin-left margin-right margin-bottom padding-top padding-left padding-right padding-bottom border-width border-style border-color border-radius color background-color font-size font-weight line-height text-align white-space overflow overflow-x overflow-y flex flex-direction flex-wrap justify-content align-items align-self gap grid-template-columns grid-template-rows opacity visibility transform transform-origin".split(
    " ",
  );
export function safeURL(
  raw: string,
  capturePath: boolean,
  base = location.href,
): string {
  try {
    const u = new URL(raw, base);
    if (!/^https?:$/.test(u.protocol)) return "";
    return u.origin + (capturePath ? u.pathname : "/");
  } catch {
    return "";
  }
}
export function excluded(raw: string, policy: PrivacyPolicy): boolean {
  try {
    return policy.exclude_paths.some((p) =>
      new URL(raw, location.href).pathname.startsWith(p),
    );
  } catch {
    return true;
  }
}
export function publicText(
  element: Element | null,
  policy: PrivacyPolicy,
): boolean {
  if (!element || policy.masking !== "allowlist") return false;
  if (
    element.closest(
      "input,textarea,select,[contenteditable]," + blockedElements,
    )
  )
    return false;
  try {
    if (policy.block_selectors.some((s) => element.closest(s))) return false;
    return policy.public_text_selectors.some((s) => element.closest(s));
  } catch {
    return false;
  }
}
// No raw DOM, URL-bearing attributes, scripts, CSS text or input values cross the transport.
export function scrubReplay(
  raw: any,
  policy: PrivacyPolicy,
  resolveNode: (id: number) => Node | null,
): any | null {
  const event = structuredClone(raw);
  function attributes(id: number) {
    const node = resolveNode(id);
    const result: Record<string, string> = {};
    if (!(node instanceof Element)) return result;
    const style = getComputedStyle(node);
    result["style"] =
      layoutProperties
        .map((key) => `${key}:${style.getPropertyValue(key)}`)
        .filter((s) => !/url\s*\(|["'<>\\]/i.test(s))
        .join(";") + ";font-family:sans-serif";
    return result;
  }
  let count = 0;
  function sanitizeNode(n: any, depth = 0): any {
    if (++count > 6000 || depth > 80) throw new Error("Replay DOM limit");
    const dom = resolveNode(n.id);
    if (n.type === 3)
      return {
        type: 3,
        id: n.id,
        textContent: publicText(dom?.parentElement || null, policy)
          ? String(n.textContent).slice(0, 2048)
          : "***",
        vetraPublic: publicText(dom?.parentElement || null, policy),
      };
    if (n.type === 1)
      return { type: 1, id: n.id, name: "html", publicId: "", systemId: "" };
    if (n.type !== 0 && n.type !== 2)
      return { type: 3, id: n.id, textContent: "" };
    let blocked = false;
    if (dom instanceof Element) {
      try {
        blocked =
          dom.matches(blockedElements) ||
          policy.block_selectors.some((s) => dom.matches(s));
      } catch {
        blocked = true;
      }
    }
    return {
      type: n.type,
      id: n.id,
      ...(n.type === 2
        ? { tagName: blocked ? "div" : n.tagName, attributes: attributes(n.id) }
        : {}),
      childNodes: blocked
        ? []
        : (n.childNodes || []).map((c: any) => sanitizeNode(c, depth + 1)),
    };
  }
  if (event.type === 2)
    event.data = {
      node: sanitizeNode(event.data.node),
      initialOffset: event.data.initialOffset,
    };
  else if (event.type === 4)
    event.data = {
      href: safeURL(event.data.href, policy.capture_path),
      width: event.data.width,
      height: event.data.height,
    };
  else if (event.type === 3) {
    const data = event.data;
    if (![0, 1, 2, 3, 4, 5, 6].includes(data.source)) return null;
    if (data.source === 5)
      event.data = {
        source: 5,
        id: data.id,
        text: "***",
        isChecked: false,
      };
    if (data.source === 0)
      event.data = {
        source: 0,
        texts: (data.texts || []).map((v: any) => {
          const allowed = publicText(
            resolveNode(v.id)?.parentElement || null,
            policy,
          );
          return {
            id: v.id,
            value: allowed ? String(v.value).slice(0, 2048) : "***",
            vetraPublic: allowed,
          };
        }),
        attributes: (data.attributes || []).map((v: any) => ({
          id: v.id,
          attributes: attributes(v.id),
        })),
        removes: (data.removes || []).map((v: any) => ({
          id: v.id,
          parentId: v.parentId,
        })),
        adds: (data.adds || []).map((v: any) => ({
          parentId: v.parentId,
          nextId: v.nextId,
          node: sanitizeNode(v.node),
        })),
      };
  } else if (event.type !== 0 && event.type !== 1) return null;
  return event;
}
