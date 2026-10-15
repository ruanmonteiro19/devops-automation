// @vitest-environment jsdom
import { afterEach, describe, it, expect, vi } from "vitest";
import { render, screen, cleanup } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { groupTopology, TopologyWorkspace } from "./topology-workspace";
import { createDemoSnapshot } from "@/lib/demo-data";
import { type TopologyMap } from "@/lib/api";
import { layoutTopology } from "@/lib/topology";
vi.mock("@tanstack/react-router", () => ({
  Link: ({ children, params, ...rest }: any) => (
    <a {...rest} href={`/${params._splat}`}>
      {children}
    </a>
  ),
  useNavigate: () => vi.fn(),
}));
afterEach(cleanup);
const graph: TopologyMap = {
  nodes: [
    { id: "a", name: "API", kind: "service", status: "unknown" },
    { id: "b", name: "Worker", kind: "service", status: "unknown" },
    { id: "c", name: "DB", kind: "database", status: "unknown" },
  ],
  edges: [
    {
      id: "ab",
      from: "a",
      to: "b",
      kind: "span-parent",
      count: 2,
      evidence: "trace",
      confidence: "",
    },
    {
      id: "bc",
      from: "b",
      to: "c",
      kind: "db",
      count: 3,
      evidence: "trace",
      confidence: "",
    },
  ],
};
describe("Topology views", () => {
  it("groups observed nodes without inventing traffic and expands exact members", () => {
    const collapsed = groupTopology(graph, "kind", []);
    expect(collapsed.graph.nodes).toHaveLength(2);
    expect(collapsed.graph.edges).toHaveLength(1);
    expect(collapsed.graph.edges[0]!.count).toBe(3);
    expect(collapsed.graph.edges[0]!.kind).toBe("db");
    const expanded = groupTopology(graph, "kind", ["group:kind:service"]);
    expect(expanded.graph.nodes.map((n) => n.id)).toContain("a");
    expect(
      expanded.graph.edges.some((e) => e.from === "a" && e.to === "b"),
    ).toBe(true);
    expect(graph.edges[0]!.count).toBe(2);
  });
  it("exposes all ten views and connector destinations", () => {
    render(
      <QueryClientProvider client={new QueryClient()}>
        <TopologyWorkspace
          path="topologia"
          demo={true}
          snapshot={createDemoSnapshot()}
          renderMap={() => null}
        />
      </QueryClientProvider>,
    );
    expect(
      screen
        .getByRole("link", { name: /VISÃO UNIFICADA/ })
        .getAttribute("href"),
    ).toBe("/topologia/all");
    expect(
      screen.getByRole("link", { name: /KUBERNETES/ }).getAttribute("href"),
    ).toBe("/topologia/kubernetes");
    expect(screen.getByRole("link", { name: /GOOGLE CLOUD/ })).toBeTruthy();
    expect(screen.getByRole("link", { name: /NETWORK DEVICES/ })).toBeTruthy();
  });
  it("produces stable finite radial positions without changing relationships", () => {
    const first = layoutTopology(graph, "radial");
    const reversed = layoutTopology(
      { ...graph, nodes: [...graph.nodes].reverse() },
      "radial",
    );
    expect([...first]).toEqual([...reversed]);
    for (const position of first.values()) {
      expect(Number.isFinite(position.x)).toBe(true);
      expect(Number.isFinite(position.y)).toBe(true);
    }
    expect(new Set([...first.values()].map((p) => `${p.x}:${p.y}`)).size).toBe(
      3,
    );
  });
});
