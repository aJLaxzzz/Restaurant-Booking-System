import { useEffect, useMemo, useState } from 'react';
import { api } from '../api';
import { format, parseISO } from 'date-fns';
import { ru } from 'date-fns/locale';
import { useAuth } from '../auth';
import { reservationStatusLabelRu } from '../utils/reservationStatus';

type Row = {
  reservation_id: string;
  table_number: number;
  start_time: string;
  guest_count: number;
  status: string;
  client_name: string;
  phone?: string;
};

type MenuItem = {
  id: string;
  category_id: string;
  name: string;
  price_kopecks: number;
  image_url?: string;
  description?: string;
};
type MenuCategory = { id: string; name: string; sort_order: number };
type OrderLine = {
  id: string;
  item_name: string;
  quantity: number;
  guest_label: string;
  line_total_kopecks: number;
};

function WaiterOrderPanel({
  reservationId,
  resStatus,
  canEditMenu,
}: {
  reservationId: string;
  resStatus: string;
  canEditMenu: boolean;
}) {
  const [lines, setLines] = useState<OrderLine[]>([]);
  const [total, setTotal] = useState(0);
  const [menuItems, setMenuItems] = useState<MenuItem[]>([]);
  const [categories, setCategories] = useState<MenuCategory[]>([]);
  const [activeCatId, setActiveCatId] = useState<string | null>(null);
  const [restaurantId, setRestaurantId] = useState<string | null>(null);
  const [qtyStr, setQtyStr] = useState('1');
  const [guestLabel, setGuestLabel] = useState('Гость 1');
  const [msg, setMsg] = useState('');
  const [menuOpen, setMenuOpen] = useState(false);

  const load = async () => {
    const { data } = await api.get<{
      restaurant_id: string;
      lines: OrderLine[];
      total_kopecks: number;
    }>(`/reservations/${reservationId}/order`);
    setRestaurantId(data.restaurant_id);
    setLines(Array.isArray(data.lines) ? (data.lines as OrderLine[]) : []);
    setTotal(data.total_kopecks || 0);
    if (data.restaurant_id) {
      const m = await api.get<{ items?: MenuItem[] | null; categories?: MenuCategory[] | null }>(
        `/restaurants/${data.restaurant_id}/menu`
      );
      const items = Array.isArray(m.data.items) ? m.data.items : [];
      const catsRaw = Array.isArray(m.data.categories) ? m.data.categories : [];
      setMenuItems(items);
      const cats = [...catsRaw].sort((a, b) => a.sort_order - b.sort_order || a.name.localeCompare(b.name, 'ru'));
      setCategories(cats);
      const firstWith = cats.find((c) => items.some((i) => i.category_id === c.id));
      setActiveCatId(firstWith?.id ?? cats[0]?.id ?? null);
    }
  };

  useEffect(() => {
    if (resStatus !== 'seated' && resStatus !== 'in_service') return;
    void load().catch(() => setMsg('Не удалось загрузить заказ'));
  }, [reservationId, resStatus]);

  const itemsInCategory = useMemo(() => {
    if (!activeCatId) return [];
    return menuItems.filter((i) => i.category_id === activeCatId);
  }, [menuItems, activeCatId]);

  const qty = useMemo(() => {
    const n = parseInt(qtyStr, 10);
    if (!Number.isFinite(n) || n < 1) return 1;
    return Math.min(99, n);
  }, [qtyStr]);

  const addLine = async (menuItemId: string) => {
    if (!canEditMenu || !menuItemId) return;
    setMsg('');
    try {
      await api.post(`/reservations/${reservationId}/order/lines`, {
        menu_item_id: menuItemId,
        quantity: qty,
        guest_label: guestLabel,
        note: '',
      });
      await load();
    } catch {
      setMsg('Не удалось добавить');
    }
  };

  const removeLine = async (lid: string) => {
    if (!canEditMenu) return;
    try {
      await api.delete(`/reservations/${reservationId}/order/lines/${lid}`);
      await load();
    } catch {
      setMsg('Не удалось удалить строку');
    }
  };

  if (resStatus !== 'seated' && resStatus !== 'in_service') {
    return null;
  }

  return (
    <div className="waiter-order-block">
      <h4>Заказ по меню</h4>
      <ul className="waiter-order-lines">
        {lines.map((l) => (
          <li key={l.id}>
            {l.item_name} ×{l.quantity} ({l.guest_label}) — {(l.line_total_kopecks / 100).toFixed(0)} ₽
            {canEditMenu && (
              <button type="button" className="secondary btn-sm" onClick={() => void removeLine(l.id)}>
                ✕
              </button>
            )}
          </li>
        ))}
      </ul>
      <p className="waiter-order-total">
        <strong>Итого: {(total / 100).toFixed(0)} ₽</strong>
      </p>
      {restaurantId && menuItems.length > 0 && canEditMenu && (
        <button type="button" className="btn btn-sm" style={{ marginTop: 10 }} onClick={() => setMenuOpen(true)}>
          Добавить блюдо
        </button>
      )}
      {restaurantId && menuItems.length > 0 && !canEditMenu && (
        <p className="hint">Добавление позиций доступно только назначенному официанту за стол.</p>
      )}
      {menuOpen && restaurantId && menuItems.length > 0 && canEditMenu && (
        <div
          className="modal-backdrop"
          role="dialog"
          aria-modal="true"
          aria-labelledby="waiter-order-modal-title"
          onClick={() => setMenuOpen(false)}
        >
          <div className="modal-panel waiter-order-add-modal" onClick={(e) => e.stopPropagation()}>
            <h3 id="waiter-order-modal-title">Добавить блюдо</h3>
            {categories.length > 0 && (
              <div className="waiter-order-modal-cats" role="tablist" aria-label="Категории меню">
                {categories.map((c) => (
                  <button
                    key={c.id}
                    type="button"
                    className={c.id === activeCatId ? 'btn btn-sm' : 'secondary btn-sm'}
                    onClick={() => setActiveCatId(c.id)}
                  >
                    {c.name}
                  </button>
                ))}
              </div>
            )}
            <div className="waiter-order-qty-row">
              <label className="compact muted">Кол-во</label>
              <input
                type="text"
                inputMode="numeric"
                value={qtyStr}
                onChange={(e) => {
                  const v = e.target.value;
                  if (v === '' || /^\d{1,2}$/.test(v)) setQtyStr(v);
                }}
                onBlur={() => {
                  const n = parseInt(qtyStr, 10);
                  if (!Number.isFinite(n) || n < 1) setQtyStr('1');
                  else setQtyStr(String(Math.min(99, n)));
                }}
              />
              <input placeholder="Кто заказал" value={guestLabel} onChange={(e) => setGuestLabel(e.target.value)} />
            </div>
            <div className="public-menu-grid waiter-menu-grid">
              {itemsInCategory.map((m) => (
                <article key={m.id} className="public-menu-card waiter-menu-card">
                  <button type="button" className="waiter-menu-card-hit" onClick={() => void addLine(m.id)}>
                    <div
                      className={`public-menu-card-visual${m.image_url ? '' : ' public-menu-card-visual--empty'}`}
                      style={m.image_url ? { backgroundImage: `url(${m.image_url})` } : undefined}
                      aria-hidden
                    />
                    <div className="public-menu-card-body">
                      <h4>{m.name}</h4>
                      {m.description ? <p className="muted public-menu-desc">{m.description}</p> : null}
                      <p className="public-menu-price">{(m.price_kopecks / 100).toFixed(0)} ₽ · в заказ</p>
                    </div>
                  </button>
                </article>
              ))}
            </div>
            {itemsInCategory.length === 0 && <p className="muted compact">В этой категории нет позиций.</p>}
            <div className="btn-row" style={{ marginTop: 12 }}>
              <button type="button" className="secondary" onClick={() => setMenuOpen(false)}>
                Закрыть
              </button>
            </div>
          </div>
        </div>
      )}
      {msg && <p className="form-msg">{msg}</p>}
    </div>
  );
}

export default function WaiterPage() {
  const { user } = useAuth();
  const canEditMenu = user?.role === 'waiter';
  const [rows, setRows] = useState<Row[]>([]);
  const [noteFor, setNoteFor] = useState<string | null>(null);
  const [noteText, setNoteText] = useState('');
  const [msg, setMsg] = useState('');

  const load = async () => {
    const { data } = await api.get<Row[]>('/waiter/my-tables');
    setRows(Array.isArray(data) ? data : []);
  };

  useEffect(() => {
    void load();
  }, []);

  const start = async (id: string) => {
    await api.post(`/reservations/${id}/start-service`, {});
    await load();
  };

  const done = async (id: string) => {
    await api.post(`/reservations/${id}/complete`, {});
    await load();
  };

  const sendNote = async () => {
    if (!noteFor || !noteText.trim()) return;
    setMsg('');
    try {
      await api.post('/waiter/notes', { reservation_id: noteFor, note: noteText.trim() });
      setNoteText('');
      setNoteFor(null);
      setMsg('Заметка сохранена');
    } catch {
      setMsg('Не удалось сохранить');
    }
  };

  return (
    <div className="page-stack">
      <div className="card hero-card">
        <h1>Столы официанта</h1>
        <p className="muted">Назначенные брони: посадка, обслуживание, заказ по меню и заметки.</p>
      </div>

      <div className="card">
        {rows.map((r) => (
          <div key={r.reservation_id} className="waiter-row">
            <div className="waiter-row-head">
              <strong>Стол №{r.table_number}</strong>
              <span className="status-pill">{reservationStatusLabelRu(r.status)}</span>
            </div>
            <p className="waiter-meta">
              {r.client_name}
              {r.phone && <> · {r.phone}</>} · {r.guest_count}{' '}
              {r.guest_count === 1 ? 'гость' : 'гостей'} ·{' '}
              {format(parseISO(r.start_time), 'd MMMM HH:mm', { locale: ru })}
            </p>
            <WaiterOrderPanel reservationId={r.reservation_id} resStatus={r.status} canEditMenu={canEditMenu} />
            <div className="btn-row">
              {r.status === 'seated' && (
                <button type="button" className="btn" onClick={() => void start(r.reservation_id)}>
                  Начать обслуживание
                </button>
              )}
              {r.status === 'in_service' && (
                <button type="button" className="btn" onClick={() => void done(r.reservation_id)}>
                  Стол освобождён
                </button>
              )}
              <button
                type="button"
                className="secondary btn-sm"
                onClick={() => setNoteFor(noteFor === r.reservation_id ? null : r.reservation_id)}
              >
                Заметка
              </button>
            </div>
            {noteFor === r.reservation_id && (
              <div className="note-box">
                <textarea
                  rows={2}
                  placeholder="Аллергии, торт, задержка…"
                  value={noteText}
                  onChange={(e) => setNoteText(e.target.value)}
                />
                <button type="button" className="btn btn-sm" onClick={() => void sendNote()}>
                  Сохранить заметку
                </button>
              </div>
            )}
          </div>
        ))}
        {rows.length === 0 && <p className="muted">Нет назначенных столов на сейчас</p>}
        {msg && <p className="form-msg success">{msg}</p>}
      </div>
    </div>
  );
}
