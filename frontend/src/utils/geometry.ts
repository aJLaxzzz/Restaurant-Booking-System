/** Абсолютная площадь многоугольника [x0,y0,...] (замкнутый контур). */
export function polygonArea(poly: number[]): number {
  if (poly.length < 6) return Infinity;
  let a = 0;
  const n = poly.length;
  for (let i = 0; i < n; i += 2) {
    const j = (i + 2) % n;
    a += poly[i] * poly[j + 1] - poly[j] * poly[i + 1];
  }
  return Math.abs(a / 2);
}

/** Ray-casting point-in-polygon; poly is [x0,y0,x1,y1,...] closed polygon */
export function pointInPolygon(x: number, y: number, poly: number[]): boolean {
  if (poly.length < 6) return false;
  let inside = false;
  const m = poly.length;
  for (let i = 0; i < m; i += 2) {
    const xi = poly[i];
    const yi = poly[i + 1];
    const j = (i - 2 + m) % m;
    const xj = poly[j];
    const yj = poly[j + 1];
    const intersect = yi > y !== yj > y && x < ((xj - xi) * (y - yi)) / (yj - yi + 1e-12) + xi;
    if (intersect) inside = !inside;
  }
  return inside;
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

/** Мин. квадрат расстояния от точки до ломаной (граница полигона). */
export function minDistSqToPolygonBoundary(px: number, py: number, poly: number[]): number {
  if (poly.length < 6) return Infinity;
  let m = Infinity;
  const n = poly.length;
  for (let i = 0; i < n; i += 2) {
    const j = (i + 2) % n;
    const d = distSqPointSegment(px, py, poly[i], poly[i + 1], poly[j], poly[j + 1]);
    m = Math.min(m, d);
  }
  return m;
}

/**
 * Все вершины inner внутри outer или у границы outer (общие стороны вложенных прямоугольников).
 */
export function polygonVerticesInsideOrOnEdge(
  inner: number[],
  outer: number[],
  edgeTol = 10,
): boolean {
  if (inner.length < 6 || outer.length < 6) return false;
  const tol2 = edgeTol * edgeTol;
  for (let i = 0; i < inner.length; i += 2) {
    const x = inner[i];
    const y = inner[i + 1];
    if (pointInPolygon(x, y, outer)) continue;
    if (minDistSqToPolygonBoundary(x, y, outer) <= tol2) continue;
    return false;
  }
  return true;
}

export function polygonCentroid(poly: number[]): { x: number; y: number } {
  let sx = 0;
  let sy = 0;
  const n = poly.length / 2;
  for (let i = 0; i < poly.length; i += 2) {
    sx += poly[i];
    sy += poly[i + 1];
  }
  return { x: sx / n, y: sy / n };
}

/** Точка в кольце: внутри outer и ни в одной из дыр (замкнутые полигоны). */
export function pointInPolygonWithHoles(x: number, y: number, outer: number[], holes: number[][]): boolean {
  if (!pointInPolygon(x, y, outer)) return false;
  for (const h of holes) {
    if (h.length >= 6 && pointInPolygon(x, y, h)) return false;
  }
  return true;
}

/** Площадь «пончика»: outer минус дыры. */
export function effectivePolygonArea(outer: number[], holes: number[][]): number {
  let a = polygonArea(outer);
  for (const h of holes) {
    a -= polygonArea(h);
  }
  return Math.max(0, a);
}

function vertexInsideOrNearOuterBoundary(x: number, y: number, outer: number[], tol2: number): boolean {
  if (pointInPolygon(x, y, outer)) return true;
  return minDistSqToPolygonBoundary(x, y, outer) <= tol2;
}

/**
 * Внутренняя зона для «дырки»: общая стена с внешней часто даёт 1–2 вершины «снаружи» для ray-cast;
 * центроид при этом остаётся внутри / у границы.
 */
function polygonQualifiesAsHoleInOuter(
  inner: number[],
  outer: number[],
  outerArea: number,
  edgeTol = 22,
): boolean {
  if (inner.length < 6 || outer.length < 6) return false;
  const innerArea = polygonArea(inner);
  if (!Number.isFinite(innerArea) || !Number.isFinite(outerArea)) return false;
  if (innerArea < 1) return false;
  if (innerArea >= outerArea) return false;

  if (polygonVerticesInsideOrOnEdge(inner, outer, edgeTol)) return true;

  const tol2 = edgeTol * edgeTol;
  const { x: cx, y: cy } = polygonCentroid(inner);
  if (
    !pointInPolygon(cx, cy, outer) &&
    minDistSqToPolygonBoundary(cx, cy, outer) > tol2
  ) {
    return false;
  }

  const nv = inner.length / 2;
  let near = 0;
  for (let i = 0; i < inner.length; i += 2) {
    if (vertexInsideOrNearOuterBoundary(inner[i], inner[i + 1], outer, tol2)) near++;
  }
  return near >= Math.ceil(nv * 0.6);
}

export type ZonePolyRef = { poly: number[]; area: number };

/** Полигоны вложенных zone_named, которые задают «дырку» в outer (не содержатся целиком в другом кандидате внутри outer). */
export function zoneNamedDirectHolePolys(outer: number[], outerArea: number, others: ZonePolyRef[]): number[][] {
  const inside = others.filter((o) => polygonQualifiesAsHoleInOuter(o.poly, outer, outerArea));
  return inside
    .filter(
      (h) => !inside.some((k) => k !== h && polygonQualifiesAsHoleInOuter(h.poly, k.poly, k.area)),
    )
    .map((x) => x.poly);
}

/** Обратный обход вершин (для subpath-дыры и even-odd при общих рёбрах). */
export function reversePolygonRingFlat(poly: number[]): number[] {
  const out: number[] = [];
  for (let i = poly.length - 2; i >= 0; i -= 2) {
    out.push(poly[i], poly[i + 1]);
  }
  return out;
}
