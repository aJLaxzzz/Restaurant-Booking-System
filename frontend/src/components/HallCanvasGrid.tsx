import { memo, useCallback } from 'react';
import { Shape } from 'react-konva';
import type Konva from 'konva';

type Props = {
  show: boolean;
  step: number;
  minX: number;
  maxX: number;
  minY: number;
  maxY: number;
};

export const HallCanvasGrid = memo(function HallCanvasGrid({ show, step, minX, maxX, minY, maxY }: Props) {
  const sceneFunc = useCallback(
    (context: Konva.Context, shape: Konva.Shape) => {
      if (!show || step <= 0) return;
      const x0 = Math.floor(minX / step) * step;
      const x1 = Math.ceil(maxX / step) * step;
      const y0 = Math.floor(minY / step) * step;
      const y1 = Math.ceil(maxY / step) * step;
      context.beginPath();
      for (let gx = x0; gx <= x1; gx += step) {
        context.moveTo(gx, minY);
        context.lineTo(gx, maxY);
      }
      for (let gy = y0; gy <= y1; gy += step) {
        context.moveTo(minX, gy);
        context.lineTo(maxX, gy);
      }
      context.strokeShape(shape);
    },
    [show, step, minX, maxX, minY, maxY],
  );

  if (!show) return null;
  return (
    <Shape
      sceneFunc={sceneFunc as (ctx: Konva.Context, shape: Konva.Shape) => void}
      stroke="rgba(51,65,85,0.38)"
      strokeWidth={0.5}
      listening={false}
      perfectDrawEnabled={false}
      hitStrokeWidth={0}
    />
  );
});
