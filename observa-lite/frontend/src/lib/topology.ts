import type { TopologyMap } from "./api";
export type TopologyLayout = "grid" | "horizontal" | "vertical" | "radial";
export type TopologyOptions = {
  search?: string;
  kind?: string;
  hideIsolated?: boolean;
  limit?: number;
};
export function filterTopology(
  map: TopologyMap,
  options: TopologyOptions = {},
) {
  const search = (options.search ?? "").trim().toLowerCase();
  let nodes = map.nodes
    .filter(
      (node) =>
        (!options.kind || node.kind === options.kind) &&
        (!search || (node.name + " " + node.id).toLowerCase().includes(search)),
    )
    .sort((a, b) => a.id.localeCompare(b.id));
  let ids = new Set(nodes.map((node) => node.id));
  let edges = map.edges.filter(
    (edge) => ids.has(edge.from) && ids.has(edge.to),
  );
  if (options.hideIsolated) {
    const connected = new Set(edges.flatMap((edge) => [edge.from, edge.to]));
    nodes = nodes.filter((node) => connected.has(node.id));
  }
  const matchedNodes = nodes.length;
  const limit = Math.max(1, Math.min(500, Math.floor(options.limit ?? 500)));
  nodes = nodes.slice(0, limit);
  ids = new Set(nodes.map((node) => node.id));
  edges = edges
    .filter((edge) => ids.has(edge.from) && ids.has(edge.to))
    .sort((a, b) => a.id.localeCompare(b.id));
  return {
    nodes,
    edges,
    matchedNodes,
    truncated: matchedNodes > nodes.length,
    limit,
  };
}
export function layoutTopology(
  map: TopologyMap,
  layout: TopologyLayout,
): Map<string, { x: number; y: number }> {
  const sorted = [...map.nodes].sort((a, b) => a.id.localeCompare(b.id));
  const positions = new Map<string, { x: number; y: number }>();
  if (layout === "radial") {
    const degree = new Map(sorted.map((n) => [n.id, 0]));
    for (const e of map.edges) {
      degree.set(e.from, (degree.get(e.from) ?? 0) + 1);
      degree.set(e.to, (degree.get(e.to) ?? 0) + 1);
    }
    const radial = [...sorted].sort(
      (a, b) =>
        (degree.get(b.id) ?? 0) - (degree.get(a.id) ?? 0) ||
        a.id.localeCompare(b.id),
    );
    radial.forEach((node, index) => {
      const angle = index * Math.PI * (3 - Math.sqrt(5));
      const radius = index === 0 ? 0 : 180 + Math.sqrt(index) * 65;
      positions.set(node.id, {
        x: 900 + Math.cos(angle) * radius,
        y: 900 + Math.sin(angle) * radius,
      });
    });
    return positions;
  }
  if (layout === "grid") {
    const columns = Math.max(1, Math.ceil(Math.sqrt(sorted.length)));
    sorted.forEach((node, i) =>
      positions.set(node.id, {
        x: 60 + (i % columns) * 280,
        y: 60 + Math.floor(i / columns) * 150,
      }),
    );
    return positions;
  }
  const ids = new Set(sorted.map((node) => node.id)),
    indegree = new Map(sorted.map((node) => [node.id, 0])),
    out = new Map<string, string[]>(),
    levels = new Map(sorted.map((node) => [node.id, 0]));
  const seenEdges = new Set<string>();
  for (const edge of map.edges) {
    const key = JSON.stringify([edge.from, edge.to]);
    if (!ids.has(edge.from) || !ids.has(edge.to) || seenEdges.has(key))
      continue;
    seenEdges.add(key);
    indegree.set(edge.to, (indegree.get(edge.to) ?? 0) + 1);
    out.set(edge.from, [...(out.get(edge.from) ?? []), edge.to]);
  }
  const ready = sorted
    .filter((node) => indegree.get(node.id) === 0)
    .map((node) => node.id);
  const visited = new Set<string>();
  while (ready.length) {
    ready.sort();
    const id = ready.shift()!;
    visited.add(id);
    for (const target of out.get(id) ?? []) {
      levels.set(
        target,
        Math.max(levels.get(target) ?? 0, (levels.get(id) ?? 0) + 1),
      );
      indegree.set(target, (indegree.get(target) ?? 0) - 1);
      if (indegree.get(target) === 0) ready.push(target);
    }
  }
  // Cycles and nodes downstream of cycles share a final bounded layer.
  const fallback = visited.size
    ? Math.max(...[...visited].map((id) => levels.get(id) ?? 0)) + 1
    : 0;
  for (const node of sorted)
    if (!visited.has(node.id)) levels.set(node.id, fallback);
  const lanes = new Map<number, number>();
  for (const node of sorted) {
    const layer = levels.get(node.id) ?? 0,
      lane = lanes.get(layer) ?? 0;
    lanes.set(layer, lane + 1);
    positions.set(
      node.id,
      layout === "horizontal"
        ? { x: 60 + layer * 320, y: 60 + lane * 140 }
        : { x: 60 + lane * 280, y: 60 + layer * 180 },
    );
  }
  return positions;
}

/** Collapse device interfaces while preserving their observed external relationships. */
export function collapseDeviceInterfaces(
  map: TopologyMap,
  expanded: string[] = [],
) {
  const core = [...map.nodes];
  const relations = [...map.edges];
  const existing = new Set(core.map((node) => node.id));
  const contains = new Set(
    relations
      .filter((edge) => edge.kind === "contains")
      .map((edge) => JSON.stringify([edge.from, edge.to])),
  );
  for (const [parent, items] of Object.entries(map.interface_inventory ?? {})) {
    for (const item of items) {
      if (!existing.has(item.id)) {
        core.push(item);
        existing.add(item.id);
      }
      const key = JSON.stringify([parent, item.id]);
      if (!contains.has(key)) {
        relations.push({
          id: "interface-inventory:" + item.id,
          from: parent,
          to: item.id,
          kind: "contains",
          count: 1,
          evidence: "SNMP interface inventory",
          confidence: "observed",
        });
        contains.add(key);
      }
    }
  }
  map = { ...map, nodes: core, edges: relations };
  const byID = new Map(map.nodes.map((node) => [node.id, node]));
  const wrappers = new Map<string, string>();
  for (const node of map.nodes) {
    if (!["snmp", "snmp_exporter"].includes(node.kind)) continue;
    const children = map.edges.filter(
      (edge) =>
        edge.from === node.id &&
        edge.kind === "contains" &&
        byID.get(edge.to)?.kind === "network.device",
    );
    if (children.length === 1) wrappers.set(node.id, children[0]!.to);
  }
  const parents = new Map<string, string>();
  const counts = new Map<string, number>();
  for (const edge of map.edges) {
    if (
      edge.kind !== "contains" ||
      byID.get(edge.to)?.kind !== "network.interface"
    )
      continue;
    parents.set(edge.to, edge.from);
    counts.set(edge.from, (counts.get(edge.from) || 0) + 1);
  }
  const hidden = new Set(
    [...parents]
      .filter(([, parent]) => !expanded.includes(parent))
      .map(([id]) => id),
  );
  const project = (id: string) =>
    wrappers.get(id) ?? (hidden.has(id) ? parents.get(id)! : id);
  const edges = new Map<string, TopologyMap["edges"][number]>();
  for (const edge of map.edges) {
    const from = project(edge.from),
      to = project(edge.to);
    if (from === to) continue;
    const id = JSON.stringify([from, to, edge.kind]);
    const prior = edges.get(id);
    if (prior) prior.count += edge.count;
    else edges.set(id, { ...edge, id, from, to });
  }
  return {
    graph: {
      nodes: map.nodes.filter(
        (node) => !hidden.has(node.id) && !wrappers.has(node.id),
      ),
      edges: [...edges.values()],
    },
    counts,
    parents,
  };
}
export function topologyNeighborhood(map: TopologyMap, id: string, depth = 1) {
  const nodes = new Set<string>();
  const edges = new Set<string>();
  const kinds = new Map(map.nodes.map((node) => [node.id, node.kind]));
  if (!kinds.has(id)) return { nodes, edges };
  const maximum = Math.max(1, Math.min(3, depth));
  const distances = new Map([[id, 0]]);
  const queue: [string, number][] = [[id, 0]];
  const weight = (edge: TopologyMap["edges"][number]) =>
    edge.kind === "contains" &&
    (kinds.get(edge.from) === "network.interface" ||
      kinds.get(edge.to) === "network.interface")
      ? 0
      : 1;
  for (let index = 0; index < queue.length; index++) {
    const [current, distance] = queue[index]!;
    if (distances.get(current) !== distance) continue;
    for (const edge of map.edges) {
      if (edge.from !== current && edge.to !== current) continue;
      const next = edge.from === current ? edge.to : edge.from;
      const nextDistance = distance + weight(edge);
      if (
        nextDistance <= maximum &&
        nextDistance < (distances.get(next) ?? Infinity)
      ) {
        distances.set(next, nextDistance);
        queue.push([next, nextDistance]);
      }
    }
  }
  for (const node of distances.keys()) nodes.add(node);
  for (const edge of map.edges) {
    if (
      nodes.has(edge.from) &&
      nodes.has(edge.to) &&
      Math.min(distances.get(edge.from)!, distances.get(edge.to)!) +
        weight(edge) <=
        maximum
    )
      edges.add(edge.id);
  }
  return { nodes, edges };
}
