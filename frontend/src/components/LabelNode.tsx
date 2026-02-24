import type { NodeProps, Node } from "@xyflow/react";

type LabelNodeData = {
  label: string;
  color?: string;
  size?: "small" | "normal";
  centered?: boolean;
  rightAligned?: boolean;
};

type LabelNodeType = Node<LabelNodeData, "label">;

export function LabelNode({ data }: NodeProps<LabelNodeType>) {
  const isSmall = data.size === "small";
  return (
    <div
      style={{
        fontSize: isSmall ? 10 : 16,
        fontWeight: isSmall ? 600 : 700,
        textTransform: isSmall ? "uppercase" : undefined,
        letterSpacing: isSmall ? "0.06em" : undefined,
        color: data.color ?? "#999",
        whiteSpace: "nowrap",
        userSelect: "none",
        transform: data.centered ? "translateX(-50%)" : data.rightAligned ? "translateX(-100%)" : undefined,
      }}
    >
      {data.label}
    </div>
  );
}
