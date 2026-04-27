import { useEffect, useState } from "react";
import {
  Background,
  Controls,
  MiniMap,
  ReactFlow,
  type Edge,
  type Node,
  type NodeProps,
} from "@xyflow/react";
import type { TopologyView, TopologyNode } from "../lib/api";
import { layoutTopology } from "../lib/layout";

type Props = {
  topology?: TopologyView;
  onSelect(node?: TopologyNode): void;
};

export function GraphCanvas({ topology, onSelect }: Props) {
  const [nodes, setNodes] = useState<Node[]>([]);
  const [edges, setEdges] = useState<Edge[]>([]);

  useEffect(() => {
    let cancelled = false;
    if (!topology) {
      setNodes([]);
      setEdges([]);
      return;
    }
    void layoutTopology(topology).then((layout) => {
      if (cancelled) {
        return;
      }
      setNodes(layout.nodes);
      setEdges(layout.edges);
    });
    return () => {
      cancelled = true;
    };
  }, [topology]);

  return (
    <div className="graph-panel">
      <ReactFlow
        fitView
        nodes={nodes}
        edges={edges}
        nodeTypes={{ region: RegionNode, service: ServiceNode }}
        onNodeClick={(_, node) => onSelect(node.data as TopologyNode)}
      >
        <MiniMap pannable zoomable />
        <Background gap={24} />
        <Controls />
      </ReactFlow>
    </div>
  );
}

function RegionNode({ data }: NodeProps<Node<{ label: string; subnet?: string }>>) {
  return (
    <div className="region-node">
      <div className="region-node__title">{data.label}</div>
      <div className="region-node__meta">{data.subnet}</div>
    </div>
  );
}

function ServiceNode({ data }: NodeProps<Node<TopologyNode>>) {
  return (
    <div className={`service-node service-node--${data.kind} ${data.highlighted ? "is-highlighted" : ""}`}>
      <div className="service-node__kind">{data.kind}</div>
      <div className="service-node__title">{data.label}</div>
      {data.ip ? <div className="service-node__meta">{data.ip}</div> : null}
    </div>
  );
}
