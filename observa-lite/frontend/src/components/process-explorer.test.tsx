// @vitest-environment jsdom
import { afterEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { ProcessExplorer } from "./process-explorer";
import { createDemoSnapshot } from "@/lib/demo-data";
import { type ProcessInfo } from "@/lib/api";
import { isSystemProcess } from "@/lib/processes";
vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, params, ...rest }: any) => (
    <a {...rest} href={`/${params._splat}`}>
      {children}
    </a>
  ),
}));
const process = (
  name: string,
  pid: number,
  runtime = "native",
  rss = 1024,
): ProcessInfo => ({ name, pid, runtime, service: name, rss_bytes: rss });
function mount() {
  const snapshot = createDemoSnapshot();
  snapshot.hosts = [
    {
      id: "host-a",
      name: "PROD-A",
      os: "Linux",
      addresses: [],
      last_seen: new Date().toISOString(),
      processes: [
        process("systemd", 1),
        process("java", 20, "java", 5000000),
        process("custom-payments", 30, "native", 1000000),
        process("kworker/0:1", 40),
      ],
      connections: [
        {
          pid: 20,
          local_ip: "10.0.0.1",
          local_port: 8080,
          remote_ip: "10.0.0.2",
          remote_port: 443,
          state: "ESTABLISHED",
        },
      ],
    },
    {
      id: "host-b",
      name: "PROD-B",
      os: "Windows",
      addresses: [],
      last_seen: new Date().toISOString(),
      processes: [
        process("svchost.exe", 50),
        process("node", 1, "nodejs", 6000000),
      ],
      connections: [],
    },
  ];
  render(<ProcessExplorer snapshot={snapshot} />);
}
afterEach(cleanup);
describe("Application process inventory", () => {
  it("hides OS components while retaining custom applications and PID-1 runtimes", () => {
    mount();
    expect(screen.queryByRole("button", { name: "systemd" })).toBeNull();
    expect(screen.queryByRole("button", { name: "svchost.exe" })).toBeNull();
    expect(screen.getByRole("button", { name: "java" })).toBeTruthy();
    expect(
      screen.getByRole("button", { name: "custom-payments" }),
    ).toBeTruthy();
    expect(screen.getByRole("button", { name: "node" })).toBeTruthy();
    fireEvent.click(screen.getByLabelText("MOSTRAR PROCESSOS DE SISTEMA"));
    expect(screen.getByRole("button", { name: "systemd" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "svchost.exe" })).toBeTruthy();
  });
  it("filters hosts and runtimes independently", () => {
    mount();
    fireEvent.change(screen.getByLabelText("FILTRAR HOST"), {
      target: { value: "host-a" },
    });
    expect(screen.queryByRole("button", { name: "node" })).toBeNull();
    fireEvent.change(screen.getByLabelText("FILTRAR RUNTIME"), {
      target: { value: "java" },
    });
    expect(screen.getByRole("button", { name: "java" })).toBeTruthy();
    expect(
      screen.queryByRole("button", { name: "custom-payments" }),
    ).toBeNull();
  });
  it("shows real PID connections without advertising unavailable instrumentation", () => {
    mount();
    fireEvent.click(screen.getByRole("button", { name: "DETALHES java 20" }));
    expect(screen.getByText("10.0.0.1:8080 → 10.0.0.2:443")).toBeTruthy();
    expect(
      (
        screen.getByRole("switch", {
          name: "DEEP MONITORING",
        }) as HTMLInputElement
      ).disabled,
    ).toBe(true);
    expect(
      screen
        .getByRole("link", { name: "CONFIGURAR INSTRUMENTAÇÃO" })
        .getAttribute("href"),
    ).toBe("/ajuda/otlp");
  });
  it("does not hide arbitrary names resembling OS components", () => {
    expect(isSystemProcess(process("systemd-payments", 7))).toBe(false);
    expect(isSystemProcess(process("sshd-api", 8))).toBe(false);
    expect(isSystemProcess(process("my-java-api", 1, "java"))).toBe(false);
    expect(isSystemProcess(process("kworker/2:1", 8))).toBe(true);
  });
});
