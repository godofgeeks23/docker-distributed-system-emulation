import ELK from "elkjs/lib/elk.bundled.js";
import type { Edge, Node } from "@xyflow/react";
import type { TopologyView } from "./api";

const elk = new ELK();

const nodeSizes: Record<string, { width: number; height: number }> = {
  backbone: { width: 220, height: 72 },
  region: { width: 280, height: 220 },
  router: { width: 180, height: 64 },
  probe: { width: 180, height: 64 },
};

export async function layoutTopology(topology: TopologyView): Promise<{
  nodes: Node[];
  edges: Edge[];
}> {
  const parentNodes = topology.nodes.filter((node) => node.kind === "region");
  const childNodes = topology.nodes.filter((node) => node.parent_id);
  const rootNodes = topology.nodes.filter((node) => !node.parent_id && node.kind !== "region");

  const graph = {
    id: "root",
    layoutOptions: {
      "elk.algorithm": "layered",
      "elk.direction": "RIGHT",
      "elk.padding": "[top=32,left=32,bottom=32,right=32]",
      "elk.spacing.nodeNode": "40",
      "elk.layered.spacing.nodeNodeBetweenLayers": "60",
    },
    children: [
      ...parentNodes.map((region, index) => ({
        id: region.id,
        width: nodeSizes.region.width,
        height: nodeSizes.region.height,
        layoutOptions: {
          "elk.algorithm": "layered",
          "elk.direction": "DOWN",
          "elk.padding": "[top=44,left=16,bottom=16,right=16]",
          "elk.spacing.nodeNode": "28",
        },
        children: childNodes
          .filter((node) => node.parent_id === region.id)
          .map((node) => ({
            id: node.id,
            width: sizeFor(node.kind).width,
            height: sizeFor(node.kind).height,
          })),
      })),
      ...rootNodes.map((node) => ({
        id: node.id,
        width: sizeFor(node.kind).width,
        height: sizeFor(node.kind).height,
      })),
    ],
    edges: topology.edges.map((edge) => ({
      id: edge.id,
      sources: [edge.source],
      targets: [edge.target],
    })),
  };

  const layout = await elk.layout(graph);
  const positions = new Map<string, { x: number; y: number; parentId?: string }>();

  for (const child of layout.children ?? []) {
    positions.set(child.id, { x: child.x ?? 0, y: child.y ?? 0 });
    for (const nested of child.children ?? []) {
      positions.set(nested.id, {
        x: nested.x ?? 0,
        y: nested.y ?? 0,
        parentId: child.id,
      });
    }
  }

  const nodes: Node[] = topology.nodes.map((node) => {
    const position = positions.get(node.id) ?? { x: 0, y: 0, parentId: node.parent_id };
    return {
      id: node.id,
      type: node.kind === "region" ? "region" : "service",
      position: { x: position.x, y: position.y },
      parentId: position.parentId,
      extent: position.parentId ? "parent" : undefined,
      draggable: false,
      selectable: true,
      data: node,
      style: sizeFor(node.kind),
    };
  });

  const edges: Edge[] = topology.edges.map((edge) => ({
    id: edge.id,
    source: edge.source,
    target: edge.target,
    label: edge.label,
    type: edge.kind === "wan-link" ? "smoothstep" : "straight",
    animated: edge.kind === "wan-link",
    style: {
      strokeWidth: edge.highlighted ? 3 : 2,
    },
    data: edge,
  }));

  return { nodes, edges };
}

function sizeFor(kind: string) {
  return nodeSizes[kind] ?? { width: 180, height: 64 };
}
