import { useCallback, useEffect, useState } from 'react';
import { Stage, Layer, Circle, Rect, Line, Text, Group } from 'react-konva';
import { api } from '../api';
import { useHallWebSocket } from '../hooks/useHallWebSocket';

export type TableShape = {
  id: string;
  number: number;
  capacity: number;
  x: number;
  y: number;
  shape: string;
  status: string;
  radius?: number;
};

type Wall = Record<string, number>;

type Props = {
  hallId: string;
  editMode: boolean;
  onTableSelect?: (t: TableShape | null) => void;
  selectedId?: string | null;
  /** Если задано — столы с false визуально отключены (шаг выбора слота). */
  availabilityById?: Record<string, boolean> | null;
};

const colors: Record<string, string> = {
  available: '#2d8f47',
  occupied: '#c43b2c',
  blocked: '#888888',
  locked_by_other: '#d4a017',
  selected: '#2563eb',
};

function isRectShape(shape: string) {
  return shape === 'square' || shape === 'rectangle' || shape === 'rect';
}

export function HallCanvas({ hallId, editMode, onTableSelect, selectedId, availabilityById }: Props) {
  const [tables, setTables] = useState<TableShape[]>([]);
  const [walls, setWalls] = useState<Wall[]>([]);

  const load = useCallback(async () => {
    const { data } = await api.get<{ tables: TableShape[]; walls: Wall[] }>(`/halls/${hallId}/layout`);
    setTables(data.tables);
    setWalls(data.walls || []);
  }, [hallId]);

  useEffect(() => {
    void load();
  }, [load]);

  const onWs = useCallback(
    (msg: unknown) => {
      const m = msg as { event?: string };
      if (
        m.event === 'hall.layout_updated' ||
        m.event === 'table.booked' ||
        m.event === 'table.freed' ||
        m.event === 'table.locked' ||
        m.event === 'table.status_changed' ||
        m.event === 'reservation.status_changed'
      ) {
        void load();
      }
    },
    [load]
  );

  useHallWebSocket(hallId, onWs);

  const handleDragEnd = (id: string, x: number, y: number) => {
    setTables((prev) => prev.map((t) => (t.id === id ? { ...t, x, y } : t)));
  };

  const saveLayout = async () => {
    await api.put(`/halls/${hallId}/layout`, {
      tables: tables.map((t) => ({
        id: t.id,
        number: t.number,
        capacity: t.capacity,
        x: t.x,
        y: t.y,
        shape: t.shape || 'circle',
        radius: t.radius || 28,
      })),
      walls,
    });
    await load();
  };

  const addTable = async () => {
    const n = tables.length ? Math.max(...tables.map((t) => t.number)) + 1 : 1;
    await api.post(`/halls/${hallId}/tables`, {
      table_number: n,
      capacity: 4,
      x: 200,
      y: 200,
      shape: 'circle',
    });
    await load();
  };

  const w = 920;
  const h = 640;

  return (
    <div className="hall-canvas-wrap">
      {editMode && (
        <div className="hall-toolbar">
          <button type="button" onClick={() => void addTable()}>
            + Стол
          </button>
          <button type="button" className="secondary" onClick={() => void saveLayout()}>
            Сохранить позиции
          </button>
        </div>
      )}
      <Stage width={w} height={h} className="hall-stage">
        <Layer>
          {walls.map((wall, i) => (
            <Line
              key={i}
              points={[wall.x1, wall.y1, wall.x2, wall.y2]}
              stroke="#3d3428"
              strokeWidth={5}
              lineCap="round"
              shadowBlur={2}
              shadowColor="rgba(0,0,0,0.15)"
            />
          ))}
          {tables.map((t) => {
            const r = t.radius || 28;
            const fill = colors[t.status] || colors.available;
            const stroke = selectedId === t.id ? '#0f172a' : '#1e293b';
            const strokeW = selectedId === t.id ? 4 : 1.5;
            const slotOk = availabilityById == null ? true : availabilityById[t.id] === true;
            const dim = availabilityById != null && !slotOk && !editMode;
            const opacity = dim ? 0.38 : 1;
            const canClick =
              editMode ||
              (slotOk && (t.status === 'available' || t.status === 'selected' || t.status === 'locked_by_other'));

            return (
              <Group
                key={t.id}
                x={t.x}
                y={t.y}
                opacity={opacity}
                draggable={editMode}
                onDragEnd={(e) => {
                  const node = e.target;
                  handleDragEnd(t.id, node.x(), node.y());
                }}
                onTap={() => {
                  if (!canClick && !editMode) return;
                  if (editMode) {
                    onTableSelect?.(t);
                    return;
                  }
                  if (dim) return;
                  if (t.status === 'blocked') return;
                  if (t.status === 'available' || t.status === 'selected' || t.status === 'locked_by_other') {
                    onTableSelect?.(t);
                  }
                }}
                onClick={() => {
                  if (!canClick && !editMode) return;
                  if (editMode) {
                    onTableSelect?.(t);
                    return;
                  }
                  if (dim) return;
                  if (t.status === 'blocked') return;
                  if (t.status === 'available' || t.status === 'selected' || t.status === 'locked_by_other') {
                    onTableSelect?.(t);
                  }
                }}
              >
                {isRectShape(t.shape) ? (
                  <Rect
                    x={-r}
                    y={-r}
                    width={r * 2}
                    height={r * 2}
                    cornerRadius={6}
                    fill={fill}
                    stroke={stroke}
                    strokeWidth={strokeW}
                  />
                ) : (
                  <Circle radius={r} fill={fill} stroke={stroke} strokeWidth={strokeW} />
                )}
                <Text text={String(t.number)} fontSize={14} fontStyle="bold" fill="#fff" x={-6} y={-8} listening={false} />
                <Text
                  text={`до ${t.capacity}`}
                  fontSize={10}
                  fill="rgba(255,255,255,0.92)"
                  x={-18}
                  y={r - 4}
                  listening={false}
                />
              </Group>
            );
          })}
        </Layer>
      </Stage>
    </div>
  );
}
