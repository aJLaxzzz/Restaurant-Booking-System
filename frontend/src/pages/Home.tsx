import { useCallback, useEffect, useLayoutEffect, useMemo, useState } from 'react';
import { Link, useLocation } from 'react-router-dom';
import { useAuth } from '../auth';
import { api } from '../api';
import { resolvePublicImageUrl } from '../utils/publicAssetUrl';
import { RestaurantsMap } from '../components/RestaurantsMap';
import { RestaurantRatingStars } from '../components/RestaurantRatingStars';

type Restaurant = {
  id: string;
  name: string;
  slug: string;
  city: string;
  description: string;
  photo_url: string;
  address?: string;
  phone?: string;
  opens_at?: string;
  closes_at?: string;
  rating_avg?: number | null;
  rating_count?: number;
  lat?: number | null;
  lng?: number | null;
};

function scrollToVenues() {
  document.getElementById('home-venues')?.scrollIntoView({ behavior: 'smooth', block: 'start' });
}

function displayVenueName(r: { name: string; slug?: string }) {
  const n = (r.name || '').trim();
  if (n) return n;
  const s = (r.slug || '').trim();
  if (s) return s.replace(/-/g, ' ');
  return 'Ресторан';
}

export default function Home() {
  const { user } = useAuth();
  const location = useLocation();
  const [venues, setVenues] = useState<Restaurant[]>([]);
  const [loading, setLoading] = useState(true);
  const [cityFilter, setCityFilter] = useState<string>('');
  const [mapMode, setMapMode] = useState(false);

  const cities = useMemo(() => {
    const s = new Set<string>();
    for (const v of venues) {
      const c = (v.city || '').trim();
      if (c) s.add(c);
    }
    return [...s].sort((a, b) => a.localeCompare(b, 'ru'));
  }, [venues]);

  const filteredVenues = useMemo(() => {
    if (!cityFilter) return venues;
    return venues.filter((v) => (v.city || '').trim() === cityFilter);
  }, [venues, cityFilter]);

  const cityCenter = useMemo((): { lat: number; lng: number; zoom: number } => {
    // Если город не выбран (показываем все города) — не усредняем координаты,
    // иначе центр получается «между» Москвой и СПб (примерно Тверь).
    const cityDefaults: Record<string, { lat: number; lng: number; zoom: number }> = {
      Москва: { lat: 55.7558, lng: 37.6173, zoom: 11 },
      'Санкт-Петербург': { lat: 59.9386, lng: 30.3141, zoom: 11 },
    };

    if (!cityFilter) {
      return cityDefaults['Москва'];
    }

    const list = filteredVenues
      .map((v) => ({ lat: v.lat, lng: v.lng }))
      .filter((x): x is { lat: number; lng: number } => typeof x.lat === 'number' && typeof x.lng === 'number');
    if (list.length === 0) {
      return cityDefaults[cityFilter] ?? cityDefaults['Москва'];
    }
    const lat = list.reduce((s, p) => s + p.lat, 0) / list.length;
    const lng = list.reduce((s, p) => s + p.lng, 0) / list.length;
    // На уровне города обычно хватает 12–13.
    return { lat, lng, zoom: 12 };
  }, [filteredVenues, cityFilter]);

  const mapPins = useMemo(() => {
    return filteredVenues
      .filter((v) => typeof v.lat === 'number' && typeof v.lng === 'number')
      .map((v) => ({
        id: v.id,
        name: displayVenueName(v),
        city: v.city,
        address: v.address,
        lat: v.lat as number,
        lng: v.lng as number,
      }));
  }, [filteredVenues]);

  const loadVenues = useCallback(async () => {
    setLoading(true);
    try {
      const { data } = await api.get<Restaurant[]>('/restaurants');
      setVenues(Array.isArray(data) ? data : []);
    } catch {
      setVenues([]);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadVenues();
  }, [location.key, loadVenues]);

  useEffect(() => {
    const onVis = () => {
      if (document.visibilityState === 'visible') void loadVenues();
    };
    document.addEventListener('visibilitychange', onVis);
    return () => document.removeEventListener('visibilitychange', onVis);
  }, [loadVenues]);

  useLayoutEffect(() => {
    if (location.hash === '#home-venues') {
      requestAnimationFrame(() => scrollToVenues());
    }
  }, [location.hash, location.pathname]);

  const countClass =
    filteredVenues.length <= 1 ? ' home-venue-grid--sparse' : '';

  return (
    <div className="home-landing">
      <section className="home-hero-pro" aria-labelledby="home-hero-title">
        <div className="home-hero-glow" aria-hidden />
        <div className="home-hero-inner">
          <div className="home-hero-copy">
            <span className="home-pill">Бронирование онлайн</span>
            <h1 id="home-hero-title" className="home-hero-title">
              Столик в ресторане — <span className="home-hero-accent">без звонков и ожидания</span>
            </h1>
            <p className="home-hero-lead">
              Начните с главной: выберите ресторан в списке ниже, откройте карточку — там меню и кнопка брони. Зона и стол — на
              интерактивной схеме зала.
            </p>
            <div className="home-hero-actions">
              {user?.role === 'client' || user?.role === 'owner' ? (
                <button type="button" className="btn home-hero-cta" onClick={() => scrollToVenues()}>
                  Забронировать стол
                </button>
              ) : (
                <Link to="/login" className="btn home-hero-cta">
                  Войти для брони
                </Link>
              )}
              {!user && (
                <Link to="/register" className="btn secondary home-hero-secondary">
                  Создать аккаунт
                </Link>
              )}
              {(user?.role === 'client' || user?.role === 'owner') && (
                <Link to="/me" className="btn secondary home-hero-secondary">
                  Мои брони
                </Link>
              )}
            </div>
            <ul className="home-hero-checks">
              <li>Схема зала в реальном времени</li>
              <li>Гибкие правила отмены</li>
              <li>История визитов в профиле</li>
            </ul>
          </div>

          <div className="home-hero-aside" aria-hidden>
            <div className="home-hero-card">
              <div className="home-hero-card-head">
                <span className="home-hero-card-dot" />
                Популярное сегодня
              </div>
              <div className="home-hero-card-rows">
                <div className="home-hero-faux-row">
                  <span className="home-hero-faux-thumb" />
                  <span>
                    <span className="home-hero-faux-line home-hero-faux-line--title" />
                    <span className="home-hero-faux-line home-hero-faux-line--sub" />
                  </span>
                </div>
                <div className="home-hero-faux-row">
                  <span className="home-hero-faux-thumb home-hero-faux-thumb--b" />
                  <span>
                    <span className="home-hero-faux-line home-hero-faux-line--title" />
                    <span className="home-hero-faux-line home-hero-faux-line--sub" />
                  </span>
                </div>
                <div className="home-hero-faux-row">
                  <span className="home-hero-faux-thumb home-hero-faux-thumb--c" />
                  <span>
                    <span className="home-hero-faux-line home-hero-faux-line--title" />
                    <span className="home-hero-faux-line home-hero-faux-line--sub" />
                  </span>
                </div>
              </div>
              <div className="home-hero-card-foot">
                <span className="home-hero-spark" />
                У окна · тихий зал · терраса
              </div>
            </div>
            <div className="home-hero-badges">
              <span className="home-float-badge home-float-badge--a">Депозит</span>
              <span className="home-float-badge home-float-badge--b">Меню</span>
              <span className="home-float-badge home-float-badge--c">Напоминание</span>
            </div>
          </div>
        </div>
      </section>

      <section className="home-strip" aria-label="Возможности">
        <div className="home-strip-inner">
          <div className="home-strip-item">
            <span className="home-strip-icon" aria-hidden>
              ◈
            </span>
            <span>Интерактивный зал</span>
          </div>
          <div className="home-strip-item">
            <span className="home-strip-icon" aria-hidden>
              ✦
            </span>
            <span>Оплата брони</span>
          </div>
          <div className="home-strip-item">
            <span className="home-strip-icon" aria-hidden>
              ◎
            </span>
            <span>Меню до визита</span>
          </div>
          <div className="home-strip-item">
            <span className="home-strip-icon" aria-hidden>
              ✓
            </span>
            <span>Мои брони</span>
          </div>
        </div>
      </section>

      <section id="home-venues" className="home-block home-venues" aria-labelledby="venues-title">
        <header className="home-block-head">
          <div>
            <h2 id="venues-title" className="home-block-title">
              Куда сходить
            </h2>
            <p className="home-block-sub">Заведения с онлайн-бронированием и актуальным меню</p>
          </div>
          {!loading && venues.length > 0 && (
            <span className="home-venues-count">
              {filteredVenues.length} {filteredVenues.length === 1 ? 'место' : filteredVenues.length < 5 ? 'места' : 'мест'}
            </span>
          )}
        </header>

        {!loading && venues.length > 0 && (
          <div className="home-city-filter" role="group" aria-label="Фильтр по городу">
            <span className="home-city-filter-label muted">Город</span>
            <button
              type="button"
              className={!cityFilter ? 'home-city-chip home-city-chip--active' : 'home-city-chip'}
              onClick={() => setCityFilter('')}
            >
              Все города
            </button>
            {cities.length > 0 ? (
              cities.map((c) => (
                <button
                  key={c}
                  type="button"
                  className={cityFilter === c ? 'home-city-chip home-city-chip--active' : 'home-city-chip'}
                  onClick={() => setCityFilter(c)}
                >
                  {c}
                </button>
              ))
            ) : (
              <span className="muted compact">город не указан</span>
            )}
            <span style={{ flex: 1 }} />
            <button
              type="button"
              className={mapMode ? 'home-city-chip home-city-chip--active' : 'home-city-chip'}
              onClick={() => setMapMode((v) => !v)}
              title="Показать рестораны на карте"
            >
              Карта
            </button>
          </div>
        )}

        {loading ? (
          <div className="home-venue-grid home-venue-grid--skeleton">
            {[1, 2, 3].map((i) => (
              <div key={i} className="home-venue-skel">
                <div className="home-venue-skel-img" />
                <div className="home-venue-skel-body">
                  <div className="home-venue-skel-line home-venue-skel-line--sm" />
                  <div className="home-venue-skel-line home-venue-skel-line--lg" />
                  <div className="home-venue-skel-line" />
                </div>
              </div>
            ))}
          </div>
        ) : venues.length === 0 ? (
          <div className="home-empty">
            <p>Пока нет доступных ресторанов. Загляните позже или обновите страницу.</p>
          </div>
        ) : filteredVenues.length === 0 ? (
          <div className="home-empty">
            <p>Нет заведений в выбранном городе.</p>
          </div>
        ) : mapMode ? (
          <div style={{ marginTop: 14 }}>
            <RestaurantsMap pins={mapPins} center={{ lat: cityCenter.lat, lng: cityCenter.lng }} zoom={cityCenter.zoom} />
            {mapPins.length === 0 && (
              <p className="muted compact" style={{ marginTop: 10 }}>
                Для выбранного фильтра нет координат ресторанов (lat/lng). Добавьте их в seed или через админку.
              </p>
            )}
          </div>
        ) : (
          <div className={`home-venue-grid${countClass}`}>
            {filteredVenues.map((r) => (
              <Link key={r.id} to={`/restaurant/${r.id}`} className="home-venue-card">
                <div className="home-venue-card-media">
                  <div
                    className={`home-venue-cover${r.photo_url ? '' : ' home-venue-cover--placeholder'}`}
                    style={
                      r.photo_url
                        ? {
                            backgroundImage: `url(${resolvePublicImageUrl(r.photo_url)})`,
                          }
                        : undefined
                    }
                  />
                  <div className="home-venue-card-shade" />
                  <span className="home-venue-badge">{(r.city || '').trim() || '—'}</span>
                  <span className="home-venue-arrow" aria-hidden>
                    →
                  </span>
                </div>
                <div className="home-venue-card-body">
                  <h3 className="home-venue-name">{displayVenueName(r)}</h3>
                  <div className="home-venue-line">
                    <RestaurantRatingStars compact ratingAvg={r.rating_avg} ratingCount={r.rating_count} />
                  </div>
                  {r.address ? (
                    <p className="home-venue-line muted compact">{r.address}</p>
                  ) : null}
                  {(r.opens_at && r.closes_at) || r.phone ? (
                    <p className="home-venue-line muted compact">
                      {r.opens_at && r.closes_at ? (
                        <span>
                          {r.opens_at}—{r.closes_at}
                        </span>
                      ) : null}
                      {r.phone ? (
                        <span>
                          {r.opens_at && r.closes_at ? ' · ' : ''}
                          {r.phone}
                        </span>
                      ) : null}
                    </p>
                  ) : null}
                  <p className="home-venue-desc">{r.description || 'Бронирование и меню онлайн'}</p>
                  <span className="home-venue-cta">
                    Меню и бронь <span className="home-venue-cta-arrow">→</span>
                  </span>
                </div>
              </Link>
            ))}
          </div>
        )}
      </section>

      <section className="home-block home-perks" aria-labelledby="perks-title">
        <h2 id="perks-title" className="home-block-title home-block-title--center">
          Почему удобно
        </h2>
        <div className="home-perks-grid">
          <article className="home-perk">
            <div className="home-perk-icon">⌗</div>
            <h3>Видите стол на схеме</h3>
            <p>Не абстрактный «номер столика», а расположение относительно окна, прохода и зоны.</p>
          </article>
          <article className="home-perk">
            <div className="home-perk-icon">◇</div>
            <h3>Понятный депозит</h3>
            <p>Подтверждение брони оплатой с правилами возврата — без сюрпризов в день визита.</p>
          </article>
          <article className="home-perk">
            <div className="home-perk-icon">✧</div>
            <h3>Меню до визита</h3>
            <p>Оцените блюда заранее: категории, описания и цены в карточке ресторана.</p>
          </article>
          <article className="home-perk">
            <div className="home-perk-icon">⏱</div>
            <h3>История и напоминания</h3>
            <p>Активные брони и прошлые визиты — в профиле. Меньше хаоса в переписке и звонках.</p>
          </article>
        </div>
      </section>

      <section className="home-block home-how-pro" aria-labelledby="how-title">
        <h2 id="how-title" className="home-block-title">
          Как это работает
        </h2>
        <ol className="home-how-timeline">
          <li className="home-how-step">
            <span className="home-how-step-num">1</span>
            <div>
              <h3>Заведение и зал</h3>
              <p>Выберите город и зону — основной зал, терраса или отдельный зал.</p>
            </div>
          </li>
          <li className="home-how-step">
            <span className="home-how-step-num">2</span>
            <div>
              <h3>Слот и стол</h3>
              <p>Схема показывает вместимость и расположение — у окна или в центре.</p>
            </div>
          </li>
          <li className="home-how-step">
            <span className="home-how-step-num">3</span>
            <div>
              <h3>Депозит и визит</h3>
              <p>Подтвердите бронь оплатой. Все активные визиты — в разделе «Мои брони».</p>
            </div>
          </li>
        </ol>
      </section>

      <footer className="home-dev card" aria-label="Копирайт">
        <p className="muted" style={{ margin: 0 }}>
          © 2026 Hypeaters Inc. Все права зарегистрированы.
        </p>
      </footer>
    </div>
  );
}
