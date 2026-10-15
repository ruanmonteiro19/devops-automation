import { describe, expect, it } from "vitest";
import {
  filterTopology,
  layoutTopology,
  collapseDeviceInterfaces,
  topologyNeighborhood,
} from "./topology";
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
  it("bounds nodes to500 and returns exact truncation metadata and valid edges", () => {
    const large = {
      nodes: Array.from({ length: 530 }, (_, i) =>
        node(String(i).padStart(3, "0")),
      ),
      edges: [edge("000", "529"), edge("000", "001")],
    };
    const result = filterTopology(large);
    expect(result.nodes.length).toBe(500);
    expect(result.matchedNodes).toBe(530);
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

describe("Device port drill-down", () => {
  it("hides interfaces and duplicate collection wrappers while retaining their connections", () => {
    const graph: TopologyMap = {
      nodes: [
        { id: "collector", kind: "snmp", name: "Profile", status: "observed" },
        {
          id: "device",
          kind: "network.device",
          name: "Core",
          status: "observed",
        },
        { id: "host", kind: "host", name: "App", status: "observed" },
      ],
      edges: [
        {
          id: "wrapper",
          from: "collector",
          to: "device",
          kind: "contains",
          count: 1,
          evidence: "inventory",
          confidence: "observed",
        },
        {
          id: "link",
          from: "port",
          to: "host",
          kind: "lldp",
          count: 1,
          evidence: "LLDP",
          confidence: "advertised",
        },
      ],
      interface_inventory: {
        device: [
          {
            id: "port",
            kind: "network.interface",
            name: "Gi1",
            status: "observed",
          },
        ],
      },
    };
    const folded = collapseDeviceInterfaces(graph);
    expect(folded.graph.nodes.map((node) => node.id)).toEqual([
      "device",
      "host",
    ]);
    expect(folded.counts.get("device")).toBe(1);
    expect(folded.graph.edges.find((edge) => edge.kind === "lldp")?.from).toBe(
      "device",
    );
    const opened = collapseDeviceInterfaces(graph, ["device"]);
    expect(opened.graph.nodes.some((node) => node.id === "port")).toBe(true);
    expect(opened.graph.edges.find((edge) => edge.kind === "lldp")?.from).toBe(
      "port",
    );
    expect(graph.nodes).toHaveLength(3);
  });
  it("highlights only the selected neighborhood at the chosen depth", () => {
    const graph: TopologyMap = {
      nodes: ["device", "host", "apm", "other"].map((id) => ({
        id,
        name: id,
        kind: "host",
        status: "observed",
      })),
      edges: [
        {
          id: "dh",
          from: "device",
          to: "host",
          kind: "lldp",
          count: 1,
          evidence: "LLDP",
          confidence: "advertised",
        },
        {
          id: "ha",
          from: "host",
          to: "apm",
          kind: "runs-service",
          count: 1,
          evidence: "host.id",
          confidence: "resource-identity",
        },
      ],
    };
    expect([...topologyNeighborhood(graph, "device", 1).nodes]).toEqual([
      "device",
      "host",
    ]);
    expect(topologyNeighborhood(graph, "device", 2).nodes.has("apm")).toBe(
      true,
    );
    expect(topologyNeighborhood(graph, "device", 3).nodes.has("other")).toBe(
      false,
    );
  });
});
