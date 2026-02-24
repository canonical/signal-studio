import { useMemo } from "react";
import {
  ReactFlow,
  type Node,
  type Edge,
  Position,
  Background,
  BackgroundVariant,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import type { CollectorConfig, Signal } from "../types/api";
import { ComponentNode } from "./ComponentNode";
import { LabelNode } from "./LabelNode";
import { PipelineBackground } from "./PipelineBackground";

interface PipelineGraphProps {
  config: CollectorConfig;
}

const SIGNAL_COLORS: Record<Signal, string> = {
  traces: "#a855f7",
  metrics: "#22c55e",
  logs: "#f59e0b",
};

const NODE_TYPES = {
  component: ComponentNode,
  label: LabelNode,
  pipelineBackground: PipelineBackground,
};

const COL_GAP = 200;
const ROW_GAP = 64;
const PIPELINE_GAP = 60;

type Role = "receiver" | "processor" | "exporter";

const COLUMN_LABELS: Role[] = ["receiver", "processor", "exporter"];

export function PipelineGraph({ config }: PipelineGraphProps) {
  const { nodes, edges } = useMemo(() => buildGraph(config), [config]);

  return (
    <div className="pipeline-canvas">
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={NODE_TYPES}
        fitView
        fitViewOptions={{ padding: 0.15 }}
        proOptions={{ hideAttribution: true }}
        nodesDraggable={false}
        nodesConnectable={false}
        elementsSelectable={false}
        panOnDrag
        zoomOnScroll
        minZoom={0.3}
        maxZoom={2}
        colorMode="light"
      >
        <Background variant={BackgroundVariant.Dots} gap={16} size={1} />
      </ReactFlow>
    </div>
  );
}

function buildGraph(config: CollectorConfig): {
  nodes: Node[];
  edges: Edge[];
} {
  const nodes: Node[] = [];
  const edges: Edge[] = [];
  const pipelines = Object.entries(config.pipelines);

  // Compute a uniform node width from the longest label across all pipelines.
  // Uses a monospace font at 13px ≈ 7.8px per character, plus padding/icon overhead.
  const globalMaxChars = pipelines.reduce((max, [, p]) => {
    for (const list of [
      p.receivers ?? [],
      p.processors ?? [],
      p.exporters ?? [],
    ]) {
      for (const name of list) {
        if (name.length > max) max = name.length;
      }
    }
    return max;
  }, 0);
  const globalNodeWidth = Math.max(
    120,
    Math.ceil(20 + 14 + globalMaxChars * 7.8),
  );

  const colX = [0, COL_GAP, COL_GAP + globalNodeWidth + COL_GAP];

  let yOffset = 0;

  for (const [pipelineName, pipeline] of pipelines) {
    const signal = pipeline.signal;
    const color = SIGNAL_COLORS[signal] ?? "#6b7280";

    // Pipeline name label
    nodes.push({
      id: `title-${pipelineName}`,
      type: "label",
      position: { x: 0, y: yOffset },
      data: { label: capitalize(pipelineName), color, size: "normal" },
      selectable: false,
    });

    const titleOffset = 28;

    const columns: { role: Role; items: string[] }[] = [
      { role: "receiver", items: pipeline.receivers ?? [] },
      { role: "processor", items: pipeline.processors ?? [] },
      { role: "exporter", items: pipeline.exporters ?? [] },
    ];

    const procs = pipeline.processors ?? [];

    const headerOffset = titleOffset + 36;

    // Pipeline box dimensions (needed to center processor label and right-align exporters)
    const PAD = 32;
    const bgWidth = colX[2]! + globalNodeWidth + PAD * 2;
    const boxCenterX = -PAD + bgWidth / 2;
    const exporterX = bgWidth - PAD * 2 - globalNodeWidth;

    const maxRows = Math.max(
      ...columns.map((c) => Math.max(c.items.length, 1)),
    );
    const NODE_H = 28;

    for (let col = 0; col < columns.length; col++) {
      const { role, items } = columns[col]!;

      // Column header positioning
      const headerX =
        role === "processor"
          ? boxCenterX
          : role === "exporter"
            ? exporterX + globalNodeWidth
            : colX[col]!;
      nodes.push({
        id: `header-${pipelineName}-${COLUMN_LABELS[col]!}`,
        type: "label",
        position: { x: headerX, y: yOffset + titleOffset },
        data: {
          label: COLUMN_LABELS[col]! + "s",
          size: "small",
          centered: role === "processor",
          rightAligned: role === "exporter",
        },
        selectable: false,
      });

      for (let row = 0; row < items.length; row++) {
        const name = items[row]!;
        const nodeId = `${pipelineName}::${role}::${name}`;

        nodes.push({
          id: nodeId,
          type: "component",
          position: {
            x:
              role === "processor"
                ? boxCenterX - globalNodeWidth / 2
                : role === "exporter"
                  ? exporterX
                  : colX[col]!,
            y: yOffset + headerOffset + row * ROW_GAP,
          },
          sourcePosition: Position.Right,
          targetPosition: Position.Left,
          data: { label: name, role, signal, color, width: globalNodeWidth },
        });
      }
    }

    // Pipeline background
    const contentBottom = headerOffset + (maxRows - 1) * ROW_GAP + NODE_H;
    const bgHeight = PAD + contentBottom + PAD;
    nodes.push({
      id: `bg-${pipelineName}`,
      type: "pipelineBackground",
      position: { x: -PAD, y: yOffset - PAD },
      data: { width: bgWidth, height: bgHeight, color },
      selectable: false,
      draggable: false,
      style: { zIndex: -1 },
    });

    // Edges: sequential processor chain
    const edgeStyle = { stroke: color, strokeWidth: 1.5, opacity: 0.5 };

    if (procs.length === 0) {
      // No processors: receivers connect directly to exporters
      for (const src of pipeline.receivers ?? []) {
        for (const tgt of pipeline.exporters ?? []) {
          const srcId = `${pipelineName}::receiver::${src}`;
          const tgtId = `${pipelineName}::exporter::${tgt}`;
          edges.push({
            id: `${srcId}->${tgtId}`,
            source: srcId,
            sourceHandle: "right",
            target: tgtId,
            targetHandle: "left",
            animated: true,
            style: edgeStyle,
          });
        }
      }
    } else {
      // Receivers → first processor (left → right)
      for (const src of pipeline.receivers ?? []) {
        const srcId = `${pipelineName}::receiver::${src}`;
        const tgtId = `${pipelineName}::processor::${procs[0]!}`;
        edges.push({
          id: `${srcId}->${tgtId}`,
          source: srcId,
          sourceHandle: "right",
          target: tgtId,
          targetHandle: "left",
          animated: true,
          style: edgeStyle,
        });
      }

      // Processor chain: proc[i] → proc[i+1] (straight vertical)
      for (let i = 0; i < procs.length - 1; i++) {
        const srcId = `${pipelineName}::processor::${procs[i]!}`;
        const tgtId = `${pipelineName}::processor::${procs[i + 1]!}`;
        edges.push({
          id: `${srcId}->${tgtId}`,
          source: srcId,
          sourceHandle: "bottom",
          target: tgtId,
          targetHandle: "top",
          type: "straight",
          animated: true,
          style: edgeStyle,
        });
      }

      // Last processor → exporters (left → right)
      for (const tgt of pipeline.exporters ?? []) {
        const srcId = `${pipelineName}::processor::${procs[procs.length - 1]!}`;
        const tgtId = `${pipelineName}::exporter::${tgt}`;
        edges.push({
          id: `${srcId}->${tgtId}`,
          source: srcId,
          sourceHandle: "right",
          target: tgtId,
          targetHandle: "left",
          animated: true,
          style: edgeStyle,
        });
      }
    }

    yOffset += headerOffset + maxRows * ROW_GAP + PIPELINE_GAP;
  }

  return { nodes, edges };
}

/** Capitalize each segment of a pipeline name (e.g. "traces/custom" → "Traces/Custom"). */
function capitalize(name: string): string {
  return name.replace(/\b\w/g, (c) => c.toUpperCase());
}
