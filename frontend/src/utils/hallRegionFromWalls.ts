/** Полигон замкнутой области по сетке: стены = барьер, край холста = барьер, BFS от клика, контур компоненты. */

import { polygonArea } from './geometry';

type Wall = Record<string, number>;
type WallArc = { type: 'arc'; cx: number; cy: number; r: number; a0: number; a1: number };
type WallQuad = { type: 'quad'; x1: number; y1: number; qx: number; qy: number; x2: number; y2: number };

function distSqPointSegment(px: number, py: number, x1: number, y1: number, x2: number, y2: number): number {
  const dx = x2 - x1;
  const dy = y2 - y1;
  const len2 = dx * dx + dy * dy;
  if (len2 < 1e-8) {
    const ex = px - x1;
    const ey = py - y1;
    return ex * ex + ey * ey;
  }
  let t = ((px - x1) * dx + (py - y1) * dy) / len2;
  t = Math.max(0, Math.min(1, t));
  const qx = x1 + t * dx;
  const qy = y1 + t * dy;
  const ex = px - qx;
  const ey = py - qy;
  return ex * ex + ey * ey;
}

/** Расстояние до отрезка, продолженного на `cap` с обоих концов — закрывает микро-щели в углах и у стыков «внутренняя стена / периметр». */
function distSqPointSegmentCapped(
  px: number,
  py: number,
  x1: number,
  y1: number,
  x2: number,
  y2: number,
  cap: number,
): number {
  const dx = x2 - x1;
  const dy = y2 - y1;
  const len = Math.hypot(dx, dy);
  if (len < 1e-8) {
    const ex = px - x1;
    const ey = py - y1;
    return ex * ex + ey * ey;
  }
  const ux = dx / len;
  const uy = dy / len;
  const ax = x1 - ux * cap;
  const ay = y1 - uy * cap;
  const bx = x2 + ux * cap;
  const by = y2 + uy * cap;
  return distSqPointSegment(px, py, ax, ay, bx, by);
}

function sampleQuad(x1: number, y1: number, qx: number, qy: number, x2: number, y2: number, steps: number) {
  const pts: { x: number; y: number }[] = [];
  for (let i = 0; i <= steps; i++) {
    const t = i / steps;
    const omt = 1 - t;
    const x = omt * omt * x1 + 2 * omt * t * qx + t * t * x2;
    const y = omt * omt * y1 + 2 * omt * t * qy + t * t * y2;
    pts.push({ x, y });
  }
  return pts;
}

function segmentsFromGeometry(
  walls: Wall[],
  wallArcs: WallArc[],
  wallQuads: WallQuad[],
): Array<{ x1: number; y1: number; x2: number; y2: number }> {
  const segs: Array<{ x1: number; y1: number; x2: number; y2: number }> = [];
  for (const w of walls) {
    segs.push({ x1: w.x1, y1: w.y1, x2: w.x2, y2: w.y2 });
  }
  for (const q of wallQuads) {
    const pts = sampleQuad(q.x1, q.y1, q.qx, q.qy, q.x2, q.y2, 24);
    for (let i = 0; i < pts.length - 1; i++) {
      segs.push({ x1: pts[i].x, y1: pts[i].y, x2: pts[i + 1].x, y2: pts[i + 1].y });
    }
  }
  for (const a of wallArcs) {
    const steps = Math.max(16, Math.ceil((Math.abs(a.a1 - a.a0) * a.r) / 40));
    for (let i = 0; i < steps; i++) {
      const t0 = a.a0 + ((a.a1 - a.a0) * i) / steps;
      const t1 = a.a0 + ((a.a1 - a.a0) * (i + 1)) / steps;
      segs.push({
        x1: a.cx + a.r * Math.cos(t0),
        y1: a.cy + a.r * Math.sin(t0),
        x2: a.cx + a.r * Math.cos(t1),
        y2: a.cy + a.r * Math.sin(t1),
      });
    }
  }
  return segs;
}

function cellBlocked(
  cx: number,
  cy: number,
  segs: Array<{ x1: number; y1: number; x2: number; y2: number }>,
  wallR2: number,
  stageW: number,
  stageH: number,
  segmentCap: number,
): boolean {
  if (cx < 0 || cy < 0 || cx > stageW || cy > stageH) return true;
  const edge = 2;
  if (cx <= edge || cy <= edge || cx >= stageW - edge || cy >= stageH - edge) return true;
  for (const s of segs) {
    if (distSqPointSegmentCapped(cx, cy, s.x1, s.y1, s.x2, s.y2, segmentCap) <= wallR2) return true;
  }
  return false;
}

const ptKey = (x: number, y: number) => `${Math.round(x * 100) / 100},${Math.round(y * 100) / 100}`;

function parseKey(k: string): { x: number; y: number } {
  const [xs, ys] = k.split(',');
  return { x: parseFloat(xs), y: parseFloat(ys) };
}

/** Контур компоненты сетки: рёбра между посещённой и непосещённой ячейкой. */
function boundaryPolygonFromVisited(
  visited: Set<string>,
  nx: number,
  ny: number,
  cs: number,
): number[] | null {
  type Edge = { a: string; b: string };
  const edges: Edge[] = [];
  const vis = (i: number, j: number) => i >= 0 && j >= 0 && i < nx && j < ny && visited.has(`${i},${j}`);

  for (let j = 0; j < ny; j++) {
    for (let i = 0; i < nx; i++) {
      if (!visited.has(`${i},${j}`)) continue;
      const x0 = i * cs;
      const y0 = j * cs;
      const x1 = (i + 1) * cs;
      const y1 = (j + 1) * cs;
      if (!vis(i, j - 1)) {
        edges.push({ a: ptKey(x0, y0), b: ptKey(x1, y0) });
      }
      if (!vis(i + 1, j)) {
        edges.push({ a: ptKey(x1, y0), b: ptKey(x1, y1) });
      }
      if (!vis(i, j + 1)) {
        edges.push({ a: ptKey(x1, y1), b: ptKey(x0, y1) });
      }
      if (!vis(i - 1, j)) {
        edges.push({ a: ptKey(x0, y1), b: ptKey(x0, y0) });
      }
    }
  }

  if (edges.length === 0) return null;

  const adj = new Map<string, string[]>();
  for (const { a, b } of edges) {
    if (!adj.has(a)) adj.set(a, []);
    if (!adj.has(b)) adj.set(b, []);
    adj.get(a)!.push(b);
    adj.get(b)!.push(a);
  }

  const start = edges[0].a;
  let prev: string | null = null;
  let cur = start;
  const pathKeys: string[] = [cur];
  const maxSteps = edges.length * 6 + 20;
  for (let step = 0; step < maxSteps; step++) {
    const neigh = adj.get(cur);
    if (!neigh || neigh.length === 0) break;
    const next = neigh.find((n) => n !== prev);
    if (next == null) break;
    pathKeys.push(next);
    prev = cur;
    cur = next;
    if (cur === start && pathKeys.length > 2) break;
  }

  if (pathKeys.length < 4) return null;

  const poly: number[] = [];
  for (let i = 0; i < pathKeys.length - 1; i++) {
    const { x, y } = parseKey(pathKeys[i]);
    poly.push(x, y);
  }
  if (poly.length >= 6) return poly;
  return null;
}

/**
 * Возвращает замкнутый полигон области, в которой лежит (worldX, worldY), ограниченной стенами и краем холста.
 */
export function floodRegionPolygonFromWalls(
  worldX: number,
  worldY: number,
  stageW: number,
  stageH: number,
  walls: Wall[],
  wallArcs: WallArc[],
  wallQuads: WallQuad[],
): number[] | null {
  const cs = 12;
  const wallR = 4;
  const wallR2 = wallR * wallR;
  /** Продление сегмента для hit-test — устраняет протекание BFS в углах и при общих сторонах вложенных прямоугольников. */
  const segmentCap = Math.max(10, wallR * 2.5);
  const nx = Math.max(1, Math.floor(stageW / cs));
  const ny = Math.max(1, Math.floor(stageH / cs));
  const segs = segmentsFromGeometry(walls, wallArcs, wallQuads);
  /** Без сегментов стен остаётся только край холста — BFS зальёт почти весь лист («зона по сетке»). */
  if (segs.length === 0) return null;

  let ix = Math.floor(worldX / cs);
  let iy = Math.floor(worldY / cs);
  ix = Math.max(0, Math.min(nx - 1, ix));
  iy = Math.max(0, Math.min(ny - 1, iy));

  const startCx = (ix + 0.5) * cs;
  const startCy = (iy + 0.5) * cs;
  if (cellBlocked(startCx, startCy, segs, wallR2, stageW, stageH, segmentCap)) {
    const dirs = [
      [0, 0],
      [1, 0],
      [-1, 0],
      [0, 1],
      [0, -1],
      [1, 1],
      [1, -1],
      [-1, 1],
      [-1, -1],
    ];
    let found = false;
    for (const [dx, dy] of dirs) {
      const jx = Math.max(0, Math.min(nx - 1, ix + dx));
      const jy = Math.max(0, Math.min(ny - 1, iy + dy));
      const cx = (jx + 0.5) * cs;
      const cy = (jy + 0.5) * cs;
      if (!cellBlocked(cx, cy, segs, wallR2, stageW, stageH, segmentCap)) {
        ix = jx;
        iy = jy;
        found = true;
        break;
      }
    }
    if (!found) return null;
  }

  const visited = new Set<string>();
  const q: [number, number][] = [[ix, iy]];
  visited.add(`${ix},${iy}`);
  while (q.length > 0) {
    const [ci, cj] = q.shift()!;
    for (const [di, dj] of [
      [1, 0],
      [-1, 0],
      [0, 1],
      [0, -1],
    ]) {
      const ni = ci + di;
      const nj = cj + dj;
      if (ni < 0 || nj < 0 || ni >= nx || nj >= ny) continue;
      const k = `${ni},${nj}`;
      if (visited.has(k)) continue;
      const cx = (ni + 0.5) * cs;
      const cy = (nj + 0.5) * cs;
      if (cellBlocked(cx, cy, segs, wallR2, stageW, stageH, segmentCap)) continue;
      visited.add(k);
      q.push([ni, nj]);
    }
  }

  if (visited.size < 2) return null;

  const gridCells = nx * ny;
  /** Незамкнутый периметр: вода заполняет почти всю сетку — не возвращаем полигон «на весь чертёж». */
  if (visited.size > gridCells * 0.86) return null;

  const poly = boundaryPolygonFromVisited(visited, nx, ny, cs);
  if (!poly || poly.length < 6) return null;
  const a = polygonArea(poly);
  if (a / (stageW * stageH) > 0.9) return null;

  return poly;
}
