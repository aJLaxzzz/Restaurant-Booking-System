/** Публичный блок рейтинга: звёзды (дробная заливка) + число + число оценок (read-only). */

const FIVE_STARS = '★★★★★';

type Props = {
  ratingAvg: number | null | undefined;
  ratingCount?: number | null;
  className?: string;
  /** компактная строка для карточек на главной */
  compact?: boolean;
};

export function RestaurantRatingStars({ ratingAvg, ratingCount, className = '', compact }: Props) {
  const count = typeof ratingCount === 'number' ? ratingCount : 0;
  const has =
    typeof ratingAvg === 'number' &&
    Number.isFinite(ratingAvg) &&
    count > 0;

  if (!has) {
    return (
      <p className={`restaurant-rating-stars-empty muted compact ${className}`.trim()}>Нет оценок</p>
    );
  }

  const clamped = Math.min(5, Math.max(0, ratingAvg));
  const fillPct = (clamped / 5) * 100;
  const oc =
    count === 1 ? 'оценка' : count > 1 && count < 5 ? 'оценки' : 'оценок';

  return (
    <div
      className={`restaurant-rating-stars ${compact ? 'restaurant-rating-stars--compact' : ''} ${className}`.trim()}
      role="img"
      aria-label={`Рейтинг ${ratingAvg.toFixed(1)} из 5, ${count} ${oc}`}
    >
      <span className="restaurant-rating-stars-track" aria-hidden>
        <span className="restaurant-rating-stars-bg">{FIVE_STARS}</span>
        <span className="restaurant-rating-stars-fill-clip" style={{ width: `${fillPct}%` }}>
          <span className="restaurant-rating-stars-fill-inner">{FIVE_STARS}</span>
        </span>
      </span>
      {!compact && (
        <span className="restaurant-rating-stars-num">{ratingAvg.toFixed(1)}</span>
      )}
      <span className="restaurant-rating-stars-meta muted">
        {compact ? `${ratingAvg.toFixed(1)} · ` : ''}
        {count} {oc}
      </span>
    </div>
  );
}
