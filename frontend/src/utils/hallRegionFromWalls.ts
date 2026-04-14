/** Зона по клику: связная компонента свободных клеток (стены + край холста = барьер), контур — ортогональная обводка (внешний контур + дыры при «пончике»). */

import { pointInPolygon, polygonArea } from './geometry';

type Wall = Record<string, number>;
type WallArc = { type: 'arc'; cx: number; cy: number; r: number; a0: number; a1: number };
type WallQuad = { type: 'quad'; x1: number; y1: number; qx: number; qy: number; x2: number; y2: number };

export type FloodRegionPolygon = {
  outer: number[];
  holes: number[][];
};

function clamp(n: number, a: number, b: number) {
  return Math.max(a, Math.min(b, n));
}

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
    const pts = sampleQuad(q.x1, q.y1, q.qx, q.qy, q.x2, q.y2, 48);
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

function cornerKey(gi: number, gj: number): string {
  return `${gi},${gj}`;
}

function buildBoundaryAdj(visited: Set<string>, nx: number, ny: number): Map<string, string[]> {
  const vis = (i: number, j: number) => i >= 0 && j >= 0 && i < nx && j < ny && visited.has(`${i},${j}`);

  const adj = new Map<string, string[]>();
  const addEdge = (a: string, b: string) => {
    if (a === b) return;
    if (!adj.has(a)) adj.set(a, []);
    if (!adj.has(b)) adj.set(b, []);
    const la = adj.get(a)!;
    if (!la.includes(b)) la.push(b);
    const lb = adj.get(b)!;
    if (!lb.includes(a)) lb.push(a);
  };

  for (let j = 0; j < ny; j++) {
    for (let i = 0; i < nx; i++) {
      if (!visited.has(`${i},${j}`)) continue;
      const x0 = i;
      const y0 = j;
      const x1 = i + 1;
      const y1 = j + 1;
      if (!vis(i, j - 1)) {
        addEdge(cornerKey(x0, y0), cornerKey(x1, y0));
      }
      if (!vis(i + 1, j)) {
        addEdge(cornerKey(x1, y0), cornerKey(x1, y1));
      }
      if (!vis(i, j + 1)) {
        addEdge(cornerKey(x1, y1), cornerKey(x0, y1));
      }
      if (!vis(i - 1, j)) {
        addEdge(cornerKey(x0, y1), cornerKey(x0, y0));
      }
    }
  }
  return adj;
}

function connectedComponents(adj: Map<string, string[]>): string[][] {
  const seen = new Set<string>();
  const out: string[][] = [];
  for (const v of adj.keys()) {
    if (seen.has(v)) continue;
    const comp: string[] = [];
    const stack = [v];
    seen.add(v);
    while (stack.length) {
      const cur = stack.pop()!;
      comp.push(cur);
      for (const u of adj.get(cur) || []) {
        if (!seen.has(u)) {
          seen.add(u);
          stack.push(u);
        }
      }
    }
    out.push(comp);
  }
  return out;
}

/**
 * Один простой цикл по компоненте граничного графа: у каждой вершины степень 2.
 */
function extractSimpleCycle(adj: Map<string, string[]>, comp: string[]): string[] | null {
  const set = new Set(comp);
  for (const v of comp) {
    const deg = (adj.get(v) || []).filter((u) => set.has(u)).length;
    if (deg !== 2) return null;
  }
  if (comp.length < 3) return null;

  const start = [...comp].sort((a, b) => a.localeCompare(b))[0];
  const nbs = (adj.get(start) || []).filter((u) => set.has(u)).sort((a, b) => a.localeCompare(b));
  if (nbs.length !== 2) return null;

  for (const first of nbs) {
    const path: string[] = [start];
    let prev = start;
    let cur = first;
    for (let iter = 0; iter < comp.length + 8; iter++) {
      const nextOpts = (adj.get(cur) || []).filter((u) => u !== prev);
      if (nextOpts.length !== 1) break;
      const next = nextOpts[0];
      if (next === start) {
        path.push(cur);
        if (path.length === comp.length) return path;
        break;
      }
      path.push(cur);
      prev = cur;
      cur = next;
    }
  }
  return null;
}

function keysPathToWorldPoly(path: string[], cs: number): number[] {
  const poly: number[] = [];
  for (const k of path) {
    const [gi, gj] = k.split(',').map(Number);
    poly.push(gi * cs, gj * cs);
  }
  return poly;
}

function dedupeConsecutiveVertices(poly: number[]): number[] {
  if (poly.length < 6) return poly;
  const out: number[] = [];
  for (let i = 0; i < poly.length; i += 2) {
    const x = poly[i];
    const y = poly[i + 1];
    const ox = out[out.length - 2];
    const oy = out[out.length - 1];
    if (out.length >= 2 && x === ox && y === oy) continue;
    out.push(x, y);
  }
  if (out.length >= 6) {
    const x0 = out[0];
    const y0 = out[1];
    const xl = out[out.length - 2];
    const yl = out[out.length - 1];
    if (x0 === xl && y0 === yl) out.splice(out.length - 2, 2);
  }
  return out;
}

function removeCollinearOrthogonal(poly: number[]): number[] {
  // Для ортогонального контура: удаляем вершины, которые не меняют направление (A->B->C на одной линии).
  if (poly.length < 6) return poly;
  const pts = dedupeConsecutiveVertices(poly);
  if (pts.length < 6) return pts;
  const out: number[] = [];
  const n = pts.length / 2;
  for (let i = 0; i < n; i++) {
    const ax = pts[((i - 1 + n) % n) * 2];
    const ay = pts[((i - 1 + n) % n) * 2 + 1];
    const bx = pts[i * 2];
    const by = pts[i * 2 + 1];
    const cx = pts[((i + 1) % n) * 2];
    const cy = pts[((i + 1) % n) * 2 + 1];
    const abx = bx - ax;
    const aby = by - ay;
    const bcx = cx - bx;
    const bcy = cy - by;
    const collinear = (abx === 0 && bcx === 0) || (aby === 0 && bcy === 0);
    if (collinear) continue;
    out.push(bx, by);
  }
  return out.length >= 6 ? out : pts;
}

function collapseOneCellZigzags(poly: number[], cs: number): number[] {
  // Схлопываем микро-зигзаги вида: ... A -> B -> C -> D ..., где B и C — "ступенька" на 1 клетку.
  // Это типичный артефакт границы клеточной компоненты.
  if (poly.length < 10) return poly;
  const pts = removeCollinearOrthogonal(poly);
  if (pts.length < 10) return pts;
  const n = pts.length / 2;
  const keep = new Array<boolean>(n).fill(true);
  const isStep = (dx: number, dy: number) => Math.abs(dx) === cs && dy === 0 || Math.abs(dy) === cs && dx === 0;
  for (let i = 0; i < n; i++) {
    const ax = pts[((i - 1 + n) % n) * 2];
    const ay = pts[((i - 1 + n) % n) * 2 + 1];
    const bx = pts[i * 2];
    const by = pts[i * 2 + 1];
    const cx = pts[((i + 1) % n) * 2];
    const cy = pts[((i + 1) % n) * 2 + 1];
    const dx = pts[((i + 2) % n) * 2];
    const dy = pts[((i + 2) % n) * 2 + 1];
    const abx = bx - ax;
    const aby = by - ay;
    const bcx = cx - bx;
    const bcy = cy - by;
    const cdx = dx - cx;
    const cdy = dy - cy;
    // A->B and C->D are same direction; B->C is a single-cell orthogonal detour.
    const sameDir = (abx === cdx && aby === cdy);
    const detourOrth = (abx === 0 && bcy === 0) || (aby === 0 && bcx === 0);
    if (sameDir && detourOrth && isStep(bcx, bcy)) {
      // drop B and C; connect A->D
      keep[i] = false;
      keep[(i + 1) % n] = false;
    }
  }
  const out: number[] = [];
  for (let i = 0; i < n; i++) {
    if (!keep[i]) continue;
    out.push(pts[i * 2], pts[i * 2 + 1]);
  }
  return out.length >= 6 ? removeCollinearOrthogonal(out) : pts;
}

function simplifyFloodBoundary(poly: number[], cs: number): number[] {
  let out = dedupeConsecutiveVertices(poly);
  out = removeCollinearOrthogonal(out);
  out = collapseOneCellZigzags(out, cs);
  out = dedupeConsecutiveVertices(out);
  out = removeCollinearOrthogonal(out);
  return out;
}

/**
 * Внешний контур (макс. площадь) и внутренние границы дыр ортогональной компоненты клеток.
 */
function boundaryPolygonsFromVisited(
  visited: Set<string>,
  nx: number,
  ny: number,
  cs: number,
): FloodRegionPolygon | null {
  const adj = buildBoundaryAdj(visited, nx, ny);
  if (adj.size === 0) return null;

  const comps = connectedComponents(adj);
  const polys: number[][] = [];

  for (const comp of comps) {
    const cyc = extractSimpleCycle(adj, comp);
    if (!cyc) {
      return null;
    }
    const poly = keysPathToWorldPoly(cyc, cs);
    if (poly.length >= 6) polys.push(poly);
  }

  if (polys.length === 0) return null;
  polys.sort((a, b) => polygonArea(b) - polygonArea(a));
  return { outer: polys[0], holes: polys.slice(1) };
}

function bboxPolygonFromVisited(visited: Set<string>, cs: number): number[] | null {
  let mini = Infinity,
    maxi = -Infinity,
    minj = Infinity,
    maxj = -Infinity;
  for (const k of visited) {
    const [si, sj] = k.split(',').map(Number);
    if (!Number.isFinite(si) || !Number.isFinite(sj)) continue;
    mini = Math.min(mini, si);
    maxi = Math.max(maxi, si);
    minj = Math.min(minj, sj);
    maxj = Math.max(maxj, sj);
  }
  if (!Number.isFinite(mini) || maxi < mini || maxj < minj) return null;
  const x0 = mini * cs;
  const y0 = minj * cs;
  const x1 = (maxi + 1) * cs;
  const y1 = (maxj + 1) * cs;
  return [x0, y0, x1, y0, x1, y1, x0, y1];
}

/**
 * Полигон связной области, в которой лежит клик: BFS по клеткам, барьеры — стены и край холста.
 * roomClipPoly — дополнительное ограничение (контур помещения), чтобы не перетекать в соседнюю комнату.
 */
export function floodRegionPolygonFromWalls(
  worldX: number,
  worldY: number,
  stageW: number,
  stageH: number,
  walls: Wall[],
  wallArcs: WallArc[],
  wallQuads: WallQuad[],
  roomClipPoly: number[] | null = null,
): FloodRegionPolygon | null {
  // Точность и стабильность:
  // - cs меньше => меньше "лесенка" и ближе к стенам, но больше клеток для BFS.
  // - wallR/segmentCap подстраиваем под cs, чтобы барьер вокруг стен был визуально ровным.
  const cs = clamp(Math.round(Math.min(stageW, stageH) / 120), 6, 10); // обычно 6–10px
  const wallR = Math.max(3, Math.round(cs / 2.5));
  const wallR2 = wallR * wallR;
  const segmentCap = Math.max(cs, Math.round(wallR * 2.75));
  const nx = Math.max(1, Math.floor(stageW / cs));
  const ny = Math.max(1, Math.floor(stageH / cs));
  const segs = segmentsFromGeometry(walls, wallArcs, wallQuads);

  const inRoom = (cx: number, cy: number) =>
    !roomClipPoly || roomClipPoly.length < 6 || pointInPolygon(cx, cy, roomClipPoly);

  let ix = Math.floor(worldX / cs);
  let iy = Math.floor(worldY / cs);
  ix = Math.max(0, Math.min(nx - 1, ix));
  iy = Math.max(0, Math.min(ny - 1, iy));

  const startCx = (ix + 0.5) * cs;
  const startCy = (iy + 0.5) * cs;
  if (!inRoom(startCx, startCy) || cellBlocked(startCx, startCy, segs, wallR2, stageW, stageH, segmentCap)) {
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
      if (inRoom(cx, cy) && !cellBlocked(cx, cy, segs, wallR2, stageW, stageH, segmentCap)) {
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
      if (!inRoom(cx, cy) || cellBlocked(cx, cy, segs, wallR2, stageW, stageH, segmentCap)) continue;
      visited.add(k);
      q.push([ni, nj]);
    }
  }

  if (visited.size < 2) return null;

  let region = boundaryPolygonsFromVisited(visited, nx, ny, cs);
  if (!region) {
    const bbox = bboxPolygonFromVisited(visited, cs);
    if (!bbox || bbox.length < 6) return null;
    region = { outer: bbox, holes: [] };
  }

  const outer = simplifyFloodBoundary(region.outer, cs);
  const holes = region.holes.map((h) => simplifyFloodBoundary(h, cs)).filter((h) => h.length >= 6);
  let eff = polygonArea(outer);
  for (const h of holes) eff -= polygonArea(h);
  if (eff <= 0 || eff / (stageW * stageH) > 0.9995) return null;

  return { outer, holes };
}
