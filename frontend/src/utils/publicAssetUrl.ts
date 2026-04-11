/** Без завершающего слэша. Пусто — относительные пути как с API. */
const PUBLIC_ORIGIN = (import.meta.env.VITE_PUBLIC_ORIGIN as string | undefined)?.replace(/\/$/, '') ?? '';

/**
 * Склеивает абсолютный URL для статики (/demo/..., /api/files/...) если задан VITE_PUBLIC_ORIGIN
 * (например фронт на CDN, картинки на другом домене или единый origin в Docker).
 */
export function resolvePublicImageUrl(path: string): string {
  if (!path || typeof path !== 'string') return '';
  const p = path.trim();
  if (p === '') return '';
  if (/^https?:\/\//i.test(p)) return p;
  if (!PUBLIC_ORIGIN) return p;
  if (p.startsWith('/')) return `${PUBLIC_ORIGIN}${p}`;
  return `${PUBLIC_ORIGIN}/${p}`;
}
