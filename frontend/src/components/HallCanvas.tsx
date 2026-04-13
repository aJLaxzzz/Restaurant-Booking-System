import { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react';
import Konva from 'konva';
import type { KonvaEventObject } from 'konva/lib/Node';
import { Stage, Layer, Circle, Rect, Line, Text, Group, Ellipse, Transformer, Shape } from 'react-konva';
import { api } from '../api';
import { useHallWebSocket } from '../hooks/useHallWebSocket';
import { HallAddTableModal, type AddTableForm } from './HallAddTableModal';
import { HallCanvasGrid } from './HallCanvasGrid';
import { HallZoneFillModal } from './HallZoneFillModal';
import { pointInPolygon, polygonArea, reversePolygonRingFlat, zoneNamedDirectHolePolys } from '../utils/geometry';
import { floodRegionPolygonFromWalls } from '../utils/hallRegionFromWalls';
import { HallCanvasWallsLayer, type WallQuad } from './HallCanvasWallsLayer';

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

type WallArc = { type: 'arc'; cx: number; cy: number; r: number; a0: number; a1: number };
type HallRoom = { polygon: number[]; kind?: string; label?: string };
type ChairPt = { dx: number; dy: number };

/** Буфер Ctrl+C / Ctrl+V: геометрия стола и смещения стульев относительно центра. */
type TableClip = {
  capacity: number;
  x: number;
  y: number;
  shape: string;
  width: number;
  height: number;
  rotation_deg: number;
  chairs: ChairPt[];
};

type RoomInset = { x: number; y: number; w: number; h: number };

type Props = {
  hallId: string;
  editMode: boolean;
  onTableSelect?: (t: TableShape | null) => void;
  selectedId?: string | null;
  availabilityById?: Record<string, boolean> | null;
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

const FIXTURE_LABELS: Record<string, string> = {
  wc: 'Санузел',
  cloakroom: 'Гардероб',
  kitchen_open: 'Кухня (открытая)',
  kitchen_closed: 'Кухня',
  bar: 'Барная стойка',
};

function defaultRoomInset(sw: number, sh: number): RoomInset {
  return { x: 0, y: 0, w: sw, h: sh };
}

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

/** Ключ отрезка без направления — для слияния `walls` и линий из `wall_segments` без дубликатов. */
function wallLineKey(w: Wall): string {
  const x1 = Number(w.x1);
  const y1 = Number(w.y1);
  const x2 = Number(w.x2);
  const y2 = Number(w.y2);
  const r = (n: number) => Math.round(n * 100) / 100;
  const p = `${r(x1)},${r(y1)},${r(x2)},${r(y2)}`;
  const q = `${r(x2)},${r(y2)},${r(x1)},${r(y1)}`;
  return p <= q ? p : q;
}

function mergeWallLinesDeduped(a: Wall[], b: Wall[]): Wall[] {
  const map = new Map<string, Wall>();
  for (const w of a) {
    const k = wallLineKey(w);
    if (!map.has(k)) map.set(k, { ...w });
  }
  for (const w of b) {
    const k = wallLineKey(w);
    if (!map.has(k)) map.set(k, { ...w });
  }
  return [...map.values()];
}

type Path2DCtx = { moveTo(x: number, y: number): void; lineTo(x: number, y: number): void; closePath(): void };

function traceClosedPoly(ctx: Path2DCtx, poly: number[]) {
  ctx.moveTo(poly[0], poly[1]);
  for (let k = 2; k < poly.length; k += 2) ctx.lineTo(poly[k], poly[k + 1]);
  ctx.closePath();
}

function personsLabelRu(n: number) {
  const k = n % 10;
  const kk = n % 100;
  if (kk >= 11 && kk <= 14) return `${n} персон`;
  if (k === 1) return `${n} персона`;
  if (k >= 2 && k <= 4) return `${n} персоны`;
  return `${n} персон`;
}

function defaultChairOffsets(capacity: number, halfW: number, halfH: number, shape: string): ChairPt[] {
  const n = Math.max(1, Math.min(24, capacity));
  const base = Math.max(halfW, halfH) + 16;
  const out: ChairPt[] = [];
  const isCircle = !isRectLike(shape) && !isEllipseLike(shape);
  const start = isCircle ? -Math.PI / 2 : Math.PI;
  const sweep = isCircle ? Math.PI * 2 : Math.PI;
  for (let i = 0; i < n; i++) {
    const t = start + (sweep * i) / n;
    out.push({ dx: Math.cos(t) * base, dy: Math.sin(t) * base });
  }
  return out;
}

function isInputLikeTarget(el: EventTarget | null) {
  if (!el || !(el instanceof HTMLElement)) return false;
  const tag = el.tagName;
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || el.isContentEditable;
}

type EditorMode = 'view' | 'walls' | 'interior';

type PlaceMode = 'none' | 'door' | 'window' | 'wallseg' | 'wallquad' | 'zone_fill';

type UndoSnap = {
  walls: Wall[];
  wallArcs: WallArc[];
  wallQuads: WallQuad[];
  rooms: HallRoom[];
  decorations: Decoration[];
  chairLayout: Record<string, ChairPt[]>;
  /** Снимок столов; отсутствует в старых записях стека до этого изменения */
  tables?: TableShape[];
};

export function HallCanvas({
  hallId,
  editMode,
  onTableSelect,
  selectedId,
  availabilityById,
  reloadNonce = 0,
}: Props) {
  const stageRef = useRef<Konva.Stage>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const tableTransformerRef = useRef<Konva.Transformer>(null);
  const tableShapeRef = useRef<Konva.Rect | Konva.Circle | Konva.Ellipse | null>(null);
  const scaleRef = useRef(1);
  const panRef = useRef({ x: 0, y: 0 });
  const spacePanRef = useRef(false);
  const loadDebounceRef = useRef<number | null>(null);

  const [tables, setTables] = useState<TableShape[]>([]);
  const [walls, setWalls] = useState<Wall[]>([]);
  const [wallArcs, setWallArcs] = useState<WallArc[]>([]);
  const [rooms, setRooms] = useState<HallRoom[]>([]);
  const [chairLayout, setChairLayout] = useState<Record<string, ChairPt[]>>({});
  const [decorations, setDecorations] = useState<Decoration[]>([]);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [scale, setScale] = useState(1);
  const [worldPan, setWorldPan] = useState({ x: 0, y: 0 });
  const [viewportW, setViewportW] = useState(920);
  const [viewportH, setViewportH] = useState(560);
  const [showGrid, setShowGrid] = useState(true);
  const [editorMode, setEditorMode] = useState<EditorMode>('interior');
  const [placeMode, setPlaceMode] = useState<PlaceMode>('none');
  const [lineStart, setLineStart] = useState<{ x: number; y: number } | null>(null);
  const [wallQuads, setWallQuads] = useState<WallQuad[]>([]);
  const [wallQuadDraft, setWallQuadDraft] = useState<{ x1: number; y1: number; x2: number; y2: number } | null>(null);
  const [zoneFillModalOpen, setZoneFillModalOpen] = useState(false);
  const [zoneFillPoints, setZoneFillPoints] = useState<number[]>([]);
  const [zoneFillLabelPos, setZoneFillLabelPos] = useState({ x: 0, y: 0 });
  const [, setUndoStack] = useState<UndoSnap[]>([]);
  const [stageW, setStageW] = useState(DEFAULT_STAGE_W);
  const [stageH, setStageH] = useState(DEFAULT_STAGE_H);
  const [stageWBuf, setStageWBuf] = useState<string | null>(null);
  const [stageHBuf, setStageHBuf] = useState<string | null>(null);
  const roomInset = useMemo(() => defaultRoomInset(stageW, stageH), [stageW, stageH]);
  const [selectedDec, setSelectedDec] = useState<number | null>(null);
  const [selectedWall, setSelectedWall] = useState<number | null>(null);
  const [selectedArc, setSelectedArc] = useState<number | null>(null);
  const [selectedQuad, setSelectedQuad] = useState<number | null>(null);
  const [panDrag, setPanDrag] = useState<{ px: number; py: number; ox: number; oy: number } | null>(null);
  const [addTableOpen, setAddTableOpen] = useState(false);
  const touchPinchRef = useRef<{ dist: number; scale: number; cx: number; cy: number; px: number; py: number } | null>(
    null,
  );
  const tablesRef = useRef<TableShape[]>([]);
  const deleteSelectedRef = useRef<() => void>(() => {});
  const selectionKeyRef = useRef({
    selectedDec: null as number | null,
    selectedWall: null as number | null,
    selectedArc: null as number | null,
    selectedQuad: null as number | null,
  });
  const chairLayoutRef = useRef<Record<string, ChairPt[]>>({});
  const editorKeyRef = useRef({
    editMode: false,
    editorMode: 'interior' as EditorMode,
    placeMode: 'none' as PlaceMode,
    selectedId: null as string | null,
    zoneFillModalOpen: false,
    addTableOpen: false,
  });
  const tableClipRef = useRef<TableClip | null>(null);
  /** После POST нового стола load() поднимает useEffect со стульями — сюда кладём раскладку из буфера вставки, чтобы не затёрлась defaultChairOffsets. */
  const pastePendingChairsRef = useRef<Map<string, ChairPt[]>>(new Map());

  useEffect(() => {
    scaleRef.current = scale;
  }, [scale]);
  useEffect(() => {
    panRef.current = worldPan;
  }, [worldPan]);
  tablesRef.current = tables;
  chairLayoutRef.current = chairLayout;

  useLayoutEffect(() => {
    const el = containerRef.current;
    if (!el) return;
    const ro = new ResizeObserver(() => {
      const r = el.getBoundingClientRect();
      const maxVH =
        typeof window !== 'undefined' ? Math.min(720, Math.max(360, Math.floor(window.innerHeight * 0.72))) : 720;
      setViewportW(Math.max(320, Math.floor(r.width)));
      setViewportH(Math.max(360, Math.min(maxVH, Math.floor(r.height))));
    });
    ro.observe(el);
    const r = el.getBoundingClientRect();
    const maxVH0 =
      typeof window !== 'undefined' ? Math.min(720, Math.max(360, Math.floor(window.innerHeight * 0.72))) : 720;
    setViewportW(Math.max(320, Math.floor(r.width)));
    setViewportH(Math.max(360, Math.min(maxVH0, Math.floor(r.height))));
    return () => ro.disconnect();
  }, []);

  useEffect(() => {
    setStageWBuf(null);
    setStageHBuf(null);
  }, [stageW, stageH]);

  const selectedTableSnap = useMemo(
    () => tables.find((t) => t.id === selectedId),
    [tables, selectedId],
  );

  useLayoutEffect(() => {
    if (
      !editMode ||
      editorMode !== 'interior' ||
      placeMode !== 'none' ||
      !selectedId ||
      !tableTransformerRef.current
    ) {
      tableTransformerRef.current?.nodes([]);
      return;
    }
    const node = tableShapeRef.current;
    if (node) {
      tableTransformerRef.current.nodes([node]);
      tableTransformerRef.current.getLayer()?.batchDraw();
    } else {
      tableTransformerRef.current.nodes([]);
    }
  }, [
    editMode,
    editorMode,
    placeMode,
    selectedId,
    selectedTableSnap?.width,
    selectedTableSnap?.height,
    selectedTableSnap?.radius,
    selectedTableSnap?.rotation_deg,
    selectedTableSnap?.shape,
  ]);

  useEffect(() => {
    if (!panDrag) return;
    const onMove = (e: MouseEvent | TouchEvent) => {
      let cx: number;
      let cy: number;
      if ('touches' in e && e.touches.length > 0) {
        cx = e.touches[0].clientX;
        cy = e.touches[0].clientY;
        e.preventDefault();
      } else if ('clientX' in e) {
        cx = e.clientX;
        cy = e.clientY;
      } else {
        return;
      }
      setWorldPan({ x: cx - panDrag.px + panDrag.ox, y: cy - panDrag.py + panDrag.oy });
    };
    const onUp = () => setPanDrag(null);
    window.addEventListener('mousemove', onMove);
    window.addEventListener('mouseup', onUp);
    window.addEventListener('touchmove', onMove, { passive: false });
    window.addEventListener('touchend', onUp);
    return () => {
      window.removeEventListener('mousemove', onMove);
      window.removeEventListener('mouseup', onUp);
      window.removeEventListener('touchmove', onMove);
      window.removeEventListener('touchend', onUp);
    };
  }, [panDrag]);

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.code !== 'Space') return;
      if (isInputLikeTarget(e.target)) return;
      e.preventDefault();
      spacePanRef.current = true;
    };
    const onKeyUp = (e: KeyboardEvent) => {
      if (e.code === 'Space') spacePanRef.current = false;
    };
    window.addEventListener('keydown', onKeyDown, { capture: true });
    window.addEventListener('keyup', onKeyUp);
    return () => {
      window.removeEventListener('keydown', onKeyDown, { capture: true });
      window.removeEventListener('keyup', onKeyUp);
    };
  }, []);

  const gridWorldBounds = useMemo(() => {
    const sc = Math.max(0.001, scale);
    const px = worldPan.x;
    const py = worldPan.y;
    const vx0 = (0 - px) / sc;
    const vy0 = (0 - py) / sc;
    const vx1 = (viewportW - px) / sc;
    const vy1 = (viewportH - py) / sc;
    const pad = GRID * 3;
    const minX = Math.max(0, Math.min(vx0, vx1) - pad);
    const maxX = Math.min(stageW, Math.max(vx0, vx1) + pad);
    const minY = Math.max(0, Math.min(vy0, vy1) - pad);
    const maxY = Math.min(stageH, Math.max(vy0, vy1) + pad);
    return { minX, maxX, minY, maxY };
  }, [scale, worldPan.x, worldPan.y, viewportW, viewportH, stageW, stageH]);

  const pointerToWorld = useCallback(
    (e: KonvaEventObject<unknown>) => {
      const st = e.target.getStage();
      if (!st) return null;
      const p = st.getPointerPosition();
      if (!p) return null;
      const pan = panRef.current;
      const sc = scaleRef.current;
      return { x: (p.x - pan.x) / sc, y: (p.y - pan.y) / sc };
    },
    [],
  );

  const captureUndo = () => {
    setUndoStack((s) => [
      ...s.slice(-35),
      {
        walls: walls.map((w) => ({ ...w })),
        wallArcs: wallArcs.map((a) => ({ ...a })),
        wallQuads: wallQuads.map((q) => ({ ...q })),
        rooms: rooms.map((r) => ({ ...r, polygon: [...r.polygon] })),
        decorations: decorations.map((d) => ({ ...d })),
        chairLayout: Object.fromEntries(Object.entries(chairLayout).map(([k, v]) => [k, v.map((c) => ({ ...c }))])),
        tables: tables.map((t) => ({ ...t })),
      },
    ]);
  };

  const doUndo = useCallback(() => {
    setUndoStack((stack) => {
      if (stack.length === 0) return stack;
      const next = [...stack];
      const prev = next.pop();
      if (prev) {
        if (prev.tables) {
          const cur = tablesRef.current;
          const keep = new Set(prev.tables.map((t) => t.id));
          for (const t of cur) {
            if (!keep.has(t.id)) void api.delete(`/halls/${hallId}/tables/${t.id}`);
          }
          setTables(prev.tables.map((t) => ({ ...t })));
          if (selectedId && !prev.tables.some((t) => t.id === selectedId)) onTableSelect?.(null);
        }
        setWalls(prev.walls);
        setWallArcs(prev.wallArcs);
        setWallQuads(prev.wallQuads ?? []);
        setRooms(prev.rooms);
        setDecorations(prev.decorations);
        setChairLayout(prev.chairLayout);
      }
      return next;
    });
  }, [hallId, onTableSelect, selectedId]);

  const load = useCallback(async () => {
    setLoadError(null);
    try {
      const { data } = await api.get<{
        tables: TableShape[] | null;
        walls: Wall[] | null;
        wall_segments?: unknown[] | null;
        rooms?: unknown[] | null;
        chair_layout?: Record<string, unknown> | null;
        decorations?: Decoration[] | null;
        canvas_width?: number;
        canvas_height?: number;
        room_inset?: { x?: number; y?: number; w?: number; h?: number };
      }>(`/halls/${hallId}/layout`);
      setTables(Array.isArray(data.tables) ? data.tables : []);
      let nextWalls: Wall[] = Array.isArray(data.walls) ? data.walls : [];
      const nextArcs: WallArc[] = [];
      const nextQuads: WallQuad[] = [];
      if (Array.isArray(data.wall_segments) && data.wall_segments.length > 0) {
        const lineWalls: Wall[] = [];
        for (const raw of data.wall_segments) {
          const s = raw as Record<string, unknown>;
          const typ = String(s.type || 'line');
          if (typ === 'arc') {
            const cx = Number(s.cx);
            const cy = Number(s.cy);
            const r = Number(s.r);
            const a0 = Number(s.a0);
            const a1 = Number(s.a1);
            if (Number.isFinite(cx) && Number.isFinite(cy) && r > 0 && Number.isFinite(a0) && Number.isFinite(a1)) {
              nextArcs.push({ type: 'arc', cx, cy, r, a0, a1 });
            }
          } else if (typ === 'quad') {
            const x1 = Number(s.x1);
            const y1 = Number(s.y1);
            const qx = Number(s.qx);
            const qy = Number(s.qy);
            const x2 = Number(s.x2);
            const y2 = Number(s.y2);
            if (
              [x1, y1, qx, qy, x2, y2].every((n) => Number.isFinite(n))
            ) {
              nextQuads.push({ type: 'quad', x1, y1, qx, qy, x2, y2 });
            }
          } else {
            lineWalls.push({
              x1: Number(s.x1),
              y1: Number(s.y1),
              x2: Number(s.x2),
              y2: Number(s.y2),
            });
          }
        }
        /**
         * Раньше при наличии wall_segments подменяли весь массив линий только сегментами из него —
         * стены из поля `walls` без дубликата в wall_segments пропадали из состояния.
         * Flood тогда «не видел» периметр и заливал почти весь холст.
         */
        if (lineWalls.length > 0) nextWalls = mergeWallLinesDeduped(nextWalls, lineWalls);
      }
      setWalls(nextWalls);
      setWallArcs(nextArcs);
      setWallQuads(nextQuads);
      if (Array.isArray(data.rooms)) {
        const parsed: HallRoom[] = [];
        for (const raw of data.rooms) {
          const r = raw as { polygon?: number[]; kind?: string; label?: string };
          const poly = Array.isArray(r.polygon) ? r.polygon.map((n) => Number(n)).filter((n) => Number.isFinite(n)) : [];
          if (poly.length >= 6) parsed.push({ polygon: poly, kind: r.kind, label: r.label });
        }
        setRooms(parsed);
      } else {
        setRooms([]);
      }
      if (data.chair_layout && typeof data.chair_layout === 'object') {
        const cl: Record<string, ChairPt[]> = {};
        for (const [k, v] of Object.entries(data.chair_layout)) {
          if (!Array.isArray(v)) continue;
          cl[k] = v.map((item) => {
            const o = item as { dx?: number; dy?: number; x?: number; y?: number };
            return { dx: Number(o.dx ?? o.x ?? 0), dy: Number(o.dy ?? o.y ?? 0) };
          });
        }
        setChairLayout(cl);
      } else {
        setChairLayout({});
      }
      setDecorations(Array.isArray(data.decorations) ? data.decorations : []);
      const cw = typeof data.canvas_width === 'number' && data.canvas_width > 0 ? data.canvas_width : DEFAULT_STAGE_W;
      const ch = typeof data.canvas_height === 'number' && data.canvas_height > 0 ? data.canvas_height : DEFAULT_STAGE_H;
      const sw = Math.min(2000, Math.max(400, cw));
      const sh = Math.min(2000, Math.max(400, ch));
      setStageW(sw);
      setStageH(sh);
    } catch (ex: unknown) {
      const ax = ex as { response?: { data?: { error?: string } }; message?: string };
      setLoadError(ax.response?.data?.error || ax.message || 'Не удалось загрузить схему зала');
      setTables([]);
      setWalls([]);
      setWallArcs([]);
      setWallQuads([]);
      setRooms([]);
      setChairLayout({});
      setDecorations([]);
    }
  }, [hallId]);

  useEffect(() => {
    void load();
  }, [load, reloadNonce]);

  const scheduleReload = useCallback(() => {
    if (loadDebounceRef.current != null) window.clearTimeout(loadDebounceRef.current);
    loadDebounceRef.current = window.setTimeout(() => {
      loadDebounceRef.current = null;
      void load();
    }, 300);
  }, [load]);

  useEffect(
    () => () => {
      if (loadDebounceRef.current != null) window.clearTimeout(loadDebounceRef.current);
    },
    [],
  );

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
        scheduleReload();
      }
    },
    [scheduleReload],
  );

  useHallWebSocket(hallId, onWs);

  useEffect(() => {
    setChairLayout((prev) => {
      const next = { ...prev };
      let changed = false;
      const ids = new Set<string>();
      for (const t of tables) {
        ids.add(t.id);
        const rw = t.width && t.width > 0 ? t.width / 2 : t.radius || 28;
        const rh = t.height && t.height > 0 ? t.height / 2 : t.radius || 28;
        const cap = Math.max(1, Math.min(24, t.capacity));
        const pending = pastePendingChairsRef.current.get(t.id);
        if (pending && pending.length === cap) {
          next[t.id] = pending.map((c) => ({ dx: c.dx, dy: c.dy }));
          pastePendingChairsRef.current.delete(t.id);
          changed = true;
          continue;
        }
        const cur = next[t.id];
        if (!cur || cur.length !== cap) {
          next[t.id] = defaultChairOffsets(cap, rw, rh, t.shape || 'circle');
          changed = true;
        }
      }
      for (const k of Object.keys(next)) {
        if (!ids.has(k)) {
          delete next[k];
          changed = true;
        }
      }
      return changed ? next : prev;
    });
  }, [tables]);

  const handleDragEnd = (id: string, x: number, y: number) => {
    setTables((prev) => prev.map((t) => (t.id === id ? { ...t, x: snap(x), y: snap(y) } : t)));
  };

  const wallSegmentsPayload = useMemo(
    () => [
      ...walls.map((w) => ({ type: 'line' as const, x1: w.x1, y1: w.y1, x2: w.x2, y2: w.y2 })),
      ...wallQuads,
      ...wallArcs,
    ],
    [walls, wallArcs, wallQuads],
  );

  const decorationDrawOrder = useMemo(() => {
    const non: number[] = [];
    const named: { i: number; area: number }[] = [];
    decorations.forEach((d, i) => {
      if (String(d.type) === 'zone_named') {
        named.push({ i, area: polygonArea((d.points as number[]) || []) });
      } else non.push(i);
    });
    named.sort((a, b) => b.area - a.area);
    return [...named.map((x) => x.i), ...non];
  }, [decorations]);

  const zoneNamedHoleMap = useMemo(() => {
    const list: { i: number; poly: number[]; area: number }[] = [];
    decorations.forEach((d, idx) => {
      if (String(d.type) !== 'zone_named') return;
      const poly = (d.points as number[]) || [];
      if (poly.length >= 6) list.push({ i: idx, poly, area: polygonArea(poly) });
    });
    const map = new Map<number, number[][]>();
    for (const z of list) {
      const others = list.filter((x) => x.i !== z.i).map((x) => ({ poly: x.poly, area: x.area }));
      map.set(z.i, zoneNamedDirectHolePolys(z.poly, z.area, others));
    }
    return map;
  }, [decorations]);

  const deleteSelectedTable = useCallback(async () => {
    if (!selectedId) return;
    if (!window.confirm('Удалить стол? Это действие необратимо.')) return;
    captureUndo();
    try {
      await api.delete(`/halls/${hallId}/tables/${selectedId}`);
      onTableSelect?.(null);
      await load();
    } catch (e: unknown) {
      const ax = e as { response?: { data?: { error?: string } } };
      window.alert(ax.response?.data?.error || 'Не удалось удалить стол');
    }
  }, [selectedId, hallId, load, onTableSelect]);

  const pasteTableFromClip = useCallback(
    async (clip: TableClip) => {
      const nums = tablesRef.current.map((t) => t.number);
      const nextNum = (nums.length ? Math.max(...nums) : 0) + 1;
      captureUndo();
      try {
        const { data } = await api.post<{ id: string }>(`/halls/${hallId}/tables`, {
          table_number: nextNum,
          capacity: clip.capacity,
          x: snap(clip.x + 48),
          y: snap(clip.y + 48),
          shape: clip.shape,
          width: clip.width,
          height: clip.height,
          rotation_deg: clip.rotation_deg ?? 0,
        });
        const newId = data.id;
        pastePendingChairsRef.current.set(
          newId,
          clip.chairs.map((c) => ({ dx: c.dx, dy: c.dy })),
        );
        await load();
        window.setTimeout(() => {
          const t = tablesRef.current.find((x) => x.id === newId);
          if (t) onTableSelect?.(t);
        }, 0);
      } catch {
        window.alert('Не удалось вставить стол');
      }
    },
    [hallId, load, onTableSelect],
  );

  const saveLayout = async () => {
    await api.put(`/halls/${hallId}/layout`, {
      canvas_width: stageW,
      canvas_height: stageH,
      room_inset: roomInset,
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
      wall_segments: wallSegmentsPayload,
      rooms,
      chair_layout: chairLayout,
      decorations,
    });
    await load();
  };

  const addTableFromModal = async (f: AddTableForm) => {
    captureUndo();
    const n = f.tableNumber ?? (tables.length ? Math.max(...tables.map((t) => t.number)) + 1 : 1);
    await api.post(`/halls/${hallId}/tables`, {
      table_number: n,
      capacity: f.capacity,
      x: 240,
      y: 280,
      shape: f.shape,
      width: f.width,
      height: f.height,
      rotation_deg: 0,
    });
    await load();
  };

  const addZoneLabel = () => {
    captureUndo();
    setDecorations((d) => [...d, { type: 'zone_label', text: 'Зона', x: 120, y: 80, w: 140, h: 28 }]);
  };

  const confirmZoneFill = (label: string, fill: string, stroke: string) => {
    captureUndo();
    const lx = snap(zoneFillLabelPos.x);
    const ly = snap(zoneFillLabelPos.y);
    setDecorations((d) => [
      ...d,
      {
        type: 'zone_named',
        points: [...zoneFillPoints],
        label,
        fill,
        stroke,
        labelX: lx,
        labelY: ly,
      },
    ]);
    setPlaceMode('none');
  };

  const handleFloorPointer = (e: KonvaEventObject<MouseEvent>) => {
    if (!editMode || editorMode === 'view') return;
    const p = pointerToWorld(e);
    if (!p) return;
    const x = snap(p.x);
    const y = snap(p.y);
    const px = p.x;
    const py = p.y;

    if (editorMode === 'walls') {
      if (placeMode === 'wallquad') {
        if (!wallQuadDraft) {
          if (!lineStart) {
            setLineStart({ x, y });
            return;
          }
          setWallQuadDraft({ x1: lineStart.x, y1: lineStart.y, x2: x, y2: y });
          setLineStart(null);
          return;
        }
        captureUndo();
        setWallQuads((wq) => [
          ...wq,
          { type: 'quad', x1: wallQuadDraft.x1, y1: wallQuadDraft.y1, qx: x, qy: y, x2: wallQuadDraft.x2, y2: wallQuadDraft.y2 },
        ]);
        setWallQuadDraft(null);
        setPlaceMode('none');
        return;
      }
      if (placeMode !== 'wallseg') return;
      if (!lineStart) {
        setLineStart({ x, y });
        return;
      }
      captureUndo();
      setWalls((w) => [...w, { x1: lineStart.x, y1: lineStart.y, x2: x, y2: y }]);
      setLineStart(null);
      setPlaceMode('none');
      return;
    }

    if (placeMode === 'zone_fill') {
      type ZHit = { poly: number[]; area: number };
      const decHits: ZHit[] = [];
      for (const d of decorations) {
        const typ = String(d.type || '');
        if (typ === 'zone_polygon' || typ === 'zone_named') {
          const pts = (d.points as number[]) || [];
          if (pts.length >= 6 && pointInPolygon(px, py, pts)) {
            decHits.push({ poly: [...pts], area: polygonArea(pts) });
          }
        }
      }
      const roomHits: ZHit[] = [];
      for (const room of rooms) {
        const poly = room.polygon;
        if (pointInPolygon(px, py, poly)) {
          roomHits.push({ poly: [...poly], area: polygonArea(poly) });
        }
      }
      const hits: ZHit[] = [];
      if (decHits.length > 0) {
        // Любая зона из декора: не подмешиваем flood — BFS по сетке даёт артефакты у косых стен и может «перебить» меньшую зону по площади.
        hits.push(...decHits, ...roomHits);
      } else {
        const fromWalls = floodRegionPolygonFromWalls(px, py, stageW, stageH, walls, wallArcs, wallQuads);
        if (fromWalls && fromWalls.length >= 6) {
          hits.push({ poly: [...fromWalls], area: polygonArea(fromWalls) });
        }
        hits.push(...roomHits);
      }
      if (hits.length > 0) {
        hits.sort((a, b) => a.area - b.area);
        const best = hits[0];
        setZoneFillPoints(best.poly);
        setZoneFillLabelPos({ x: px, y: py });
        setZoneFillModalOpen(true);
      }
      return;
    }

    if (placeMode === 'none') return;

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
    }
  };

  const onWheel = (e: KonvaEventObject<WheelEvent>) => {
    e.evt.preventDefault();
    const st = stageRef.current;
    if (!st) return;
    const p = st.getPointerPosition();
    if (!p) return;
    const oldScale = scaleRef.current;
    const oldPan = panRef.current;
    const mx = (p.x - oldPan.x) / oldScale;
    const my = (p.y - oldPan.y) / oldScale;
    const direction = e.evt.deltaY > 0 ? -1 : 1;
    const newScale = Math.min(4, Math.max(0.15, oldScale + direction * 0.09));
    const newPan = { x: p.x - mx * newScale, y: p.y - my * newScale };
    scaleRef.current = newScale;
    panRef.current = newPan;
    setScale(newScale);
    setWorldPan(newPan);
  };

  const onStageMouseDown = (e: KonvaEventObject<MouseEvent>) => {
    if (e.evt.button !== 0 && e.evt.button !== 1) return;
    const stage = e.target.getStage();
    if (!stage) return;
    const nm = typeof e.target.name === 'function' ? e.target.name() : '';
    const isBg = nm === 'worldBg' || e.target === stage;
    const altMid = e.evt.button === 1 || e.evt.altKey;
    const handLike = editMode && placeMode === 'none' && spacePanRef.current && isBg;
    const viewPan = editMode && editorMode === 'view' && isBg && e.evt.button === 0;
    const allow = altMid || handLike || viewPan || (!editMode && (e.evt.button === 1 || e.evt.altKey));
    if (!allow) return;
    if (editMode && placeMode !== 'none' && !altMid && editorMode !== 'view') return;
    e.evt.preventDefault();
    setPanDrag({ px: e.evt.clientX, py: e.evt.clientY, ox: worldPan.x, oy: worldPan.y });
  };

  const updateDecoration = (i: number, patch: Record<string, unknown>) => {
    setDecorations((prev) => prev.map((d, j) => (j === i ? { ...d, ...patch } : d)));
  };

  const clearWallArcPick = () => {
    setSelectedWall(null);
    setSelectedArc(null);
    setSelectedQuad(null);
  };

  const pickWall = useCallback(
    (i: number) => {
      setSelectedWall(i);
      setSelectedDec(null);
      setSelectedArc(null);
      setSelectedQuad(null);
      onTableSelect?.(null);
    },
    [onTableSelect],
  );

  const pickArc = useCallback(
    (i: number) => {
      setSelectedArc(i);
      setSelectedWall(null);
      setSelectedQuad(null);
      setSelectedDec(null);
      onTableSelect?.(null);
    },
    [onTableSelect],
  );

  const pickQuad = useCallback(
    (i: number) => {
      setSelectedQuad(i);
      setSelectedWall(null);
      setSelectedArc(null);
      setSelectedDec(null);
      onTableSelect?.(null);
    },
    [onTableSelect],
  );

  const selectDecOnly = useCallback(
    (i: number) => {
      setSelectedDec(i);
      setSelectedWall(null);
      setSelectedArc(null);
      setSelectedQuad(null);
      onTableSelect?.(null);
    },
    [onTableSelect],
  );

  const onQuadControlDrag = useCallback((quadIndex: number, qx: number, qy: number) => {
    setWallQuads((list) => list.map((q, j) => (j === quadIndex ? { ...q, qx, qy } : q)));
  }, []);

  const onWallEndpointDrag = useCallback((wallIndex: number, end: 'a' | 'b', x: number, y: number) => {
    setWalls((list) =>
      list.map((w, j) =>
        j !== wallIndex ? w : end === 'a' ? { ...w, x1: x, y1: y } : { ...w, x2: x, y2: y },
      ),
    );
  }, []);

  const deleteSelected = () => {
    if (selectedQuad !== null) {
      captureUndo();
      setWallQuads((q) => q.filter((_, j) => j !== selectedQuad));
      setSelectedQuad(null);
      return;
    }
    if (selectedWall !== null) {
      captureUndo();
      setWalls((w) => w.filter((_, j) => j !== selectedWall));
      clearWallArcPick();
      return;
    }
    if (selectedArc !== null) {
      captureUndo();
      setWallArcs((a) => a.filter((_, j) => j !== selectedArc));
      setSelectedArc(null);
      return;
    }
    if (selectedDec !== null) {
      captureUndo();
      setDecorations((d) => d.filter((_, j) => j !== selectedDec));
      setSelectedDec(null);
      return;
    }
    if (
      editMode &&
      editorMode === 'interior' &&
      placeMode === 'none' &&
      selectedId &&
      selectedWall === null &&
      selectedArc === null &&
      selectedQuad === null &&
      selectedDec === null
    ) {
      void deleteSelectedTable();
    }
  };

  deleteSelectedRef.current = deleteSelected;
  selectionKeyRef.current = { selectedDec, selectedWall, selectedArc, selectedQuad };

  editorKeyRef.current = {
    editMode,
    editorMode,
    placeMode,
    selectedId: selectedId ?? null,
    zoneFillModalOpen,
    addTableOpen,
  };

  useEffect(() => {
    if (!editMode) return;
    const onKey = (ev: KeyboardEvent) => {
      if (isInputLikeTarget(ev.target)) return;
      const mod = ev.metaKey || ev.ctrlKey;
      if (mod && ev.key.toLowerCase() === 'z' && !ev.shiftKey) {
        ev.preventDefault();
        doUndo();
        return;
      }
      const ctx = editorKeyRef.current;
      if (mod && ev.key.toLowerCase() === 'c') {
        if (
          ctx.editMode &&
          ctx.editorMode === 'interior' &&
          ctx.placeMode === 'none' &&
          ctx.selectedId &&
          !ctx.zoneFillModalOpen &&
          !ctx.addTableOpen
        ) {
          const t = tablesRef.current.find((x) => x.id === ctx.selectedId);
          if (t) {
            const tw = t.width && t.width > 0 ? t.width : (t.radius ?? 28) * 2;
            const th = t.height && t.height > 0 ? t.height : (t.radius ?? 28) * 2;
            const cap = Math.max(1, Math.min(24, t.capacity));
            const rwH = tw / 2;
            const rhH = th / 2;
            let chairs = chairLayoutRef.current[t.id] || [];
            if (chairs.length !== cap) {
              chairs = defaultChairOffsets(cap, rwH, rhH, t.shape || 'circle');
            }
            tableClipRef.current = {
              capacity: t.capacity,
              x: t.x,
              y: t.y,
              shape: (t.shape || 'circle').toLowerCase(),
              width: tw,
              height: th,
              rotation_deg: t.rotation_deg ?? 0,
              chairs: chairs.map((c) => ({ dx: c.dx, dy: c.dy })),
            };
            ev.preventDefault();
          }
        }
        return;
      }
      if (mod && ev.key.toLowerCase() === 'v') {
        if (
          ctx.editMode &&
          ctx.editorMode === 'interior' &&
          !ctx.zoneFillModalOpen &&
          !ctx.addTableOpen
        ) {
          const clip = tableClipRef.current;
          if (clip) {
            ev.preventDefault();
            void pasteTableFromClip(clip);
          }
        }
        return;
      }
      if (ev.key !== 'Delete' && ev.key !== 'Backspace') return;
      const s = selectionKeyRef.current;
      const hasStruct =
        s.selectedDec !== null || s.selectedWall !== null || s.selectedArc !== null || s.selectedQuad !== null;
      if (hasStruct) {
        ev.preventDefault();
        deleteSelectedRef.current();
        return;
      }
      if (
        ctx.editMode &&
        ctx.editorMode === 'interior' &&
        ctx.placeMode === 'none' &&
        ctx.selectedId
      ) {
        ev.preventDefault();
        void deleteSelectedTable();
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [editMode, doUndo, deleteSelectedTable, pasteTableFromClip]);

  const listenWalls = editMode && editorMode === 'walls';
  const listenInterior = editMode && editorMode === 'interior';
  const tableInteractive = listenInterior && placeMode === 'none';
  const decorSelectable = listenInterior && placeMode === 'none';
  const wallPickActive = listenWalls && placeMode === 'none';

  const resetToolState = () => {
    setPlaceMode('none');
    setLineStart(null);
    setWallQuadDraft(null);
    setSelectedDec(null);
    clearWallArcPick();
  };

  const setMode = (m: EditorMode) => {
    setEditorMode(m);
    resetToolState();
  };

  return (
    <div className="hall-canvas-wrap">
      {loadError && <p className="form-msg">{loadError}</p>}
      <HallAddTableModal open={addTableOpen} onClose={() => setAddTableOpen(false)} onSubmit={(f) => addTableFromModal(f)} />
      <HallZoneFillModal
        open={zoneFillModalOpen}
        onClose={() => setZoneFillModalOpen(false)}
        onConfirm={(label, fill, stroke) => {
          confirmZoneFill(label, fill, stroke);
          setZoneFillModalOpen(false);
        }}
      />
      {editMode && (
        <div className="hall-toolbar-panel hall-toolbar-wrap">
          <div className="hall-mode-switch" role="tablist" aria-label="Режим редактора">
            <button
              type="button"
              role="tab"
              className={editorMode === 'view' ? 'hall-mode-btn hall-mode-btn--active' : 'hall-mode-btn'}
              onClick={() => setMode('view')}
            >
              Просмотр
            </button>
            <button
              type="button"
              role="tab"
              className={editorMode === 'walls' ? 'hall-mode-btn hall-mode-btn--active' : 'hall-mode-btn'}
              onClick={() => setMode('walls')}
            >
              Стены
            </button>
            <button
              type="button"
              role="tab"
              className={editorMode === 'interior' ? 'hall-mode-btn hall-mode-btn--active' : 'hall-mode-btn'}
              onClick={() => setMode('interior')}
            >
              Интерьер
            </button>
          </div>

          <div className="hall-toolbar-group hall-toolbar-save-row">
            <button type="button" className="btn" onClick={() => void saveLayout()}>
              Сохранить схему
            </button>
            <span className="muted compact">
              Сохраняет столы, стены, зоны и размер чертежа на сервер. Откат: ⌘Z / Ctrl+Z.
            </span>
          </div>

          {editorMode === 'view' && (
            <p className="hall-toolbar-hint muted">
              Только масштаб и сдвиг схемы (мышь / тач). Левый клик по фону — перетаскивание.
            </p>
          )}

          {editorMode === 'walls' && (
            <div className="hall-toolbar-group hall-toolbar-tools">
              <span className="hall-toolbar-label">Инструменты</span>
              <button
                type="button"
                className={placeMode === 'wallseg' ? 'btn' : 'secondary'}
                onClick={() => {
                  setPlaceMode(placeMode === 'wallseg' ? 'none' : 'wallseg');
                  setLineStart(null);
                  setWallQuadDraft(null);
                }}
              >
                Линия
              </button>
              <button
                type="button"
                className={placeMode === 'wallquad' ? 'btn' : 'secondary'}
                onClick={() => {
                  setPlaceMode(placeMode === 'wallquad' ? 'none' : 'wallquad');
                  setLineStart(null);
                  setWallQuadDraft(null);
                }}
                title="Три клика: начало сегмента, конец, точка изгиба (квадратичная кривая)"
              >
                Кривая
              </button>
              <button
                type="button"
                className="secondary"
                disabled={selectedWall === null && selectedArc === null && selectedQuad === null}
                onClick={deleteSelected}
              >
                Удалить сегмент
              </button>
            </div>
          )}

          {editorMode === 'interior' && (
            <>
            <div className="hall-toolbar-group hall-toolbar-tools">
              <span className="hall-toolbar-label">Инструменты</span>
              <button
                type="button"
                className={placeMode === 'door' ? 'btn' : 'secondary'}
                onClick={() => {
                  setPlaceMode(placeMode === 'door' ? 'none' : 'door');
                  setLineStart(null);
                  setSelectedDec(null);
                  clearWallArcPick();
                  onTableSelect?.(null);
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
                  setSelectedDec(null);
                  clearWallArcPick();
                  onTableSelect?.(null);
                }}
              >
                Окно
              </button>
              <button
                type="button"
                className={placeMode === 'zone_fill' ? 'btn' : 'secondary'}
                onClick={() => {
                  setPlaceMode(placeMode === 'zone_fill' ? 'none' : 'zone_fill');
                  setSelectedDec(null);
                  clearWallArcPick();
                  onTableSelect?.(null);
                }}
                title="Клик внутри области, ограниченной стенами и краем чертежа, или внутри контура помещения / полигона зоны. При вложенных контурах выбирается меньший."
              >
                Зона по клику
              </button>
              <button type="button" className="secondary" onClick={addZoneLabel}>
                Подпись
              </button>
              <button type="button" onClick={() => setAddTableOpen(true)}>
                + Стол
              </button>
              <button
                type="button"
                className="secondary"
                disabled={selectedDec === null && !selectedId}
                onClick={deleteSelected}
              >
                Удалить объект
              </button>
            </div>
            <p className="muted" style={{ fontSize: 12, margin: '4px 0 0' }}>
              Стол: Ctrl+C / Ctrl+V — копия и вставка (вместе со стульями).
            </p>
            </>
          )}

          <div className="hall-toolbar-group hall-toolbar-view-row">
            <button type="button" className="secondary" onClick={() => setShowGrid((g) => !g)}>
              Сетка
            </button>
            <button
              type="button"
              className="secondary"
              onClick={() => {
                const cx = viewportW / 2;
                const cy = viewportH / 2;
                const oldScale = scaleRef.current;
                const oldPan = panRef.current;
                const mx = (cx - oldPan.x) / oldScale;
                const my = (cy - oldPan.y) / oldScale;
                const newScale = Math.min(4, oldScale + 0.1);
                const newPan = { x: cx - mx * newScale, y: cy - my * newScale };
                scaleRef.current = newScale;
                panRef.current = newPan;
                setScale(newScale);
                setWorldPan(newPan);
              }}
            >
              +
            </button>
            <button
              type="button"
              className="secondary"
              onClick={() => {
                const cx = viewportW / 2;
                const cy = viewportH / 2;
                const oldScale = scaleRef.current;
                const oldPan = panRef.current;
                const mx = (cx - oldPan.x) / oldScale;
                const my = (cy - oldPan.y) / oldScale;
                const newScale = Math.max(0.15, oldScale - 0.1);
                const newPan = { x: cx - mx * newScale, y: cy - my * newScale };
                scaleRef.current = newScale;
                panRef.current = newPan;
                setScale(newScale);
                setWorldPan(newPan);
              }}
            >
              −
            </button>
            <button
              type="button"
              className="secondary btn-sm"
              onClick={() => {
                scaleRef.current = 1;
                panRef.current = { x: 0, y: 0 };
                setScale(1);
                setWorldPan({ x: 0, y: 0 });
              }}
            >
              Сброс вида
            </button>
            <span className="muted hall-zoom-label">{Math.round(scale * 100)}%</span>
          </div>

          <details className="hall-toolbar-section hall-advanced-details">
            <summary className="hall-toolbar-summary">Дополнительно</summary>
            <p className="muted compact hall-canvas-help">
              Размер чертежа (px) — границы схемы зала в файле; влияет на экспорт и границы мира.
            </p>
            <div className="hall-toolbar-group">
              <span className="hall-toolbar-label">Размер чертежа (px)</span>
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
                title="Ширина чертежа"
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
                title="Высота чертежа"
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
            </div>
          </details>
        </div>
      )}
      {editMode && placeMode !== 'none' && (
        <p className="hint hall-place-hint">
          {placeMode === 'door' && (lineStart ? 'Второй клик — конец двери' : 'Первый клик — начало двери')}
          {placeMode === 'window' && (lineStart ? 'Второй клик — конец окна' : 'Первый клик — начало окна')}
          {placeMode === 'wallseg' && (lineStart ? 'Второй клик — конец сегмента' : 'Первый клик — начало сегмента')}
          {placeMode === 'wallquad' &&
            (wallQuadDraft
              ? 'Третий клик — точка изгиба кривой (потом можно тянуть жёлтую ручку)'
              : lineStart
                ? 'Второй клик — конец сегмента'
                : 'Первый клик — начало кривой стены')}
          {placeMode === 'zone_fill' &&
            'Клик: стены (flood), контур помещения (rooms), полигон зоны. Несколько зон в декоре — выбирается меньшая по площади; контур комнаты тоже участвует. Имя и цвет — в диалоге.'}
        </p>
      )}

      <div ref={containerRef} className="hall-viewport-shell">
        <Stage
          ref={stageRef}
          width={viewportW}
          height={viewportH}
          className="hall-stage"
          onWheel={onWheel}
          onMouseDown={onStageMouseDown}
          onTouchStart={(e) => {
            if (e.evt.touches.length === 2) {
              const t0 = e.evt.touches[0];
              const t1 = e.evt.touches[1];
              const dist = Math.hypot(t0.clientX - t1.clientX, t0.clientY - t1.clientY);
              const cx = (t0.clientX + t1.clientX) / 2;
              const cy = (t0.clientY + t1.clientY) / 2;
              const rect = containerRef.current?.getBoundingClientRect();
              if (!rect) return;
              const px = cx - rect.left;
              const py = cy - rect.top;
              touchPinchRef.current = { dist, scale: scaleRef.current, cx: px, cy: py, px: worldPan.x, py: worldPan.y };
              return;
            }
            if (e.evt.touches.length === 1) {
              const st = stageRef.current;
              if (!st) return;
              const node = e.target as Konva.Node;
              const nm = typeof node.name === 'function' ? node.name() : '';
              const isBg = nm === 'worldBg' || node === st;
              const allowPan =
                (!editMode && isBg) ||
                (editMode && editorMode === 'view' && isBg) ||
                (editMode && placeMode === 'none' && spacePanRef.current && isBg);
              if (allowPan) {
                const t0 = e.evt.touches[0];
                setPanDrag({ px: t0.clientX, py: t0.clientY, ox: worldPan.x, oy: worldPan.y });
              }
            }
          }}
          onTouchMove={(e) => {
            const tp = touchPinchRef.current;
            if (!tp || e.evt.touches.length < 2) return;
            e.evt.preventDefault();
            const t0 = e.evt.touches[0];
            const t1 = e.evt.touches[1];
            const dist = Math.hypot(t0.clientX - t1.clientX, t0.clientY - t1.clientY);
            const cx = (t0.clientX + t1.clientX) / 2;
            const cy = (t0.clientY + t1.clientY) / 2;
            const rect = containerRef.current?.getBoundingClientRect();
            if (!rect) return;
            const px = cx - rect.left;
            const py = cy - rect.top;
            const factor = dist / tp.dist;
            let newScale = Math.min(4, Math.max(0.15, tp.scale * factor));
            const mx = (tp.cx - tp.px) / tp.scale;
            const my = (tp.cy - tp.py) / tp.scale;
            const newPan = { x: px - mx * newScale, y: py - my * newScale };
            scaleRef.current = newScale;
            panRef.current = newPan;
            setScale(newScale);
            setWorldPan(newPan);
          }}
          onTouchEnd={() => {
            touchPinchRef.current = null;
          }}
        >
          <Layer clipX={0} clipY={0} clipWidth={viewportW} clipHeight={viewportH}>
            <Group x={worldPan.x} y={worldPan.y} scaleX={scale} scaleY={scale}>
              <Rect
                name="worldBg"
                x={0}
                y={0}
                width={stageW}
                height={stageH}
                fill="#0f172a"
                cornerRadius={12}
                listening
                onMouseDown={(e) => {
                  if (editMode && editorMode !== 'view' && placeMode !== 'none') {
                    handleFloorPointer(e);
                    return;
                  }
                  if (
                    editMode &&
                    editorMode !== 'view' &&
                    placeMode === 'none' &&
                    e.evt.button === 0 &&
                    !spacePanRef.current
                  ) {
                    onTableSelect?.(null);
                    setSelectedDec(null);
                    clearWallArcPick();
                  }
                }}
              />
              <HallCanvasGrid
                show={showGrid && editMode}
                step={GRID}
                minX={gridWorldBounds.minX}
                maxX={gridWorldBounds.maxX}
                minY={gridWorldBounds.minY}
                maxY={gridWorldBounds.maxY}
              />
              {rooms.map((room, ri) => (
                <Group key={`room-${ri}`} listening={false}>
                  <Line
                    closed
                    points={room.polygon}
                    fill="rgba(99,102,241,0.08)"
                    stroke="rgba(129,140,248,0.45)"
                    strokeWidth={2}
                    perfectDrawEnabled={false}
                  />
                  <Text
                    x={room.polygon[0] ?? 0}
                    y={(room.polygon[1] ?? 0) - 18}
                    text={String(room.label || room.kind || 'Помещение')}
                    fontSize={12}
                    fill="#a5b4fc"
                    listening={false}
                  />
                </Group>
              ))}
              <HallCanvasWallsLayer
                walls={walls}
                wallArcs={wallArcs}
                wallQuads={wallQuads}
                selectedWall={selectedWall}
                selectedArc={selectedArc}
                selectedQuad={selectedQuad}
                wallPickActive={wallPickActive}
                editMode={editMode}
                snap={snap}
                onPickWall={pickWall}
                onPickArc={pickArc}
                onPickQuad={pickQuad}
                onWallEndpointDrag={onWallEndpointDrag}
                onQuadControlDrag={onQuadControlDrag}
              />
              {decorationDrawOrder.map((i) => {
                const dec = decorations[i];
                const t = String(dec.type || '');
                const sel = selectedDec === i;
                const canEdit = decorSelectable;

                if (t === 'zone_label') {
                  const x = Number(dec.x) || 0;
                  const y = Number(dec.y) || 0;
                  const w = Number(dec.w) || 120;
                  const h = Number(dec.h) || 28;
                  const text = String(dec.text || '');
                  return (
                    <Group
                      key={`d-${i}`}
                      x={x}
                      y={y}
                      listening={decorSelectable}
                      draggable={canEdit && sel}
                      onTap={(e) => {
                        e.cancelBubble = true;
                        selectDecOnly(i);
                      }}
                      onClick={(e) => {
                        e.cancelBubble = true;
                        selectDecOnly(i);
                      }}
                      onDragEnd={(e) => {
                        updateDecoration(i, { x: snap(e.target.x()), y: snap(e.target.y()) });
                      }}
                    >
                      <Rect
                        width={w}
                        height={h}
                        fill={sel ? 'rgba(99,102,241,0.35)' : 'rgba(99,102,241,0.15)'}
                        cornerRadius={6}
                        stroke={sel ? '#a5b4fc' : undefined}
                        strokeWidth={sel ? 2 : 0}
                      />
                      <Text x={8} y={6} text={text} fontSize={13} fill="#a5b4fc" listening={false} />
                    </Group>
                  );
                }
                if (t === 'window_band') {
                  const x = Number(dec.x) || 0;
                  const y = Number(dec.y) || 0;
                  const ww = Number(dec.w) || stageW;
                  const hh = Number(dec.h) || 24;
                  return (
                    <Group
                      key={`d-${i}`}
                      x={x}
                      y={y}
                      listening={decorSelectable}
                      draggable={canEdit && sel}
                      onTap={(e) => {
                        e.cancelBubble = true;
                        selectDecOnly(i);
                      }}
                      onClick={(e) => {
                        e.cancelBubble = true;
                        selectDecOnly(i);
                      }}
                      onDragEnd={(e) => {
                        updateDecoration(i, { x: snap(e.target.x()), y: snap(e.target.y()) });
                      }}
                    >
                      <Rect
                        width={ww}
                        height={hh}
                        fill={sel ? 'rgba(56,189,248,0.45)' : 'rgba(56,189,248,0.25)'}
                        stroke={sel ? '#38bdf8' : undefined}
                        strokeWidth={sel ? 2 : 0}
                      />
                    </Group>
                  );
                }
                if (t === 'door' || t === 'window') {
                  const x1 = Number(dec.x1);
                  const y1 = Number(dec.y1);
                  const x2 = Number(dec.x2);
                  const y2 = Number(dec.y2);
                  const stroke = t === 'door' ? '#a16207' : '#38bdf8';
                  const swid = t === 'door' ? 8 : 5;
                  return (
                    <Group
                      key={`d-${i}`}
                      listening={decorSelectable}
                      onTap={(e) => {
                        e.cancelBubble = true;
                        selectDecOnly(i);
                      }}
                      onClick={(e) => {
                        e.cancelBubble = true;
                        selectDecOnly(i);
                      }}
                    >
                      <Line
                        points={[x1, y1, x2, y2]}
                        stroke={stroke}
                        strokeWidth={swid}
                        lineCap="round"
                        hitStrokeWidth={36}
                      />
                      {canEdit && sel && (
                        <>
                          <Circle
                            x={x1}
                            y={y1}
                            radius={10}
                            fill="#f8fafc"
                            stroke="#64748b"
                            draggable
                            hitStrokeWidth={24}
                            perfectDrawEnabled={false}
                            onDragEnd={(e) => {
                              updateDecoration(i, { x1: snap(e.target.x()), y1: snap(e.target.y()) });
                            }}
                          />
                          <Circle
                            x={x2}
                            y={y2}
                            radius={10}
                            fill="#f8fafc"
                            stroke="#64748b"
                            draggable
                            hitStrokeWidth={24}
                            perfectDrawEnabled={false}
                            onDragEnd={(e) => {
                              updateDecoration(i, { x2: snap(e.target.x()), y2: snap(e.target.y()) });
                            }}
                          />
                        </>
                      )}
                    </Group>
                  );
                }
                if (t === 'zone_named') {
                  const pts = (dec.points as number[]) || [];
                  if (pts.length < 6) return null;
                  const lab = String(dec.label || 'Зона');
                  const fill = String(dec.fill || 'rgba(52,211,153,0.2)');
                  const stroke = String(dec.stroke || 'rgba(16,185,129,0.55)');
                  const lx = Number(dec.labelX) || pts[0] || 0;
                  const ly = Number(dec.labelY) || pts[1] || 0;
                  const tw = Math.max(72, Math.min(320, lab.length * 9 + 28));
                  const renameZone = (e: KonvaEventObject<unknown>) => {
                    e.cancelBubble = true;
                    if (!canEdit) return;
                    const next = window.prompt('Подпись зоны', lab);
                    if (next === null) return;
                    const trimmed = next.trim();
                    if (trimmed === '' || trimmed === lab) return;
                    captureUndo();
                    updateDecoration(i, { label: trimmed });
                  };
                  const holes = zoneNamedHoleMap.get(i) ?? [];
                  const zStrokeW = sel ? 3 : 2;
                  return (
                    <Group key={`d-${i}`}>
                      {holes.length === 0 ? (
                        <Line
                          closed
                          points={pts}
                          fill={fill}
                          stroke={stroke}
                          strokeWidth={zStrokeW}
                          perfectDrawEnabled={false}
                          hitStrokeWidth={28}
                          listening={decorSelectable}
                          onTap={(e) => {
                            e.cancelBubble = true;
                            selectDecOnly(i);
                          }}
                          onClick={(e) => {
                            e.cancelBubble = true;
                            selectDecOnly(i);
                          }}
                        />
                      ) : (
                        <Shape
                          sceneFunc={(ctx) => {
                            ctx.beginPath();
                            traceClosedPoly(ctx, pts);
                            for (const h of holes) traceClosedPoly(ctx, reversePolygonRingFlat(h));
                            ctx.fillStyle = fill;
                            ctx.fill('evenodd');
                            ctx.strokeStyle = stroke;
                            ctx.lineWidth = zStrokeW;
                            ctx.lineJoin = 'round';
                            ctx.lineCap = 'round';
                            ctx.beginPath();
                            traceClosedPoly(ctx, pts);
                            ctx.stroke();
                            for (const h of holes) {
                              ctx.beginPath();
                              traceClosedPoly(ctx, h);
                              ctx.stroke();
                            }
                          }}
                          fill={fill}
                          stroke={stroke}
                          strokeWidth={zStrokeW}
                          perfectDrawEnabled={false}
                          listening={decorSelectable}
                          hitStrokeWidth={28}
                          onTap={(e) => {
                            e.cancelBubble = true;
                            selectDecOnly(i);
                          }}
                          onClick={(e) => {
                            e.cancelBubble = true;
                            selectDecOnly(i);
                          }}
                        />
                      )}
                      <Group
                        x={lx}
                        y={ly}
                        listening={decorSelectable}
                        draggable={canEdit}
                        onDragStart={() => {
                          selectDecOnly(i);
                        }}
                        onDragEnd={(e) => {
                          updateDecoration(i, { labelX: snap(e.target.x()), labelY: snap(e.target.y()) });
                        }}
                        onDblClick={renameZone}
                        onDblTap={renameZone}
                        onTap={(e) => {
                          e.cancelBubble = true;
                          selectDecOnly(i);
                        }}
                        onClick={(e) => {
                          e.cancelBubble = true;
                          selectDecOnly(i);
                        }}
                      >
                        <Rect
                          x={-tw / 2}
                          y={-14}
                          width={tw}
                          height={28}
                          fill="rgba(15,23,42,0.22)"
                          cornerRadius={5}
                          stroke={sel ? 'rgba(248,250,252,0.5)' : 'rgba(248,250,252,0.15)'}
                          strokeWidth={1}
                        />
                        <Text
                          x={-tw / 2}
                          y={-11}
                          width={tw}
                          text={lab}
                          fontSize={13}
                          fill="#f8fafc"
                          align="center"
                          listening={false}
                        />
                      </Group>
                    </Group>
                  );
                }
                if (t === 'zone') {
                  const x = Number(dec.x) || 0;
                  const y = Number(dec.y) || 0;
                  const w = Number(dec.w) || 80;
                  const h = Number(dec.h) || 80;
                  const lab = String(dec.label || 'Зона');
                  return (
                    <Group
                      key={`d-${i}`}
                      x={x}
                      y={y}
                      listening={decorSelectable}
                      draggable={canEdit && sel}
                      onTap={(e) => {
                        e.cancelBubble = true;
                        selectDecOnly(i);
                      }}
                      onClick={(e) => {
                        e.cancelBubble = true;
                        selectDecOnly(i);
                      }}
                      onDragEnd={(e) => {
                        updateDecoration(i, { x: snap(e.target.x()), y: snap(e.target.y()) });
                      }}
                    >
                      <Rect
                        width={w}
                        height={h}
                        fill={sel ? 'rgba(52,211,153,0.28)' : 'rgba(52,211,153,0.12)'}
                        stroke="rgba(52,211,153,0.5)"
                        strokeWidth={2}
                      />
                      <Text x={8} y={8} text={lab} fontSize={13} fill="#6ee7b7" listening={false} />
                    </Group>
                  );
                }
                if (t === 'zone_polygon') {
                  const pts = (dec.points as number[]) || [];
                  if (pts.length < 6) return null;
                  return (
                    <Line
                      key={`d-${i}`}
                      closed
                      points={pts}
                      fill="rgba(52,211,153,0.12)"
                      stroke={selectedDec === i ? '#34d399' : 'rgba(52,211,153,0.6)'}
                      strokeWidth={selectedDec === i ? 3 : 2}
                      hitStrokeWidth={28}
                      listening={decorSelectable}
                      onTap={(e) => {
                        e.cancelBubble = true;
                        selectDecOnly(i);
                      }}
                      onClick={(e) => {
                        e.cancelBubble = true;
                        selectDecOnly(i);
                      }}
                    />
                  );
                }
                if (t === 'fixture') {
                  const x = Number(dec.x) || 0;
                  const y = Number(dec.y) || 0;
                  const w = Number(dec.w) || 100;
                  const h = Number(dec.h) || 72;
                  const lab = String(dec.label || '');
                  const kind = String(dec.kind || '');
                  return (
                    <Group
                      key={`d-${i}`}
                      x={x}
                      y={y}
                      listening={decorSelectable}
                      draggable={canEdit && sel}
                      onTap={(e) => {
                        e.cancelBubble = true;
                        selectDecOnly(i);
                      }}
                      onClick={(e) => {
                        e.cancelBubble = true;
                        selectDecOnly(i);
                      }}
                      onDragEnd={(e) => {
                        updateDecoration(i, { x: snap(e.target.x()), y: snap(e.target.y()) });
                      }}
                    >
                      <Rect
                        width={w}
                        height={h}
                        fill={sel ? 'rgba(251,191,36,0.35)' : 'rgba(251,191,36,0.18)'}
                        stroke="#fbbf24"
                        strokeWidth={sel ? 2 : 1}
                        cornerRadius={6}
                      />
                      <Text x={8} y={10} width={w - 16} text={lab || FIXTURE_LABELS[kind] || kind} fontSize={12} fill="#fef3c7" listening={false} />
                    </Group>
                  );
                }
                return null;
              })}
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
                const labelW = Math.max(rw, r) * 2;

                return (
                  <Group
                    key={t.id}
                    x={t.x}
                    y={t.y}
                    rotation={rot}
                    opacity={opacity}
                    draggable={tableInteractive}
                    listening={!editMode || tableInteractive}
                    onDragEnd={(e) => {
                      if (e.target !== e.currentTarget) return;
                      const node = e.target;
                      handleDragEnd(t.id, node.x(), node.y());
                    }}
                    onTap={() => {
                      if (!canClick && !editMode) return;
                      if (editMode) {
                        setSelectedDec(null);
                        clearWallArcPick();
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
                        setSelectedDec(null);
                        clearWallArcPick();
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
                        ref={
                          t.id === selectedId
                            ? (node) => {
                                tableShapeRef.current = node;
                              }
                            : undefined
                        }
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
                        draggable={false}
                        onTransformEnd={(e) => {
                          if (t.id !== selectedId) return;
                          const node = e.target as Konva.Rect;
                          const sx = node.scaleX();
                          const sy = node.scaleY();
                          const nw = Math.max(20, snap(node.width() * sx));
                          const nh = Math.max(20, snap(node.height() * sy));
                          node.width(nw);
                          node.height(nh);
                          node.scaleX(1);
                          node.scaleY(1);
                          setTables((prev) =>
                            prev.map((row) =>
                              row.id !== t.id ? row : { ...row, width: nw, height: nh, radius: Math.max(nw, nh) / 2 },
                            ),
                          );
                        }}
                      />
                    ) : isEllipseLike(t.shape) ? (
                      <Ellipse
                        ref={
                          t.id === selectedId
                            ? (node) => {
                                tableShapeRef.current = node;
                              }
                            : undefined
                        }
                        radiusX={rw}
                        radiusY={rh}
                        fill={fill}
                        stroke={stroke}
                        strokeWidth={strokeW}
                        shadowBlur={8}
                        shadowColor="rgba(0,0,0,0.35)"
                        draggable={false}
                        onTransformEnd={(e) => {
                          if (t.id !== selectedId) return;
                          const node = e.target as Konva.Ellipse;
                          const nrx = Math.max(12, snap(node.radiusX() * node.scaleX()));
                          const nry = Math.max(12, snap(node.radiusY() * node.scaleY()));
                          node.radiusX(nrx);
                          node.radiusY(nry);
                          node.scaleX(1);
                          node.scaleY(1);
                          setTables((prev) =>
                            prev.map((row) =>
                              row.id !== t.id ? row : { ...row, width: nrx * 2, height: nry * 2, radius: Math.max(nrx, nry) },
                            ),
                          );
                        }}
                      />
                    ) : (
                      <Circle
                        ref={
                          t.id === selectedId
                            ? (node) => {
                                tableShapeRef.current = node;
                              }
                            : undefined
                        }
                        radius={r}
                        fill={fill}
                        stroke={stroke}
                        strokeWidth={strokeW}
                        shadowBlur={8}
                        shadowColor="rgba(0,0,0,0.35)"
                        draggable={false}
                        onTransformEnd={(e) => {
                          if (t.id !== selectedId) return;
                          const node = e.target as Konva.Circle;
                          const nr = Math.max(14, snap(node.radius() * node.scaleX()));
                          node.radius(nr);
                          node.scaleX(1);
                          node.scaleY(1);
                          setTables((prev) =>
                            prev.map((row) =>
                              row.id !== t.id ? row : { ...row, radius: nr, width: nr * 2, height: nr * 2 },
                            ),
                          );
                        }}
                      />
                    )}
                    {(chairLayout[t.id] || []).map((ch, ci) => (
                      <Circle
                        key={`${t.id}-ch-${ci}`}
                        x={ch.dx}
                        y={ch.dy}
                        radius={8}
                        fill="#94a3b8"
                        stroke="#334155"
                        strokeWidth={1}
                        draggable={tableInteractive}
                        perfectDrawEnabled={false}
                        hitStrokeWidth={32}
                        listening={tableInteractive}
                        onDragEnd={(e) => {
                          const nx = snap(e.target.x());
                          const ny = snap(e.target.y());
                          setChairLayout((cl) => ({
                            ...cl,
                            [t.id]: (cl[t.id] || []).map((p, j) => (j === ci ? { dx: nx, dy: ny } : p)),
                          }));
                        }}
                      />
                    ))}
                    <Group listening={false}>
                      <Text
                        x={-labelW / 2}
                        y={-14}
                        width={labelW}
                        align="center"
                        text={String(t.number)}
                        fontSize={14}
                        fontStyle="bold"
                        fill="#f8fafc"
                        listening={false}
                      />
                      <Text
                        x={-labelW / 2}
                        y={4}
                        width={labelW}
                        align="center"
                        text={personsLabelRu(t.capacity)}
                        fontSize={10}
                        fill="rgba(248,250,252,0.9)"
                        listening={false}
                      />
                    </Group>
                  </Group>
                );
              })}
              {editMode && editorMode === 'interior' && placeMode === 'zone_fill' && (
                <Rect
                  name="zoneFillOverlay"
                  x={0}
                  y={0}
                  width={stageW}
                  height={stageH}
                  fill="rgba(15,23,42,0.04)"
                  listening
                  onMouseDown={(e) => {
                    e.cancelBubble = true;
                    handleFloorPointer(e);
                  }}
                />
              )}
              {editMode && editorMode === 'interior' && selectedId && placeMode === 'none' && (
                <Transformer
                  ref={tableTransformerRef}
                  rotateEnabled={false}
                  borderStroke="#e2e8f0"
                  keepRatio={false}
                />
              )}
            </Group>
          </Layer>
        </Stage>
      </div>
    </div>
  );
}
