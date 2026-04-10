import { useCallback, useEffect, useRef, useState } from 'react';
import Konva from 'konva';
import type { KonvaEventObject } from 'konva/lib/Node';
import { Stage, Layer, Circle, Rect, Line, Text, Group, Ellipse } from 'react-konva';
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
  width?: number;
  height?: number;
  rotation_deg?: number;
};

type Wall = Record<string, number>;
export type Decoration = Record<string, unknown>;

type Props = {
  hallId: string;
  editMode: boolean;
  onTableSelect?: (t: TableShape | null) => void;
  selectedId?: string | null;
  availabilityById?: Record<string, boolean> | null;
  /** Увеличить при удалении стола снаружи, чтобы перезагрузить layout */
  reloadNonce?: number;
};

const colors: Record<string, string> = {
  available: '#22c55e',
  occupied: '#ef4444',
  blocked: '#64748b',
  locked_by_other: '#eab308',
  selected: '#6366f1',
};

const GRID = 20;
const DEFAULT_STAGE_W = 920;
const DEFAULT_STAGE_H = 640;

function isRectLike(shape: string) {
  const s = (shape || '').toLowerCase();
  return s === 'square' || s === 'rectangle' || s === 'rect';
}

function isEllipseLike(shape: string) {
  const s = (shape || '').toLowerCase();
  return s === 'ellipse' || s === 'oval';
}

function snap(v: number) {
  return Math.round(v / GRID) * GRID;
}

type PlaceMode = 'none' | 'door' | 'window' | 'zone';

export function HallCanvas({
  hallId,
  editMode,
  onTableSelect,
  selectedId,
  availabilityById,
  reloadNonce = 0,
}: Props) {
  const stageRef = useRef<Konva.Stage>(null);
  const [tables, setTables] = useState<TableShape[]>([]);
  const [walls, setWalls] = useState<Wall[]>([]);
  const [decorations, setDecorations] = useState<Decoration[]>([]);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [scale, setScale] = useState(1);
  const [showGrid, setShowGrid] = useState(true);
  const [placeMode, setPlaceMode] = useState<PlaceMode>('none');
  const [lineStart, setLineStart] = useState<{ x: number; y: number } | null>(null);
  const [zoneCorner, setZoneCorner] = useState<{ x: number; y: number } | null>(null);
  const [undoStack, setUndoStack] = useState<Array<{ walls: Wall[]; decorations: Decoration[] }>>([]);
  const [stageW, setStageW] = useState(DEFAULT_STAGE_W);
  const [stageH, setStageH] = useState(DEFAULT_STAGE_H);
  /** Локальный ввод размеров полотна (можно очистить поле без «0»). */
  const [stageWBuf, setStageWBuf] = useState<string | null>(null);
  const [stageHBuf, setStageHBuf] = useState<string | null>(null);

  useEffect(() => {
    setStageWBuf(null);
    setStageHBuf(null);
  }, [stageW, stageH]);

  const captureUndo = () => {
    setUndoStack((s) => [
      ...s.slice(-35),
      {
        walls: walls.map((w) => ({ ...w })),
        decorations: decorations.map((d) => ({ ...d })),
      },
    ]);
  };

  const doUndo = () => {
    setUndoStack((stack) => {
      if (stack.length === 0) return stack;
      const next = [...stack];
      const prev = next.pop();
      if (prev) {
        setWalls(prev.walls);
        setDecorations(prev.decorations);
      }
      return next;
    });
  };

  const load = useCallback(async () => {
    setLoadError(null);
    try {
      const { data } = await api.get<{
        tables: TableShape[] | null;
        walls: Wall[] | null;
        decorations?: Decoration[] | null;
        canvas_width?: number;
        canvas_height?: number;
      }>(`/halls/${hallId}/layout`);
      setTables(Array.isArray(data.tables) ? data.tables : []);
      setWalls(Array.isArray(data.walls) ? data.walls : []);
      setDecorations(Array.isArray(data.decorations) ? data.decorations : []);
      const cw = typeof data.canvas_width === 'number' && data.canvas_width > 0 ? data.canvas_width : DEFAULT_STAGE_W;
      const ch = typeof data.canvas_height === 'number' && data.canvas_height > 0 ? data.canvas_height : DEFAULT_STAGE_H;
      setStageW(Math.min(2000, Math.max(400, cw)));
      setStageH(Math.min(2000, Math.max(400, ch)));
    } catch (ex: unknown) {
      const ax = ex as { response?: { data?: { error?: string } }; message?: string };
      setLoadError(ax.response?.data?.error || ax.message || 'Не удалось загрузить схему зала');
      setTables([]);
      setWalls([]);
      setDecorations([]);
    }
  }, [hallId]);

  useEffect(() => {
    void load();
  }, [load, reloadNonce]);

  const onWs = useCallback(
    (msg: unknown) => {
      const m = msg as { event?: string };
      if (
        m.event === 'hall.layout_updated' ||
        m.event === 'table.booked' ||
        m.event === 'table.freed' ||
        m.event === 'table.locked' ||
        m.event === 'table.status_changed' ||
        m.event === 'reservation.status_changed' ||
        m.event === 'tab.paid'
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
      canvas_width: stageW,
      canvas_height: stageH,
      tables: tables.map((t) => {
        const tw = t.width && t.width > 0 ? t.width : (t.radius || 28) * 2;
        const th = t.height && t.height > 0 ? t.height : (t.radius || 28) * 2;
        return {
          id: t.id,
          number: t.number,
          capacity: t.capacity,
          x: t.x,
          y: t.y,
          shape: t.shape || 'circle',
          radius: t.radius || tw / 2,
          width: tw,
          height: th,
          rotation_deg: t.rotation_deg || 0,
        };
      }),
      walls,
      decorations,
    });
    await load();
  };

  const addTable = async (preset?: { capacity: number; shape: string; w: number; h: number }) => {
    const n = tables.length ? Math.max(...tables.map((t) => t.number)) + 1 : 1;
    const p = preset || { capacity: 4, shape: 'rect', w: 88, h: 64 };
    await api.post(`/halls/${hallId}/tables`, {
      table_number: n,
      capacity: p.capacity,
      x: 240,
      y: 280,
      shape: p.shape,
      width: p.w,
      height: p.h,
      rotation_deg: 0,
    });
    await load();
  };

  const addZoneLabel = () => {
    captureUndo();
    setDecorations((d) => [...d, { type: 'zone_label', text: 'Зона', x: 120, y: 80, w: 140, h: 28 }]);
  };

  const addPresetTerrace = () => {
    captureUndo();
    setDecorations((d) => [
      ...d,
      { type: 'zone', x: 400, y: 420, w: 200, h: 120, label: 'Терраса' },
    ]);
  };

  const handleFloorPointer = (e: KonvaEventObject<MouseEvent>) => {
    if (!editMode || placeMode === 'none') return;
    const st = e.target.getStage();
    if (!st) return;
    const p = st.getRelativePointerPosition();
    if (!p) return;
    const x = snap(p.x);
    const y = snap(p.y);
    if (placeMode === 'door' || placeMode === 'window') {
      if (!lineStart) {
        setLineStart({ x, y });
        return;
      }
      captureUndo();
      const kind = placeMode === 'door' ? 'door' : 'window';
      setDecorations((d) => [...d, { type: kind, x1: lineStart.x, y1: lineStart.y, x2: x, y2: y }]);
      setLineStart(null);
      setPlaceMode('none');
      return;
    }
    if (placeMode === 'zone') {
      if (!zoneCorner) {
        setZoneCorner({ x, y });
        return;
      }
      captureUndo();
      const x0 = Math.min(zoneCorner.x, x);
      const y0 = Math.min(zoneCorner.y, y);
      const ww = Math.abs(x - zoneCorner.x);
      const hh = Math.abs(y - zoneCorner.y);
      setDecorations((d) => [
        ...d,
        { type: 'zone', x: x0, y: y0, w: Math.max(40, ww), h: Math.max(40, hh), label: 'Зона' },
      ]);
      setZoneCorner(null);
      setPlaceMode('none');
    }
  };

  const gridLines: JSX.Element[] = [];
  if (showGrid && editMode) {
    for (let gx = 0; gx <= stageW; gx += GRID) {
      gridLines.push(
        <Line
          key={`gx-${gx}`}
          points={[gx, 0, gx, stageH]}
          stroke="#334155"
          strokeWidth={0.5}
          opacity={0.35}
          listening={false}
        />
      );
    }
    for (let gy = 0; gy <= stageH; gy += GRID) {
      gridLines.push(
        <Line
          key={`gy-${gy}`}
          points={[0, gy, stageW, gy]}
          stroke="#334155"
          strokeWidth={0.5}
          opacity={0.35}
          listening={false}
        />
      );
    }
  }

  return (
    <div className="hall-canvas-wrap">
      {loadError && <p className="form-msg">{loadError}</p>}
      {editMode && (
        <div className="hall-toolbar hall-toolbar-wrap">
          <div className="hall-toolbar-group">
            <span className="hall-toolbar-label">Столы</span>
            <button type="button" onClick={() => void addTable({ capacity: 2, shape: 'round', w: 52, h: 52 })}>
              2 места
            </button>
            <button type="button" onClick={() => void addTable({ capacity: 4, shape: 'rect', w: 88, h: 64 })}>
              4 места
            </button>
            <button type="button" onClick={() => void addTable({ capacity: 8, shape: 'rect', w: 120, h: 88 })}>
              8 мест
            </button>
            <button type="button" onClick={() => void addTable()}>
              + Стол (4)
            </button>
          </div>
          <div className="hall-toolbar-group">
            <span className="hall-toolbar-label">Декор (2× клик)</span>
          <button
            type="button"
            className={placeMode === 'door' ? 'btn' : 'secondary'}
            onClick={() => {
              setPlaceMode(placeMode === 'door' ? 'none' : 'door');
              setLineStart(null);
            }}
          >
            Дверь
          </button>
          <button
            type="button"
            className={placeMode === 'window' ? 'btn' : 'secondary'}
            onClick={() => {
              setPlaceMode(placeMode === 'window' ? 'none' : 'window');
              setLineStart(null);
            }}
          >
            Окно
          </button>
          <button
            type="button"
            className={placeMode === 'zone' ? 'btn' : 'secondary'}
            onClick={() => {
              setPlaceMode(placeMode === 'zone' ? 'none' : 'zone');
              setZoneCorner(null);
            }}
          >
            Зона (rect)
          </button>
          <button type="button" className="secondary" onClick={addZoneLabel}>
            + Подпись
          </button>
          <button type="button" className="secondary" onClick={addPresetTerrace}>
            Пресет «Терраса»
          </button>
          </div>
          <div className="hall-toolbar-group">
            <span className="hall-toolbar-label">Вид</span>
          <button type="button" className="secondary" onClick={() => setShowGrid((g) => !g)}>
            Сетка
          </button>
          <button type="button" className="secondary" onClick={() => setScale((s) => Math.min(2, Math.round((s + 0.1) * 10) / 10))}>
            +
          </button>
          <button type="button" className="secondary" onClick={() => setScale((s) => Math.max(0.5, Math.round((s - 0.1) * 10) / 10))}>
            −
          </button>
          <span className="muted hall-zoom-label">{Math.round(scale * 100)}%</span>
          <button type="button" className="secondary" onClick={doUndo} disabled={undoStack.length === 0}>
            Отмена
          </button>
          </div>
          <div className="hall-toolbar-group">
            <span className="hall-toolbar-label">Полотно px</span>
          <input
            type="text"
            inputMode="numeric"
            className="hall-size-input"
            value={stageWBuf !== null ? stageWBuf : String(Math.round(stageW))}
            onChange={(e) => {
              const v = e.target.value;
              if (v === '' || /^\d{1,4}$/.test(v)) setStageWBuf(v);
            }}
            onBlur={() => {
              const raw = stageWBuf;
              setStageWBuf(null);
              if (raw === null || raw.trim() === '') return;
              const n = parseInt(raw, 10);
              if (Number.isFinite(n)) setStageW(Math.min(2000, Math.max(400, n)));
            }}
            title="Ширина полотна"
          />
          <span className="muted">×</span>
          <input
            type="text"
            inputMode="numeric"
            className="hall-size-input"
            value={stageHBuf !== null ? stageHBuf : String(Math.round(stageH))}
            onChange={(e) => {
              const v = e.target.value;
              if (v === '' || /^\d{1,4}$/.test(v)) setStageHBuf(v);
            }}
            onBlur={() => {
              const raw = stageHBuf;
              setStageHBuf(null);
              if (raw === null || raw.trim() === '') return;
              const n = parseInt(raw, 10);
              if (Number.isFinite(n)) setStageH(Math.min(2000, Math.max(400, n)));
            }}
            title="Высота полотна"
          />
          <button
            type="button"
            className="secondary btn-sm"
            onClick={() => {
              setStageW(920);
              setStageH(640);
              setStageWBuf(null);
              setStageHBuf(null);
            }}
          >
            920×640
          </button>
          <button
            type="button"
            className="secondary btn-sm"
            onClick={() => {
              setStageW(1200);
              setStageH(800);
              setStageWBuf(null);
              setStageHBuf(null);
            }}
          >
            1200×800
          </button>
          <button
            type="button"
            className="secondary btn-sm"
            onClick={() => {
              setStageW(880);
              setStageH(600);
              setStageWBuf(null);
              setStageHBuf(null);
            }}
          >
            880×600
          </button>
          <button type="button" className="secondary" onClick={() => void saveLayout()}>
            Сохранить схему
          </button>
          </div>
        </div>
      )}
      {editMode && placeMode !== 'none' && (
        <p className="hint hall-place-hint">
          {placeMode === 'door' && (lineStart ? 'Второй клик — конец двери' : 'Первый клик — начало двери')}
          {placeMode === 'window' && (lineStart ? 'Второй клик — конец окна' : 'Первый клик — начало окна')}
          {placeMode === 'zone' && (zoneCorner ? 'Второй клик — противоположный угол зоны' : 'Первый клик — угол зоны')}
        </p>
      )}
      <Stage ref={stageRef} width={stageW} height={stageH} className="hall-stage" scaleX={scale} scaleY={scale}>
        <Layer>
          <Rect x={0} y={0} width={stageW} height={stageH} fill="#0f172a" cornerRadius={12} listening={false} />
          <Rect
            x={8}
            y={8}
            width={stageW - 16}
            height={stageH - 16}
            fill="#1e293b"
            stroke="#334155"
            strokeWidth={1}
            cornerRadius={8}
            listening={Boolean(editMode && placeMode !== 'none')}
            onMouseDown={(e) => handleFloorPointer(e)}
          />
          {gridLines}
          {decorations.map((dec, i) => {
            const t = String(dec.type || '');
            if (t === 'zone_label') {
              const x = Number(dec.x) || 0;
              const y = Number(dec.y) || 0;
              const w = Number(dec.w) || 120;
              const h = Number(dec.h) || 28;
              const text = String(dec.text || '');
              return (
                <Group key={`d-${i}`}>
                  <Rect x={x} y={y} width={w} height={h} fill="rgba(99,102,241,0.15)" cornerRadius={6} listening={false} />
                  <Text x={x + 8} y={y + 6} text={text} fontSize={13} fill="#a5b4fc" listening={false} />
                </Group>
              );
            }
            if (t === 'window_band') {
              const x = Number(dec.x) || 0;
              const y = Number(dec.y) || 0;
              const ww = Number(dec.w) || stageW;
              const hh = Number(dec.h) || 24;
              return (
                <Rect
                  key={`d-${i}`}
                  x={x}
                  y={y}
                  width={ww}
                  height={hh}
                  fill="rgba(56,189,248,0.25)"
                  listening={false}
                />
              );
            }
            if (t === 'door') {
              const x1 = Number(dec.x1);
              const y1 = Number(dec.y1);
              const x2 = Number(dec.x2);
              const y2 = Number(dec.y2);
              return (
                <Line
                  key={`d-${i}`}
                  points={[x1, y1, x2, y2]}
                  stroke="#a16207"
                  strokeWidth={8}
                  lineCap="round"
                  listening={false}
                />
              );
            }
            if (t === 'window') {
              const x1 = Number(dec.x1);
              const y1 = Number(dec.y1);
              const x2 = Number(dec.x2);
              const y2 = Number(dec.y2);
              return (
                <Line
                  key={`d-${i}`}
                  points={[x1, y1, x2, y2]}
                  stroke="#38bdf8"
                  strokeWidth={5}
                  lineCap="round"
                  listening={false}
                />
              );
            }
            if (t === 'zone') {
              const x = Number(dec.x) || 0;
              const y = Number(dec.y) || 0;
              const w = Number(dec.w) || 80;
              const h = Number(dec.h) || 80;
              const lab = String(dec.label || 'Зона');
              return (
                <Group key={`d-${i}`}>
                  <Rect
                    x={x}
                    y={y}
                    width={w}
                    height={h}
                    fill="rgba(52,211,153,0.12)"
                    stroke="rgba(52,211,153,0.5)"
                    strokeWidth={2}
                    listening={false}
                  />
                  <Text x={x + 8} y={y + 8} text={lab} fontSize={13} fill="#6ee7b7" listening={false} />
                </Group>
              );
            }
            return null;
          })}
          {walls.map((wall, i) => (
            <Line
              key={i}
              points={[wall.x1, wall.y1, wall.x2, wall.y2]}
              stroke="#475569"
              strokeWidth={4}
              lineCap="round"
            />
          ))}
          {tables.map((t) => {
            const rw = t.width && t.width > 0 ? t.width / 2 : t.radius || 28;
            const rh = t.height && t.height > 0 ? t.height / 2 : t.radius || 28;
            const r = t.radius || Math.max(rw, rh);
            const fill = colors[t.status] || colors.available;
            const stroke = selectedId === t.id ? '#e2e8f0' : '#334155';
            const strokeW = selectedId === t.id ? 3 : 1.5;
            const slotOk = availabilityById == null ? true : availabilityById[t.id] === true;
            const dim = availabilityById != null && !slotOk && !editMode;
            const opacity = dim ? 0.35 : 1;
            const canClick =
              editMode ||
              (slotOk && (t.status === 'available' || t.status === 'selected' || t.status === 'locked_by_other'));
            const rot = ((t.rotation_deg || 0) * Math.PI) / 180;

            return (
              <Group
                key={t.id}
                x={t.x}
                y={t.y}
                rotation={rot}
                opacity={opacity}
                draggable={editMode && placeMode === 'none'}
                listening={editMode && placeMode !== 'none' ? false : true}
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
                {isRectLike(t.shape) ? (
                  <Rect
                    x={-rw}
                    y={-rh}
                    width={rw * 2}
                    height={rh * 2}
                    cornerRadius={8}
                    fill={fill}
                    stroke={stroke}
                    strokeWidth={strokeW}
                    shadowBlur={8}
                    shadowColor="rgba(0,0,0,0.35)"
                  />
                ) : isEllipseLike(t.shape) ? (
                  <Ellipse
                    radiusX={rw}
                    radiusY={rh}
                    fill={fill}
                    stroke={stroke}
                    strokeWidth={strokeW}
                    shadowBlur={8}
                    shadowColor="rgba(0,0,0,0.35)"
                  />
                ) : (
                  <Circle
                    radius={r}
                    fill={fill}
                    stroke={stroke}
                    strokeWidth={strokeW}
                    shadowBlur={8}
                    shadowColor="rgba(0,0,0,0.35)"
                  />
                )}
                <Text
                  text={String(t.number)}
                  fontSize={14}
                  fontStyle="bold"
                  fill="#f8fafc"
                  x={-8}
                  y={-10}
                  listening={false}
                />
                <Text
                  text={`до ${t.capacity}`}
                  fontSize={10}
                  fill="rgba(248,250,252,0.9)"
                  x={-22}
                  y={isRectLike(t.shape) || isEllipseLike(t.shape) ? rh - 6 : r - 4}
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
