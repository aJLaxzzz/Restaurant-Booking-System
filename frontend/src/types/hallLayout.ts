/**
 * Контракт layout_json (halls.layout_json): объект с полями tables (с сервера приходят отдельно из БД),
 * walls — сегменты стен, decorations — декор.
 * Новые типы декора: door, window (сегмент x1,y1,x2,y2), zone (прямоугольник + подпись).
 */
export type WallSegment = { x1: number; y1: number; x2: number; y2: number };

export type DecorationZoneLabel = {
  type: 'zone_label';
  text: string;
  x: number;
  y: number;
  w: number;
  h: number;
};

export type DecorationWindowBand = {
  type: 'window_band';
  x: number;
  y: number;
  w: number;
  h: number;
};

export type DecorationDoor = {
  type: 'door';
  x1: number;
  y1: number;
  x2: number;
  y2: number;
};

export type DecorationWindow = {
  type: 'window';
  x1: number;
  y1: number;
  x2: number;
  y2: number;
};

export type DecorationZone = {
  type: 'zone';
  x: number;
  y: number;
  w: number;
  h: number;
  label?: string;
};

export type HallDecoration =
  | DecorationZoneLabel
  | DecorationWindowBand
  | DecorationDoor
  | DecorationWindow
  | DecorationZone
  | Record<string, unknown>;
