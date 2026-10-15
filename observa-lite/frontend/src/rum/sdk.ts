import { onLCP, onINP, onCLS, onFCP, onTTFB, type Metric } from "web-vitals";
import { record } from "rrweb";
import {
  blockedElements,
  excluded,
  publicText,
  safeURL,
  scrubReplay,
  type PrivacyPolicy,
} from "./privacy";

type Policy = PrivacyPolicy & {
  id: string;
  enabled: boolean;
  require_consent: boolean;
  consent_banner: boolean;
  replay_enabled: boolean;
  sample_rate: number;
  replay_sample_rate: number;
  identify_users: boolean;
};
type Consent = { rum: boolean; replay: boolean };
type Session = { id: string; last: number; sampled: boolean; replay: boolean };
export type VetraAPI = {
  setConsent: (choice: boolean | Consent) => void;
  identify: (pseudonymousID: string) => Promise<void>;
  resetIdentity: () => void;
  showConsent: () => void;
  status: () => { active: boolean; replay: boolean; consent: Consent };
  flush: () => Promise<void>;
};
declare global {
  interface Window {
    Vetra?: VetraAPI;
  }
}
const script = document.currentScript as HTMLScriptElement | null;
if (script && !window.Vetra) void bootstrap(script);
export async function bootstrap(script: HTMLScriptElement): Promise<void> {
  const source = new URL(script.src, location.href);
  const appID = source.searchParams.get("app") || script.dataset["app"] || "";
  if (!/^[a-zA-Z0-9_-]{1,96}$/.test(appID)) return;
  const base = source.origin;
  const originalFetch = window.fetch.bind(window);
  const endpoint = `${base}/rum/beacon/${encodeURIComponent(appID)}`;
  const key = `vetra.rum.${appID}`;
  const consentKey = `vetra.consent.${appID}`;
  let policy: Policy | null = null,
    active = false,
    consent: Consent = { rum: false, replay: false },
    session: Session | null = null,
    pageID = "",
    userID = "",
    stopRecord: (() => void) | undefined,
    timer: ReturnType<typeof setInterval> | undefined;
  let pending: any[] = [],
    replay: any[] = [],
    replayBytes = 0,
    transmitting = false,
    aborter = new AbortController(),
    cleanups: (() => void)[] = [],
    banner: HTMLElement | null = null;
  const uid = () =>
    crypto.randomUUID?.() ||
    Array.from(crypto.getRandomValues(new Uint8Array(16)), (n) =>
      n.toString(16).padStart(2, "0"),
    ).join("");
  const optOut = () =>
    navigator.doNotTrack === "1" ||
    (navigator as Navigator & { globalPrivacyControl?: boolean })
      .globalPrivacyControl === true;
  const metadata = () => {
    const ua = navigator.userAgent;
    return {
      browser: /Edg\//.test(ua)
        ? "Edge"
        : /Firefox\//.test(ua)
          ? "Firefox"
          : /Chrome\//.test(ua)
            ? "Chrome"
            : /Safari\//.test(ua)
              ? "Safari"
              : "Other",
      os: /Android/.test(ua)
        ? "Android"
        : /iPhone|iPad/.test(ua)
          ? "iOS"
          : /Windows/.test(ua)
            ? "Windows"
            : /Mac/.test(ua)
              ? "macOS"
              : /Linux/.test(ua)
                ? "Linux"
                : "Other",
      device: /iPhone|Android.*Mobile/.test(ua)
        ? "Mobile"
        : /iPad|Tablet/.test(ua)
          ? "Tablet"
          : "Desktop",
    };
  };
  function blocked() {
    return !policy || excluded(location.href, policy);
  }
  function emit(kind: string, values: Record<string, unknown> = {}) {
    if (!active || !session || !policy || blocked()) return;
    session.last = Date.now();
    try {
      sessionStorage.setItem(key, JSON.stringify(session));
    } catch {
      /* storage may be blocked */
    }
    const event = {
      id: uid() as string,
      app_id: appID,
      session_id: session.id,
      page_id: pageID,
      at: new Date().toISOString(),
      kind,
      name: kind,
      url: safeURL(location.href, policy.capture_path),
      duration_ms: 0,
      value: 0,
      status: 0,
      error: false,
      ...metadata(),
      user_id: userID,
      ...values,
    };
    if (kind === "vital") {
      event.id = `${event.page_id}_${values["name"]}`;
      pending = pending.filter((e) => e.id !== event.id);
    }
    if (pending.length < 100) pending.push(event);
  }
  async function flush() {
    if (
      !active ||
      transmitting ||
      !session ||
      !policy ||
      blocked() ||
      (!pending.length && !replay.length)
    )
      return;
    const snapshot = {
      consent: consent.rum,
      events: pending.splice(0, 100),
      replay: replay.length
        ? [
            {
              id: uid(),
              app_id: appID,
              session_id: session.id,
              page_id: pageID,
              at: new Date().toISOString(),
              url: safeURL(location.href, policy.capture_path),
              events: replay.splice(0, 200),
            },
          ]
        : [],
    };
    replayBytes = JSON.stringify(replay).length;
    const body = JSON.stringify(snapshot);
    if (body.length > 1800000) return;
    transmitting = true;
    try {
      const response = await originalFetch(endpoint, {
        method: "POST",
        mode: "cors",
        credentials: "omit",
        headers: { "Content-Type": "text/plain;charset=UTF-8" },
        body,
        keepalive: body.length < 60000,
        signal: aborter.signal,
      });
      if (!response.ok && response.status >= 500) retry();
      if (response.status === 403 || response.status === 404) stop();
    } catch {
      if (active) retry();
    } finally {
      transmitting = false;
    }
    function retry() {
      if (active && pending.length + snapshot.events.length <= 100)
        pending.unshift(...snapshot.events);
      if (active && replay.length === 0 && snapshot.replay.length) {
        replay = snapshot.replay[0]!.events;
        replayBytes = JSON.stringify(replay).length;
      }
    }
  }
  function stop() {
    active = false;
    stopRecord?.();
    stopRecord = undefined;
    clearInterval(timer);
    cleanups.forEach((fn) => fn());
    cleanups = [];
    aborter.abort();
    aborter = new AbortController();
    pending = [];
    replay = [];
    replayBytes = 0;
    userID = "";
    session = null;
    try {
      sessionStorage.removeItem(key);
    } catch {}
  }
  function beginReplay() {
    if (
      !active ||
      !session?.replay ||
      !policy?.replay_enabled ||
      !consent.replay ||
      blocked() ||
      stopRecord
    )
      return;
    try {
      stopRecord = record({
        maskAllInputs: true,
        maskTextSelector: "*",
        maskTextFn: (text, el) =>
          publicText(el, policy!) ? text.slice(0, 2048) : "***",
        blockSelector: [blockedElements, ...policy.block_selectors].join(","),
        inlineStylesheet: false,
        inlineImages: false,
        collectFonts: false,
        recordCanvas: false,
        recordCrossOriginIframes: false,
        slimDOMOptions: "all",
        sampling: { mousemove: 100, scroll: 200, input: "last" },
        checkoutEveryNms: 60000,
        emit(event) {
          if (!active || !policy || blocked()) return;
          try {
            const safe = scrubReplay(event, policy, (id) =>
              record.mirror.getNode(id),
            );
            if (!safe) return;
            const bytes = JSON.stringify(safe).length;
            if (replayBytes + bytes > 1500000 || replay.length >= 200) {
              void flush();
              return;
            }
            replay.push(safe);
            replayBytes += bytes;
          } catch {
            /* overly large DOMs are not captured */
          }
        },
      });
    } catch {
      stopRecord = undefined;
    }
  }
  function start() {
    if (active || !policy?.enabled || optOut() || !consent.rum || blocked())
      return;
    let previous: Session | null = null;
    try {
      previous = JSON.parse(sessionStorage.getItem(key) || "null");
    } catch {}
    session =
      previous && Date.now() - previous.last < 1800000
        ? previous
        : {
            id: uid(),
            last: Date.now(),
            sampled: Math.random() < policy.sample_rate,
            replay: Math.random() < policy.replay_sample_rate,
          };
    if (!session.sampled) {
      try {
        sessionStorage.setItem(key, JSON.stringify(session));
      } catch {}
      return;
    }
    active = true;
    pageID = uid();
    aborter = new AbortController();
    timer = setInterval(() => void flush(), 5000);
    const documentPageID = pageID;
    const report = (m: Metric) =>
      emit("vital", { name: m.name, value: m.value, page_id: documentPageID });
    onLCP(report, { reportAllChanges: true });
    onINP(report, { reportAllChanges: true });
    onCLS(report, { reportAllChanges: true });
    onFCP(report);
    onTTFB(report);
    const page = () => {
      const n = performance.getEntriesByType("navigation")[0] as
        PerformanceNavigationTiming | undefined;
      emit("page_view", {
        duration_ms: Math.max(
          0,
          n?.loadEventEnd || n?.domContentLoadedEventEnd || performance.now(),
        ),
      });
    };
    if (document.readyState === "complete") page();
    else {
      window.addEventListener("load", page, { once: true });
      cleanups.push(() => window.removeEventListener("load", page));
    }
    const err = (e: ErrorEvent) =>
      emit("error", { name: e.error?.name || "Error", error: true });
    const rejection = (e: PromiseRejectionEvent) =>
      emit("error", {
        name: e.reason instanceof Error ? e.reason.name : "Error",
        error: true,
      });
    window.addEventListener("error", err);
    window.addEventListener("unhandledrejection", rejection);
    cleanups.push(() => {
      window.removeEventListener("error", err);
      window.removeEventListener("unhandledrejection", rejection);
    });
    const instrumented: typeof fetch = async (input, init) => {
      const raw = input instanceof Request ? input.url : String(input),
        when = performance.now();
      try {
        const response = await originalFetch(input, init);
        if (!raw.startsWith(base + "/rum/"))
          emit("request", {
            url: safeURL(raw, policy!.capture_path),
            status: response.status,
            duration_ms: performance.now() - when,
            error: response.status >= 400,
          });
        return response;
      } catch (error) {
        if (!raw.startsWith(base + "/rum/"))
          emit("request", {
            url: safeURL(raw, policy!.capture_path),
            duration_ms: performance.now() - when,
            error: true,
          });
        throw error;
      }
    };
    window.fetch = instrumented;
    cleanups.push(() => {
      if (window.fetch === instrumented) window.fetch = originalFetch;
    });
    const oldOpen = XMLHttpRequest.prototype.open,
      oldSend = XMLHttpRequest.prototype.send;
    const requests = new WeakMap<
      XMLHttpRequest,
      { url: string; start: number }
    >();
    const open = function (this: XMLHttpRequest, ...args: any[]) {
      requests.set(this, { url: String(args[1]), start: 0 });
      return (oldOpen as any).apply(this, args);
    };
    const send = function (this: XMLHttpRequest, ...args: any[]) {
      const meta = requests.get(this);
      if (meta) {
        meta.start = performance.now();
        this.addEventListener(
          "loadend",
          () =>
            emit("request", {
              url: safeURL(meta.url, policy!.capture_path),
              duration_ms: performance.now() - meta.start,
              status: this.status,
              error: this.status === 0 || this.status >= 400,
            }),
          { once: true },
        );
      }
      return oldSend.apply(
        this,
        args as [Document | XMLHttpRequestBodyInit | null],
      );
    };
    XMLHttpRequest.prototype.open = open as typeof oldOpen;
    XMLHttpRequest.prototype.send = send;
    cleanups.push(() => {
      if (XMLHttpRequest.prototype.open === open)
        XMLHttpRequest.prototype.open = oldOpen;
      if (XMLHttpRequest.prototype.send === send)
        XMLHttpRequest.prototype.send = oldSend;
    });
    function route() {
      if (blocked()) {
        stopRecord?.();
        stopRecord = undefined;
        pending = [];
        replay = [];
        replayBytes = 0;
        return;
      }
      void flush();
      replay = [];
      replayBytes = 0;
      pageID = uid();
      emit("navigation", { duration_ms: 0 });
      stopRecord?.();
      stopRecord = undefined;
      beginReplay();
    }
    for (const name of ["pushState", "replaceState"] as const) {
      const original = history[name];
      const wrapped = function (
        this: History,
        ...args: Parameters<typeof original>
      ) {
        const result = original.apply(this, args);
        route();
        return result;
      };
      history[name] = wrapped;
      cleanups.push(() => {
        if (history[name] === wrapped) history[name] = original;
      });
    }
    window.addEventListener("popstate", route);
    const hidden = () => {
      if (document.visibilityState === "hidden") void flush();
    };
    document.addEventListener("visibilitychange", hidden);
    window.addEventListener("pagehide", hidden);
    cleanups.push(() => {
      window.removeEventListener("popstate", route);
      document.removeEventListener("visibilitychange", hidden);
      window.removeEventListener("pagehide", hidden);
    });
    beginReplay();
  }
  function setConsent(choice: boolean | Consent) {
    consent =
      typeof choice === "boolean" ? { rum: choice, replay: false } : choice;
    try {
      sessionStorage.setItem(consentKey, JSON.stringify(consent));
    } catch {}
    banner?.remove();
    banner = null;
    if (!consent.rum) {
      stop();
      return;
    }
    start();
    if (consent.replay) beginReplay();
    else {
      stopRecord?.();
      stopRecord = undefined;
      replay = [];
      replayBytes = 0;
    }
  }
  function showConsent() {
    if (banner || optOut()) return;
    banner = document.createElement("section");
    banner.dataset["vetraBlock"] = "";
    banner.setAttribute("role", "dialog");
    banner.setAttribute("aria-label", "Privacidade do diagnóstico");
    banner.style.cssText =
      "position:fixed;bottom:16px;left:16px;max-width:420px;padding:20px;background:#101b15;color:#f1f7f4;border:1px solid #278b61;z-index:2147483647;font:14px sans-serif;box-shadow:0 4px 24px #0005";
    const text = document.createElement("p");
    text.textContent =
      "Permitir diagnóstico de desempenho? A Vetra coleta tempos, erros técnicos e sessões sem registrar valores de formulários.";
    banner.append(text);
    const replayChoice = document.createElement("input");
    replayChoice.type = "checkbox";
    if (policy?.replay_enabled) {
      const label = document.createElement("label");
      label.append(
        replayChoice,
        document.createTextNode(
          " Permitir também replay mascarado desta sessão",
        ),
      );
      banner.append(label);
    }
    for (const [label, accepted] of [
      ["Permitir", true],
      ["Agora não", false],
    ] as const) {
      const button = document.createElement("button");
      button.type = "button";
      button.textContent = label;
      button.style.cssText =
        "margin:12px 8px 0 0;padding:8px 12px;border:1px solid #40966a;background:#193c29;color:white;cursor:pointer";
      button.onclick = () =>
        setConsent({ rum: accepted, replay: accepted && replayChoice.checked });
      banner.append(button);
    }
    document.body.append(banner);
  }
  window.Vetra = {
    setConsent,
    showConsent,
    identify: async (id) => {
      if (
        !active ||
        !policy?.identify_users ||
        !crypto.subtle ||
        typeof id !== "string" ||
        !id ||
        id.length > 256
      )
        return;
      const hash = await crypto.subtle.digest(
        "SHA-256",
        new TextEncoder().encode(id),
      );
      if (active)
        userID = Array.from(new Uint8Array(hash), (b) =>
          b.toString(16).padStart(2, "0"),
        ).join("");
    },
    resetIdentity: () => {
      userID = "";
    },
    status: () => ({ active, replay: !!stopRecord, consent }),
    flush,
  };
  try {
    const response = await originalFetch(
      `${base}/rum/config/${encodeURIComponent(appID)}`,
      { mode: "cors", credentials: "omit" },
    );
    if (!response.ok) return;
    policy = await response.json();
    if (!policy?.enabled || optOut()) return;
    for (const selector of [
      ...policy.public_text_selectors,
      ...policy.block_selectors,
    ])
      document.querySelector(selector);
    let saved: Consent | null = null;
    try {
      saved = JSON.parse(sessionStorage.getItem(consentKey) || "null");
    } catch {}
    if (saved) setConsent(saved);
    else if (!policy.require_consent) setConsent({ rum: true, replay: false });
    else if (consent.rum) start();
    else if (policy.consent_banner && !blocked()) {
      if (document.body) showConsent();
      else
        window.addEventListener("DOMContentLoaded", showConsent, {
          once: true,
        });
    }
  } catch {
    /* Application behavior must not depend on monitoring availability. */
  }
}
