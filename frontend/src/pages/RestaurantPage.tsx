import { useEffect, useMemo, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { api } from '../api';
import { resolvePublicImageUrl } from '../utils/publicAssetUrl';
import { RestaurantRatingStars } from '../components/RestaurantRatingStars';

type RestaurantDetail = {
  id: string;
  name: string;
  slug: string;
  city: string;
  description: string;
  photo_url: string;
  photo_gallery_urls?: string[];
  address?: string;
  phone?: string;
  opens_at?: string;
  closes_at?: string;
  extra_json?: Record<string, unknown>;
  rating_avg?: number | null;
  rating_count?: number;
};

type MenuCategory = {
  id: string;
  parent_id: string | null;
  name: string;
  sort_order: number;
};

type MenuItem = {
  id: string;
  category_id: string;
  name: string;
  description: string;
  price_kopecks: number;
  sort_order: number;
  image_url: string;
};

type MenuPayload = { categories: MenuCategory[]; items: MenuItem[] };

function formatRub(kopecks: number) {
  return (kopecks / 100).toLocaleString('ru-RU', {
    style: 'currency',
    currency: 'RUB',
    maximumFractionDigits: 0,
  });
}

function displayRestaurantName(v: { name?: string; slug?: string } | null | undefined) {
  if (!v) return 'Ресторан';
  const n = (v.name || '').trim();
  if (n) return n;
  const s = (v.slug || '').trim();
  if (s) return s.replace(/-/g, ' ');
  return 'Ресторан';
}

export default function RestaurantPage() {
  const { id } = useParams<{ id: string }>();
  const [venue, setVenue] = useState<RestaurantDetail | null>(null);
  const [menu, setMenu] = useState<MenuPayload | null>(null);
  const [err, setErr] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    setErr(null);
    void (async () => {
      setVenue(null);
      setMenu(null);
      try {
        const { data: v } = await api.get<RestaurantDetail>(`/restaurants/${id}`);
        setVenue(v);
      } catch (e: unknown) {
        const ax = e as { response?: { status?: number; data?: { error?: string } }; message?: string };
        const hint =
          ax.response?.data?.error ||
          (ax.response?.status === 404 ? 'Ресторан не найден' : null) ||
          ax.message ||
          'ошибка сети';
        setErr(`Не удалось загрузить ресторан: ${hint}`);
        return;
      }
      try {
        const { data: m } = await api.get<MenuPayload>(`/restaurants/${id}/menu`);
        setMenu(m);
      } catch {
        setMenu({ categories: [], items: [] });
      }
    })();
  }, [id]);

  const galleryUrls = useMemo(() => {
    if (!venue) return [];
    if (Array.isArray(venue.photo_gallery_urls) && venue.photo_gallery_urls.length > 0) {
      return venue.photo_gallery_urls.filter((u) => typeof u === 'string' && u.trim() !== '');
    }
    const g = venue.extra_json?.photo_gallery;
    if (!Array.isArray(g)) return [];
    return g.filter((x): x is string => typeof x === 'string' && x.trim() !== '');
  }, [venue]);

  /** Миниатюры: галерея; если обложка не входит в галерею — добавляем её первой. */
  const thumbUrls = useMemo(() => {
    if (!venue) return [];
    const cover = (venue.photo_url || '').trim();
    if (!cover) return galleryUrls;
    if (galleryUrls.some((u) => u === cover)) return galleryUrls;
    return [cover, ...galleryUrls];
  }, [venue, galleryUrls]);

  const [heroImageUrl, setHeroImageUrl] = useState<string>('');

  useEffect(() => {
    if (!venue) {
      setHeroImageUrl('');
      return;
    }
    const cover = (venue.photo_url || '').trim();
    if (thumbUrls.length === 0) {
      setHeroImageUrl(cover);
      return;
    }
    setHeroImageUrl(cover || thumbUrls[0]);
  }, [venue?.id, venue?.photo_url, thumbUrls]);

  const sections = useMemo(() => {
    if (!menu) return [];
    const cats = [...menu.categories].sort((a, b) => a.sort_order - b.sort_order || a.name.localeCompare(b.name));
    const roots = cats.filter((c) => !c.parent_id);
    const childrenOf = (pid: string) => cats.filter((c) => c.parent_id === pid);
    const byCat = new Map<string, MenuItem[]>();
    for (const it of menu.items) {
      const arr = byCat.get(it.category_id) ?? [];
      arr.push(it);
      byCat.set(it.category_id, arr);
    }
    for (const [, arr] of byCat) {
      arr.sort((a, b) => a.sort_order - b.sort_order || a.name.localeCompare(b.name));
    }
    return roots.map((root) => ({
      root,
      childCats: childrenOf(root.id),
      itemsRoot: byCat.get(root.id) ?? [],
      byChild: Object.fromEntries(
        childrenOf(root.id).map((ch) => [ch.id, byCat.get(ch.id) ?? []] as const),
      ) as Record<string, MenuItem[]>,
    }));
  }, [menu]);

  if (!id) return <p className="muted">Некорректная ссылка</p>;
  if (err) {
    return (
      <div className="restaurant-public">
        <p className="muted">{err}</p>
        <Link to="/">На главную</Link>
      </div>
    );
  }
  if (!venue) {
    return (
      <div className="restaurant-public">
        <p className="muted">Загрузка…</p>
        <Link to="/">На главную</Link>
      </div>
    );
  }

  return (
    <div className="restaurant-public">
      <nav className="restaurant-public-breadcrumb">
        <Link to="/">Главная</Link>
        <span aria-hidden> / </span>
        <span>{venue ? displayRestaurantName(venue) : 'Меню'}</span>
      </nav>

      <header className="restaurant-public-hero">
        <div
          className={`restaurant-public-cover${heroImageUrl ? '' : ' restaurant-public-cover--placeholder'}`}
          style={
            heroImageUrl ? { backgroundImage: `url(${resolvePublicImageUrl(heroImageUrl)})` } : undefined
          }
          role="img"
          aria-label={venue ? `Фото: ${displayRestaurantName(venue)}` : 'Фото ресторана'}
        />
        <div className="restaurant-public-headline">
          <p className="restaurant-city">{venue?.city}</p>
          <h1>{displayRestaurantName(venue)}</h1>
          <div className="restaurant-public-rating">
            <RestaurantRatingStars ratingAvg={venue.rating_avg} ratingCount={venue.rating_count} />
          </div>
          {venue?.description ? <p className="restaurant-public-lead">{venue.description}</p> : null}
          {venue?.address ? (
            <p className="muted restaurant-public-meta restaurant-public-address">{venue.address}</p>
          ) : null}
          {(venue?.phone || venue?.opens_at || venue?.closes_at) && (
            <p className="muted restaurant-public-meta">
              {venue.phone && <span>{venue.phone}</span>}
              {venue.opens_at && venue.closes_at && (
                <span>
                  {venue.phone ? ' · ' : ''}
                  {venue.opens_at}—{venue.closes_at}
                </span>
              )}
            </p>
          )}
          {typeof venue?.extra_json?.contact_email === 'string' && venue.extra_json.contact_email.trim() !== '' && (
            <p className="muted restaurant-public-meta">
              Email:{' '}
              <a href={`mailto:${String(venue.extra_json.contact_email).trim()}`}>
                {String(venue.extra_json.contact_email).trim()}
              </a>
            </p>
          )}
          <div className="btn-row">
            <Link to={`/hall?restaurant_id=${venue?.id}`} className="btn">
              Забронировать стол
            </Link>
          </div>
        </div>
      </header>

      {thumbUrls.length > 0 && (
        <section className="restaurant-public-gallery-wrap" aria-label="Фотографии ресторана">
          <h2 className="restaurant-public-gallery-title">Фото</h2>
          <p className="muted compact" style={{ margin: 0 }}>
            Нажмите миниатюру, чтобы показать снимок в блоке выше.
          </p>
          <div className="restaurant-public-gallery" role="list">
            {thumbUrls.map((src, gi) => {
              const active = src === heroImageUrl;
              return (
                <button
                  key={`${gi}-${src}`}
                  type="button"
                  className={`restaurant-public-gallery-item${active ? ' restaurant-public-gallery-item--active' : ''}`}
                  style={{ backgroundImage: `url(${resolvePublicImageUrl(src)})` }}
                  aria-pressed={active}
                  aria-label={`Показать фото ${gi + 1} из ${thumbUrls.length}`}
                  onClick={() => setHeroImageUrl(src)}
                />
              );
            })}
          </div>
        </section>
      )}

      <section className="restaurant-public-menu" aria-labelledby="menu-heading">
        <h2 id="menu-heading" className="restaurant-public-menu-title">
          Меню
        </h2>
        {menu === null ? (
          <p className="muted">Загрузка меню…</p>
        ) : menu.items.length === 0 ? (
          <p className="muted">Позиции меню скоро появятся.</p>
        ) : (
          <div className="restaurant-menu-sections">
            {sections.map(({ root, childCats, itemsRoot, byChild }) => (
              <div key={root.id} className="restaurant-menu-block">
                <h3 className="restaurant-menu-cat">{root.name}</h3>
                {itemsRoot.length > 0 && (
                  <div className="public-menu-grid">
                    {itemsRoot.map((it) => (
                      <article key={it.id} className="public-menu-card">
                        <div
                          className={`public-menu-card-visual${it.image_url ? '' : ' public-menu-card-visual--empty'}`}
                          style={
                            it.image_url
                              ? { backgroundImage: `url(${resolvePublicImageUrl(it.image_url)})` }
                              : undefined
                          }
                          aria-hidden
                        />
                        <div className="public-menu-card-body">
                          <h4>{it.name}</h4>
                          {it.description ? <p className="muted public-menu-desc">{it.description}</p> : null}
                          <p className="public-menu-price">{formatRub(it.price_kopecks)}</p>
                        </div>
                      </article>
                    ))}
                  </div>
                )}
                {childCats.map((ch) => {
                  const list = byChild[ch.id] ?? [];
                  if (list.length === 0) return null;
                  return (
                    <div key={ch.id} className="restaurant-menu-sub">
                      <h4 className="restaurant-menu-subtitle">{ch.name}</h4>
                      <div className="public-menu-grid">
                        {list.map((it) => (
                          <article key={it.id} className="public-menu-card">
                            <div
                              className={`public-menu-card-visual${it.image_url ? '' : ' public-menu-card-visual--empty'}`}
                              style={
                                it.image_url
                                  ? { backgroundImage: `url(${resolvePublicImageUrl(it.image_url)})` }
                                  : undefined
                              }
                              aria-hidden
                            />
                            <div className="public-menu-card-body">
                              <h4>{it.name}</h4>
                              {it.description ? <p className="muted public-menu-desc">{it.description}</p> : null}
                              <p className="public-menu-price">{formatRub(it.price_kopecks)}</p>
                            </div>
                          </article>
                        ))}
                      </div>
                    </div>
                  );
                })}
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}
