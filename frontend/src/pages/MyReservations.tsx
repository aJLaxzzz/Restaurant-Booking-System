import { useEffect, useMemo, useState } from 'react';
import { useAuth } from '../auth';
import { api } from '../api';
import { format, parseISO } from 'date-fns';
import { reservationStatusLabelRu } from '../utils/reservationStatus';
import { resolvePublicImageUrl } from '../utils/publicAssetUrl';

type Row = {
  id: string;
  restaurant_id?: string;
  restaurant_name?: string;
  table_number: number;
  start_time: string;
  end_time: string;
  guest_count: number;
  status: string;
};

function formatRub(kopecks: number) {
  return (kopecks / 100).toLocaleString('ru-RU', {
    style: 'currency',
    currency: 'RUB',
    maximumFractionDigits: 0,
  });
}

function axiosErrorText(e: unknown, fallback: string): string {
  const ax = e as { response?: { data?: { error?: string } }; message?: string };
  return ax.response?.data?.error || ax.message || fallback;
}

type OrderLineRow = {
  id: string;
  item_name: string;
  quantity: number;
  line_total_kopecks: number;
  guest_label: string;
  added_by?: string;
  served?: boolean;
};

type MenuItem = {
  id: string;
  category_id: string;
  name: string;
  description?: string;
  price_kopecks: number;
  image_url?: string;
};
type MenuCategory = { id: string; name: string; sort_order: number };

function ClientSelfOrder({
  reservationId,
  restaurantId,
}: {
  reservationId: string;
  restaurantId: string;
}) {
  const [tab, setTab] = useState<'menu' | 'order'>('menu');
  const [menu, setMenu] = useState<MenuItem[]>([]);
  const [categories, setCategories] = useState<MenuCategory[]>([]);
  const [activeCatId, setActiveCatId] = useState<string | null>(null);
  const [lines, setLines] = useState<OrderLineRow[]>([]);
  const [total, setTotal] = useState(0);
  const [qty, setQty] = useState(1);
  const [msg, setMsg] = useState('');

  const load = async () => {
    const [{ data: ord }, m] = await Promise.all([
      api.get<{ lines: OrderLineRow[]; total_kopecks: number }>(`/reservations/${reservationId}/order`),
      api.get<{ items?: MenuItem[]; categories?: MenuCategory[] }>(`/restaurants/${restaurantId}/menu`),
    ]);
    setLines(Array.isArray(ord.lines) ? ord.lines : []);
    setTotal(ord.total_kopecks || 0);
    const rawItems = Array.isArray(m.data.items) ? m.data.items : [];
    const seenId = new Set<string>();
    const items = rawItems.filter((it) => {
      if (seenId.has(it.id)) return false;
      seenId.add(it.id);
      return true;
    });
    const catsRaw = Array.isArray(m.data.categories) ? m.data.categories : [];
    setMenu(items);
    const cats = [...catsRaw].sort((a, b) => a.sort_order - b.sort_order || a.name.localeCompare(b.name, 'ru'));
    setCategories(cats);
    const firstWith = cats.find((c) => items.some((i) => i.category_id === c.id));
    setActiveCatId(firstWith?.id ?? cats[0]?.id ?? null);
  };

  useEffect(() => {
    void load().catch(() => setMsg('Не удалось загрузить заказ'));
  }, [reservationId, restaurantId]);

  const itemsInCategory = useMemo(() => {
    if (!activeCatId) return [];
    return menu.filter((i) => i.category_id === activeCatId);
  }, [menu, activeCatId]);

  const add = async (itemId: string) => {
    setMsg('');
    try {
      await api.post(`/reservations/${reservationId}/order/lines`, {
        menu_item_id: itemId,
        quantity: qty,
        guest_label: 'Гость',
        note: '',
      });
      await load();
    } catch {
      setMsg('Не удалось добавить позицию');
    }
  };

  const removeLine = async (lid: string) => {
    setMsg('');
    try {
      await api.delete(`/reservations/${reservationId}/order/lines/${lid}`);
      await load();
    } catch {
      setMsg('Не удалось удалить строку');
    }
  };

  return (
    <div className="my-res-order">
      <div className="btn-row tight" style={{ marginBottom: 12 }}>
        <button
          type="button"
          className={tab === 'menu' ? 'btn btn-sm' : 'secondary btn-sm'}
          onClick={() => setTab('menu')}
        >
          Меню
        </button>
        <button
          type="button"
          className={tab === 'order' ? 'btn btn-sm' : 'secondary btn-sm'}
          onClick={() => setTab('order')}
        >
          Заказ
        </button>
      </div>

      {tab === 'menu' && (
        <>
          <h4 className="my-res-order-title">Меню по категориям</h4>
          <div className="my-res-order-controls">
            <label className="compact muted">Кол-во</label>
            <input type="number" min={1} max={99} value={qty} onChange={(e) => setQty(Number(e.target.value))} />
          </div>
          {categories.length > 0 && (
            <div className="waiter-order-modal-cats" style={{ marginBottom: 10 }} role="tablist">
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
          <div className="public-menu-grid my-res-menu-grid">
            {itemsInCategory.map((it) => (
              <article key={it.id} className="public-menu-card my-res-menu-card">
                <button type="button" className="my-res-menu-card-hit" onClick={() => void add(it.id)}>
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
                    <p className="public-menu-price">{formatRub(it.price_kopecks)} · добавить</p>
                  </div>
                </button>
              </article>
            ))}
          </div>
          {itemsInCategory.length === 0 && <p className="muted">В этой категории нет позиций</p>}
        </>
      )}

      {tab === 'order' && (
        <>
          <h4 className="my-res-order-title">Ваш заказ</h4>
          <p className="muted compact">
            Удалить можно только свои позиции, пока официант не отметил «принесено».
          </p>
          <ul className="my-res-order-lines">
            {lines.map((l) => (
              <li key={l.id} style={{ display: 'flex', flexWrap: 'wrap', alignItems: 'center', gap: '0.35rem' }}>
                <span className="status-pill" style={{ fontSize: '0.72rem' }}>
                  {l.added_by === 'waiter' ? 'Официант' : 'Гость'}
                </span>
                {l.served ? (
                  <span className="status-pill" style={{ fontSize: '0.72rem' }}>
                    Принесено
                  </span>
                ) : null}
                <span>
                  {l.item_name} ×{l.quantity} ({l.guest_label}) — {formatRub(l.line_total_kopecks)}
                </span>
                {l.added_by === 'client' && !l.served && (
                  <button type="button" className="secondary btn-sm" onClick={() => void removeLine(l.id)}>
                    Удалить
                  </button>
                )}
              </li>
            ))}
          </ul>
        </>
      )}

      <p className="my-res-order-total">
        <strong>Итого: {formatRub(total)}</strong>
      </p>
      {msg && <p className="form-msg">{msg}</p>}
    </div>
  );
}

export default function MyReservations() {
  const { user } = useAuth();
  const [rows, setRows] = useState<Row[]>([]);
  const [loadErr, setLoadErr] = useState<string | null>(null);

  const load = async () => {
    setLoadErr(null);
    try {
      const { data } = await api.get<Row[]>('/reservations/my');
      setRows(Array.isArray(data) ? data : []);
    } catch (e: unknown) {
      const ax = e as { response?: { data?: { error?: string } }; message?: string };
      setLoadErr(ax.response?.data?.error || ax.message || 'Не удалось загрузить брони');
      setRows([]);
    }
  };

  useEffect(() => {
    if (!user) return;
    void load();
  }, [user?.id]);

  /** На одного гостя иногда приходит несколько строк (дубликаты в ответе / тестовые данные) — не дублируем строки таблицы. */
  const rowsUnique = useMemo(() => {
    const m = new Map<string, Row>();
    for (const r of rows) {
      if (!m.has(r.id)) m.set(r.id, r);
    }
    return [...m.values()];
  }, [rows]);

  /**
   * Блок «меню / заказ за столом» только у одной брони: иначе при нескольких seated/in_service
   * (ошибка данных или несколько активных записей) интерфейс повторялся бы целиком.
   */
  const selfOrderReservationId = useMemo(() => {
    const eligible = rowsUnique.filter(
      (r) =>
        (r.status === 'seated' || r.status === 'in_service') &&
        r.restaurant_id &&
        r.restaurant_id.length > 0,
    );
    if (eligible.length === 0) return null;
    eligible.sort((a, b) => {
      const pri = (x: Row) => (x.status === 'in_service' ? 2 : 1);
      if (pri(b) !== pri(a)) return pri(b) - pri(a);
      return new Date(b.start_time).getTime() - new Date(a.start_time).getTime();
    });
    return eligible[0].id;
  }, [rowsUnique]);

  const cancel = async (id: string) => {
    if (!confirm('Отменить бронь?')) return;
    await api.delete(`/reservations/${id}`);
    await load();
  };

  return (
    <div className="page-stack">
      <div className="card">
        <h2>Мои брони</h2>
        <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th>Ресторан</th>
                <th>Стол</th>
                <th>Начало</th>
                <th>Гости</th>
                <th>Статус</th>
                <th>Счёт</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {rowsUnique.map((r) => (
                <ReservationBlock
                  key={r.id}
                  r={r}
                  onCancel={cancel}
                  selfOrderReservationId={selfOrderReservationId}
                />
              ))}
            </tbody>
          </table>
        </div>
        {loadErr && <p className="form-msg">{loadErr}</p>}
        {rowsUnique.length === 0 && !loadErr && <p className="muted">Нет активных броней</p>}
      </div>
    </div>
  );
}

function ReservationBlock({
  r,
  onCancel,
  selfOrderReservationId,
}: {
  r: Row;
  onCancel: (id: string) => void;
  selfOrderReservationId: string | null;
}) {
  const [orderTotal, setOrderTotal] = useState<number | null>(null);
  const [orderOpen, setOrderOpen] = useState(false);
  const [orderRefresh, setOrderRefresh] = useState(0);
  const [payFeedback, setPayFeedback] = useState<{ kind: 'ok' | 'err'; text: string } | null>(null);

  useEffect(() => {
    setPayFeedback(null);
  }, [r.id]);

  useEffect(() => {
    void (async () => {
      try {
        const { data } = await api.get<{ total_kopecks: number; order: { status: string } | null }>(
          `/reservations/${r.id}/order`,
        );
        setOrderTotal(data.total_kopecks);
        setOrderOpen(data.order != null && data.order.status === 'open');
      } catch {
        setOrderTotal(null);
      }
    })();
  }, [r.id, r.status, orderRefresh]);

  const payTab = async () => {
    setPayFeedback(null);
    try {
      const { data } = await api.post<{
        payment_id?: string;
        closed_without_payment?: boolean;
        checkout_url?: string;
      }>(`/reservations/${r.id}/order/checkout`);
      if (data.closed_without_payment) {
        setOrderRefresh((n) => n + 1);
        setPayFeedback({
          kind: 'ok',
          text: 'Счёт закрыт без дополнительной оплаты (например, депозит покрыл сумму).',
        });
        return;
      }
      const pid = data.payment_id;
      if (pid) {
        window.location.assign(`/pay/${pid}`);
        return;
      }
      const url = data.checkout_url;
      if (url && typeof url === 'string') {
        window.location.assign(url.startsWith('/') ? url : `/${url}`);
        return;
      }
      setPayFeedback({ kind: 'err', text: 'Сервер не вернул ссылку на оплату. Обновите страницу.' });
    } catch (e: unknown) {
      setPayFeedback({
        kind: 'err',
        text: axiosErrorText(e, 'Не удалось перейти к оплате'),
      });
    }
  };

  const showPay = orderTotal != null && orderTotal > 0 && orderOpen && (r.status === 'seated' || r.status === 'in_service');
  const showSelfOrder =
    (r.status === 'seated' || r.status === 'in_service') &&
    r.restaurant_id &&
    r.restaurant_id.length > 0 &&
    selfOrderReservationId === r.id;

  return (
    <>
      <tr>
        <td>{r.restaurant_name?.trim() ? r.restaurant_name : '—'}</td>
        <td>№{r.table_number}</td>
        <td>{format(parseISO(r.start_time), 'dd.MM.yyyy HH:mm')}</td>
        <td>{r.guest_count}</td>
        <td>
          <span className="status-pill">{reservationStatusLabelRu(r.status)}</span>
        </td>
        <td>
          {orderTotal != null && orderTotal > 0 ? (
            <span>{(orderTotal / 100).toFixed(0)} ₽</span>
          ) : (
            <span className="muted">—</span>
          )}
        </td>
        <td>
          <div className="btn-row tight">
            {showPay && (
              <button type="button" className="btn btn-sm" onClick={() => void payTab()}>
                Оплатить счёт
              </button>
            )}
            {(r.status === 'pending_payment' || r.status === 'confirmed') && (
              <button type="button" className="secondary btn-sm" onClick={() => void onCancel(r.id)}>
                Отменить
              </button>
            )}
          </div>
          {payFeedback && (
            <p className={payFeedback.kind === 'err' ? 'form-msg' : 'muted compact'} style={{ marginTop: 8, marginBottom: 0 }}>
              {payFeedback.text}
            </p>
          )}
        </td>
      </tr>
      {showSelfOrder && (
        <tr className="my-res-order-row">
          <td colSpan={7}>
            <ClientSelfOrder reservationId={r.id} restaurantId={r.restaurant_id!} />
          </td>
        </tr>
      )}
    </>
  );
}
