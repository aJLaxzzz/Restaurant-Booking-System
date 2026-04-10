import { useEffect, useMemo, useState } from 'react';
import { api } from '../api';
import { format, parseISO } from 'date-fns';
import { ru } from 'date-fns/locale';
import { useAuth } from '../auth';

type Row = {
  reservation_id: string;
  table_number: number;
  start_time: string;
  guest_count: number;
  status: string;
  client_name: string;
  phone?: string;
};

type MenuItem = { id: string; name: string; price_kopecks: number };
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
  const [restaurantId, setRestaurantId] = useState<string | null>(null);
  const [itemId, setItemId] = useState('');
  const [dishQuery, setDishQuery] = useState('');
  const [qty, setQty] = useState(1);
  const [guestLabel, setGuestLabel] = useState('Гость 1');
  const [msg, setMsg] = useState('');

  const filteredMenu = useMemo(() => {
    const q = dishQuery.trim().toLowerCase();
    if (!q) return menuItems.slice(0, 20);
    return menuItems.filter((m) => m.name.toLowerCase().includes(q)).slice(0, 15);
  }, [menuItems, dishQuery]);

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
      const m = await api.get<{ items: { id: string; name: string; price_kopecks: number }[] | null }>(
        `/restaurants/${data.restaurant_id}/menu`
      );
      const items = Array.isArray(m.data.items) ? m.data.items : [];
      setMenuItems(items);
      if (!itemId && items.length) {
        setItemId(items[0].id);
        setDishQuery(items[0].name);
      }
    }
  };

  useEffect(() => {
    if (resStatus !== 'seated' && resStatus !== 'in_service') return;
    void load().catch(() => setMsg('Не удалось загрузить заказ'));
  }, [reservationId, resStatus]);

  const addLine = async () => {
    if (!canEditMenu || !itemId) return;
    setMsg('');
    try {
      await api.post(`/reservations/${reservationId}/order/lines`, {
        menu_item_id: itemId,
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

  const pickDish = (m: MenuItem) => {
    setItemId(m.id);
    setDishQuery(m.name);
  };

  if (resStatus !== 'seated' && resStatus !== 'in_service') {
    return null;
  }

  return (
    <div className="waiter-order-block">
      <h4>Заказ по меню</h4>
      {restaurantId && menuItems.length > 0 && canEditMenu && (
        <div className="waiter-order-add">
          <div className="waiter-dish-search">
            <label className="compact muted">Блюдо (вводите название)</label>
            <input
              type="text"
              value={dishQuery}
              onChange={(e) => {
                setDishQuery(e.target.value);
                const q = e.target.value.trim().toLowerCase();
                const hit = menuItems.find((m) => m.name.toLowerCase() === q);
                if (hit) setItemId(hit.id);
              }}
              placeholder="Начните вводить…"
              list={`menu-dl-${reservationId}`}
            />
            <datalist id={`menu-dl-${reservationId}`}>
              {menuItems.map((m) => (
                <option key={m.id} value={m.name} />
              ))}
            </datalist>
            {filteredMenu.length > 0 && (
              <ul className="waiter-suggest-list">
                {filteredMenu.map((m) => (
                  <li key={m.id}>
                    <button type="button" className="link-like" onClick={() => pickDish(m)}>
                      {m.name} — {(m.price_kopecks / 100).toFixed(0)} ₽
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>
          <input type="number" min={1} max={99} value={qty} onChange={(e) => setQty(Number(e.target.value))} />
          <input placeholder="Кто заказал" value={guestLabel} onChange={(e) => setGuestLabel(e.target.value)} />
          <button type="button" className="btn btn-sm" onClick={() => void addLine()}>
            Добавить
          </button>
        </div>
      )}
      {restaurantId && menuItems.length > 0 && !canEditMenu && (
        <p className="hint">Добавление позиций доступно только назначенному официанту за стол.</p>
      )}
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
              <span className="status-pill">{r.status}</span>
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
