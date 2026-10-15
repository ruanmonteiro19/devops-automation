import { describe, expect, it } from "vitest";
import { filterTopology, layoutTopology } from "./topology";
import type { TopologyMap, MapNode, MapEdge } from "./api";
const node = (id: string, kind = "service"): MapNode => ({
  id,
  name: "Node " + id,
  kind,
  status: "unknown",
});
const edge = (from: string, to: string): MapEdge => ({
  id: from + to,
  from,
  to,
  kind: "observed",
  count: 1,
  evidence: "parent span",
  confidence: "observed",
});
const map: TopologyMap = {
  nodes: [node("a"), node("b", "database"), node("c"), node("isolated")],
  edges: [edge("a", "b"), edge("b", "c"), edge("missing", "a")],
};
describe("observed topology filtering", () => {
  it("filters by node name and id, dropping invisible edge endpoints", () => {
    const result = filterTopology(map, { search: "node b" });
    expect(result.nodes.map((n) => n.id)).toEqual(["b"]);
    expect(result.edges).toEqual([]);
    expect(filterTopology(map, { search: "isolated" }).nodes.length).toBe(1);
  });
  it("applies kind filter before hide-isolated", () => {
    expect(
      filterTopology(map, { kind: "database", hideIsolated: true }).nodes,
    ).toEqual([]);
    const result = filterTopology(map, { hideIsolated: true });
    expect(result.nodes.map((n) => n.id)).toEqual(["a", "b", "c"]);
    expect(result.edges.length).toBe(2);
  });
  it("bounds nodes to200 and returns exact truncation metadata and valid edges", () => {
    const large = {
      nodes: Array.from({ length: 230 }, (_, i) =>
        node(String(i).padStart(3, "0")),
      ),
      edges: [edge("000", "229"), edge("000", "001")],
    };
    const result = filterTopology(large);
    expect(result.nodes.length).toBe(200);
    expect(result.matchedNodes).toBe(230);
    expect(result.truncated).toBe(true);
    expect(result.edges.map((e) => e.id)).toEqual(["000001"]);
    expect(filterTopology(large, { search: "Node 22" }).truncated).toBe(false);
  });
});
describe("deterministic layered layout", () => {
  it("places DAG children after parents in each axis", () => {
    const horizontal = layoutTopology(map, "horizontal"),
      vertical = layoutTopology(map, "vertical");
    expect(horizontal.get("a")!.x).toBeLessThan(horizontal.get("b")!.x);
    expect(horizontal.get("b")!.x).toBeLessThan(horizontal.get("c")!.x);
    expect(vertical.get("a")!.y).toBeLessThan(vertical.get("b")!.y);
  });
  it("handles cycles and self-loops with finite nonoverlapping positions", () => {
    const cyclic = {
      nodes: [node("a"), node("b"), node("c")],
      edges: [edge("a", "b"), edge("b", "a"), edge("c", "c")],
    };
    for (const mode of ["horizontal", "vertical", "grid"] as const) {
      const values = [...layoutTopology(cyclic, mode).values()];
      expect(
        values.every((p) => Number.isFinite(p.x) && Number.isFinite(p.y)),
      ).toBe(true);
      expect(new Set(values.map((p) => JSON.stringify(p))).size).toBe(3);
    }
  });
  it("is invariant to input order and ignores dangling edges", () => {
    const first = layoutTopology(map, "horizontal");
    const reverse = layoutTopology(
      { nodes: [...map.nodes].reverse(), edges: [...map.edges].reverse() },
      "horizontal",
    );
    expect([...first]).toEqual([...reverse]);
    expect(layoutTopology({ nodes: [], edges: [] }, "vertical").size).toBe(0);
  });
});
