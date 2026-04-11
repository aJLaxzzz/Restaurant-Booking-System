import { memo } from 'react';
import { Line, Shape, Circle } from 'react-konva';

type Wall = Record<string, number>;
type WallArc = { type: 'arc'; cx: number; cy: number; r: number; a0: number; a1: number };
export type WallQuad = { type: 'quad'; x1: number; y1: number; qx: number; qy: number; x2: number; y2: number };

type Props = {
  walls: Wall[];
  wallArcs: WallArc[];
  wallQuads: WallQuad[];
  selectedWall: number | null;
  selectedArc: number | null;
  selectedQuad: number | null;
  wallPickActive: boolean;
  editMode: boolean;
  snap: (v: number) => number;
  onPickWall: (i: number) => void;
  onPickArc: (i: number) => void;
  onPickQuad: (i: number) => void;
  onWallEndpointDrag: (wallIndex: number, end: 'a' | 'b', x: number, y: number) => void;
  onQuadControlDrag: (quadIndex: number, qx: number, qy: number) => void;
};

export const HallCanvasWallsLayer = memo(function HallCanvasWallsLayer({
  walls,
  wallArcs,
  wallQuads,
  selectedWall,
  selectedArc,
  selectedQuad,
  wallPickActive,
  editMode,
  snap,
  onPickWall,
  onPickArc,
  onPickQuad,
  onWallEndpointDrag,
  onQuadControlDrag,
}: Props) {
  return (
    <>
      {walls.map((wall, i) => (
        <Line
          key={`w-${i}`}
          points={[wall.x1, wall.y1, wall.x2, wall.y2]}
          stroke={selectedWall === i ? '#94a3b8' : '#475569'}
          strokeWidth={4}
          lineCap="round"
          hitStrokeWidth={40}
          perfectDrawEnabled={false}
          listening={wallPickActive}
          onTap={(e) => {
            e.cancelBubble = true;
            onPickWall(i);
          }}
          onClick={(e) => {
            e.cancelBubble = true;
            onPickWall(i);
          }}
        />
      ))}
      {wallQuads.map((q, i) => (
        <Shape
          key={`wq-${i}`}
          stroke={selectedQuad === i ? '#94a3b8' : '#475569'}
          strokeWidth={4}
          hitStrokeWidth={40}
          perfectDrawEnabled={false}
          sceneFunc={(context, shape) => {
            context.beginPath();
            context.moveTo(q.x1, q.y1);
            context.quadraticCurveTo(q.qx, q.qy, q.x2, q.y2);
            context.strokeShape(shape);
          }}
          listening={wallPickActive}
          onTap={(e) => {
            e.cancelBubble = true;
            onPickQuad(i);
          }}
          onClick={(e) => {
            e.cancelBubble = true;
            onPickQuad(i);
          }}
        />
      ))}
      {wallArcs.map((arc, i) => (
        <Shape
          key={`wa-${i}`}
          stroke={selectedArc === i ? '#94a3b8' : '#475569'}
          strokeWidth={4}
          hitStrokeWidth={40}
          perfectDrawEnabled={false}
          sceneFunc={(context, shape) => {
            context.beginPath();
            context.arc(arc.cx, arc.cy, arc.r, arc.a0, arc.a1, false);
            context.strokeShape(shape);
          }}
          listening={wallPickActive}
          onTap={(e) => {
            e.cancelBubble = true;
            onPickArc(i);
          }}
          onClick={(e) => {
            e.cancelBubble = true;
            onPickArc(i);
          }}
        />
      ))}
      {selectedWall !== null && editMode && wallPickActive && walls[selectedWall] && (
        <>
          <Circle
            x={walls[selectedWall].x1}
            y={walls[selectedWall].y1}
            radius={11}
            fill="#f8fafc"
            stroke="#475569"
            draggable
            hitStrokeWidth={26}
            perfectDrawEnabled={false}
            onDragEnd={(e) => {
              onWallEndpointDrag(selectedWall, 'a', snap(e.target.x()), snap(e.target.y()));
            }}
          />
          <Circle
            x={walls[selectedWall].x2}
            y={walls[selectedWall].y2}
            radius={11}
            fill="#f8fafc"
            stroke="#475569"
            draggable
            hitStrokeWidth={26}
            perfectDrawEnabled={false}
            onDragEnd={(e) => {
              onWallEndpointDrag(selectedWall, 'b', snap(e.target.x()), snap(e.target.y()));
            }}
          />
        </>
      )}
      {selectedQuad !== null &&
        editMode &&
        wallPickActive &&
        wallQuads[selectedQuad] &&
        (() => {
          const q = wallQuads[selectedQuad];
          return (
            <Circle
              x={q.qx}
              y={q.qy}
              radius={11}
              fill="#fef08a"
              stroke="#475569"
              draggable
              hitStrokeWidth={26}
              perfectDrawEnabled={false}
              onDragEnd={(e) => {
                onQuadControlDrag(selectedQuad, snap(e.target.x()), snap(e.target.y()));
              }}
            />
          );
        })()}
    </>
  );
});
