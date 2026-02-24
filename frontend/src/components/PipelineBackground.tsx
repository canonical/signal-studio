import type { NodeProps, Node } from "@xyflow/react";

type PipelineBackgroundData = {
  width: number;
  height: number;
  color: string;
};

type PipelineBackgroundType = Node<PipelineBackgroundData, "pipelineBackground">;

export function PipelineBackground({ data }: NodeProps<PipelineBackgroundType>) {
  return (
    <div
      className="pipeline-background"
      style={{
        width: data.width,
        height: data.height,
        borderColor: data.color,
        backgroundColor: `${data.color}08`,
      }}
    />
  );
}
