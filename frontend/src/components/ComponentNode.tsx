import { Handle, Position, type NodeProps, type Node } from "@xyflow/react";
import type { Signal } from "../types/api";
import { componentType, componentQualifier } from "../types/api";

type Role = "receiver" | "processor" | "exporter";

type ComponentNodeData = {
  label: string;
  role: Role;
  signal: Signal;
  color: string;
  width?: number;
};

type ComponentNodeType = Node<ComponentNodeData, "component">;

const ROLE_ICONS: Record<Role, string> = {
  receiver: "\u2193",   // ↓
  processor: "\u2699",  // ⚙
  exporter: "\u2191",   // ↑
};

export function ComponentNode({ data }: NodeProps<ComponentNodeType>) {
  const type = componentType(data.label);
  const qualifier = componentQualifier(data.label);

  return (
    <div
      className="component-node"
      style={{
        borderColor: data.color,
        background: `${data.color}15`,
        ...(data.width ? { width: data.width, boxSizing: "border-box" as const } : {}),
      }}
    >
      <Handle type="target" position={Position.Left} id="left" />
      <Handle type="target" position={Position.Top} id="top" />
      <span className="component-node__icon">{ROLE_ICONS[data.role]}</span>
      <span>{type}</span>
      {qualifier && (
        <span className="component-node__qualifier">/{qualifier}</span>
      )}
      <Handle type="source" position={Position.Right} id="right" />
      <Handle type="source" position={Position.Bottom} id="bottom" />
    </div>
  );
}
