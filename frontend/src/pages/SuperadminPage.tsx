import { useCallback, useEffect, useMemo, useState } from 'react';
import { api } from '../api';
import {
  reservationStatusLabelRu,
  userAccountStatusLabelRu,
  userRoleLabelRu,
} from '../utils/reservationStatus';

type Tab =
  | 'applications'
  | 'users'
  | 'restaurants'
  | 'reservations'
  | 'menu'
  | 'orders'
  | 'settings';

type AppRow = {
  id: string;
  email: string;
  full_name: string;
  phone: string;
  created_at?: string;
};

type UserRow = {
  id: string;
  email: string;
  full_name: string;
  phone: string;
  role: string;
  status?: string;
  owner_application_status?: string;
};

type RestRow = {
  id: string;
  name: string;
  slug: string;
  city: string;
  owner_email?: string;
};

type ResvRow = {
  id: string;
  user_id: string;
  full_name: string;
  phone: string;
  table_number: number;
  start_time: string;
  end_time: string;
  guest_count: number;
  status: string;
  created_by: string;
  comment?: string;
};

const TABS: { id: Tab; label: string }[] = [
  { id: 'applications', label: 'Заявки владельцев' },
  { id: 'users', label: 'Пользователи' },
  { id: 'restaurants', label: 'Рестораны' },
  { id: 'reservations', label: 'Брони' },
  { id: 'menu', label: 'Меню' },
  { id: 'orders', label: 'Заказы' },
  { id: 'settings', label: 'Настройки' },
];

export default function SuperadminPage() {
  const [tab, setTab] = useState<Tab>('applications');
  const [apps, setApps] = useState<AppRow[]>([]);
  const [restaurants, setRestaurants] = useState<RestRow[]>([]);
  const [pickedRestaurantId, setPickedRestaurantId] = useState<string>('');
  const [userQ, setUserQ] = useState('');
  const [users, setUsers] = useState<UserRow[]>([]);
  const [reservations, setReservations] = useState<ResvRow[]>([]);
  const [menuCats, setMenuCats] = useState<{ id: string; name: string; sort_order: number }[]>([]);
  const [menuItems, setMenuItems] = useState<{ id: string; name: string; price_kopecks: number }[]>([]);
  const [orderRid, setOrderRid] = useState('');
  const [orderJson, setOrderJson] = useState<string>('');
  const [settingsJson, setSettingsJson] = useState('');
  const [settingsEdit, setSettingsEdit] = useState('');
  const [restSettingsJson, setRestSettingsJson] = useState('');
  const [restSettingsEdit, setRestSettingsEdit] = useState('');
  const [msg, setMsg] = useState('');

  const loadApps = useCallback(async () => {
    const { data } = await api.get<AppRow[]>('/superadmin/owner-applications');
    setApps(Array.isArray(data) ? data : []);
  }, []);

  const loadRest = useCallback(async () => {
    const { data } = await api.get<RestRow[]>('/superadmin/restaurants');
    const list = Array.isArray(data) ? data : [];
    setRestaurants(list);
    setPickedRestaurantId((prev) => prev || (list[0]?.id ?? ''));
  }, []);

  const loadUsers = useCallback(async () => {
    const { data } = await api.get<UserRow[]>('/superadmin/users', { params: { q: userQ || undefined } });
    setUsers(Array.isArray(data) ? data : []);
  }, [userQ]);

  const loadReservations = useCallback(async () => {
    if (!pickedRestaurantId) {
      setReservations([]);
      return;
    }
    const { data } = await api.get<ResvRow[]>('/reservations', {
      params: { restaurant_id: pickedRestaurantId },
    });
    setReservations(Array.isArray(data) ? data : []);
  }, [pickedRestaurantId]);

  const loadMenu = useCallback(async () => {
    if (!pickedRestaurantId) {
      setMenuCats([]);
      setMenuItems([]);
      return;
    }
    const [c, i] = await Promise.all([
      api.get<{ id: string; name: string; sort_order: number }[]>('/admin/menu/categories', {
        params: { restaurant_id: pickedRestaurantId },
      }),
      api.get<{ id: string; name: string; price_kopecks: number }[]>('/admin/menu/items', {
        params: { restaurant_id: pickedRestaurantId },
      }),
    ]);
    setMenuCats(Array.isArray(c.data) ? c.data : []);
    setMenuItems(Array.isArray(i.data) ? i.data : []);
  }, [pickedRestaurantId]);

  const loadSettings = useCallback(async () => {
    const { data } = await api.get<Record<string, unknown>>('/superadmin/settings');
    const s = JSON.stringify(data, null, 2);
    setSettingsJson(s);
    setSettingsEdit(s);
  }, []);

  const loadRestSettings = useCallback(async () => {
    if (!pickedRestaurantId) {
      setRestSettingsJson('');
      setRestSettingsEdit('');
      return;
    }
    const { data } = await api.get<Record<string, unknown>>(
      `/superadmin/restaurants/${pickedRestaurantId}/settings`,
    );
    const s = JSON.stringify(data, null, 2);
    setRestSettingsJson(s);
    setRestSettingsEdit(s);
  }, [pickedRestaurantId]);

  useEffect(() => {
    void loadApps().catch(() => setMsg('Не удалось загрузить заявки'));
  }, [loadApps]);

  useEffect(() => {
    void loadRest().catch(() => {});
  }, [loadRest]);

  useEffect(() => {
    const t = setTimeout(() => {
      void loadUsers().catch(() => {});
    }, 300);
    return () => clearTimeout(t);
  }, [loadUsers, userQ]);

  useEffect(() => {
    if (tab !== 'reservations') return;
    void loadReservations().catch(() => setMsg('Не удалось загрузить брони'));
  }, [tab, loadReservations]);

  useEffect(() => {
    if (tab !== 'menu') return;
    void loadMenu().catch(() => setMsg('Не удалось загрузить меню'));
  }, [tab, loadMenu]);

  useEffect(() => {
    if (menuCats.length === 0) {
      setNewItemCat('');
      return;
    }
    setNewItemCat((prev) => (prev && menuCats.some((c) => c.id === prev) ? prev : menuCats[0].id));
  }, [menuCats]);

  useEffect(() => {
    if (tab !== 'settings') return;
    void loadSettings().catch(() => setMsg('Не удалось загрузить настройки'));
  }, [tab, loadSettings]);

  useEffect(() => {
    if (tab !== 'settings') return;
    void loadRestSettings().catch(() => {});
  }, [tab, loadRestSettings, pickedRestaurantId]);

  const restaurantSelect = useMemo(
    () => (
      <div className="field-block">
        <label>Ресторан (контекст)</label>
        <select value={pickedRestaurantId} onChange={(e) => setPickedRestaurantId(e.target.value)}>
          <option value="">—</option>
          {restaurants.map((r) => (
            <option key={r.id} value={r.id}>
              {r.name} ({r.city})
            </option>
          ))}
        </select>
      </div>
    ),
    [restaurants, pickedRestaurantId],
  );

  const approve = async (id: string) => {
    setMsg('');
    try {
      await api.post(`/superadmin/owner-applications/${id}/approve`);
      await loadApps();
      setMsg('Одобрено');
    } catch (ex: unknown) {
      const m = ex as { response?: { data?: { error?: string } } };
      setMsg(m.response?.data?.error || 'Ошибка');
    }
  };

  const reject = async (id: string) => {
    setMsg('');
    try {
      await api.post(`/superadmin/owner-applications/${id}/reject`);
      await loadApps();
      setMsg('Отклонено');
    } catch (ex: unknown) {
      const m = ex as { response?: { data?: { error?: string } } };
      setMsg(m.response?.data?.error || 'Ошибка');
    }
  };

  const saveUser = async (u: UserRow, role: string, status: string, full_name: string, phone: string) => {
    setMsg('');
    try {
      await api.put(`/superadmin/users/${u.id}`, { role, status, full_name, phone });
      setMsg('Пользователь обновлён');
      await loadUsers();
    } catch (ex: unknown) {
      const m = ex as { response?: { data?: { error?: string } } };
      setMsg(m.response?.data?.error || 'Ошибка');
    }
  };

  const saveRestaurant = async (r: RestRow, patch: Partial<RestRow & { owner_user_id: string }>) => {
    setMsg('');
    try {
      await api.put(`/superadmin/restaurants/${r.id}`, patch);
      setMsg('Ресторан сохранён');
      await loadRest();
    } catch (ex: unknown) {
      const m = ex as { response?: { data?: { error?: string } } };
      setMsg(m.response?.data?.error || 'Ошибка');
    }
  };

  const cancelReservation = async (id: string) => {
    if (!window.confirm('Отменить бронь от имени администратора?')) return;
    setMsg('');
    try {
      await api.delete(`/reservations/${id}`);
      setMsg('Бронь отменена');
      await loadReservations();
    } catch (ex: unknown) {
      const m = ex as { response?: { data?: { error?: string } } };
      setMsg(m.response?.data?.error || 'Ошибка');
    }
  };

  const loadOrder = async () => {
    const rid = orderRid.trim();
    if (!rid) return;
    setMsg('');
    try {
      const { data } = await api.get<unknown>(`/reservations/${rid}/order`);
      setOrderJson(JSON.stringify(data, null, 2));
    } catch (ex: unknown) {
      const m = ex as { response?: { data?: { error?: string } } };
      setOrderJson('');
      setMsg(m.response?.data?.error || 'Не удалось загрузить заказ');
    }
  };

  const saveSettings = async () => {
    setMsg('');
    try {
      const obj = JSON.parse(settingsEdit) as Record<string, unknown>;
      await api.put('/superadmin/settings', obj);
      setMsg('Настройки сохранены');
      await loadSettings();
    } catch (ex: unknown) {
      const m = ex as { response?: { data?: { error?: string } }; message?: string };
      setMsg(m.response?.data?.error || m.message || 'Некорректный JSON');
    }
  };

  const saveRestSettings = async () => {
    if (!pickedRestaurantId) return;
    setMsg('');
    try {
      const obj = JSON.parse(restSettingsEdit) as Record<string, unknown>;
      await api.put(`/superadmin/restaurants/${pickedRestaurantId}/settings`, obj);
      setMsg('Настройки ресторана сохранены');
      await loadRestSettings();
    } catch (ex: unknown) {
      const m = ex as { response?: { data?: { error?: string } }; message?: string };
      setMsg(m.response?.data?.error || m.message || 'Некорректный JSON');
    }
  };

  const [newCatName, setNewCatName] = useState('');
  const [newItemCat, setNewItemCat] = useState('');
  const [newItemName, setNewItemName] = useState('');
  const [newItemPriceRub, setNewItemPriceRub] = useState('');

  const addCategory = async () => {
    if (!pickedRestaurantId || !newCatName.trim()) return;
    setMsg('');
    try {
      await api.post(
        '/admin/menu/categories',
        { name: newCatName.trim(), sort_order: 0 },
        { params: { restaurant_id: pickedRestaurantId } },
      );
      setNewCatName('');
      setMsg('Категория создана');
      await loadMenu();
    } catch (ex: unknown) {
      const m = ex as { response?: { data?: { error?: string } } };
      setMsg(m.response?.data?.error || 'Ошибка');
    }
  };

  const addMenuItem = async () => {
    if (!pickedRestaurantId || !newItemCat || !newItemName.trim()) return;
    const rub = parseFloat(newItemPriceRub.replace(',', '.'));
    if (!Number.isFinite(rub) || rub < 0) {
      setMsg('Укажите цену в рублях');
      return;
    }
    setMsg('');
    try {
      await api.post(
        '/admin/menu/items',
        {
          category_id: newItemCat,
          name: newItemName.trim(),
          price_kopecks: Math.round(rub * 100),
          sort_order: 0,
        },
        { params: { restaurant_id: pickedRestaurantId } },
      );
      setNewItemName('');
      setNewItemPriceRub('');
      setMsg('Позиция создана');
      await loadMenu();
    } catch (ex: unknown) {
      const m = ex as { response?: { data?: { error?: string } } };
      setMsg(m.response?.data?.error || 'Ошибка');
    }
  };

  const saveMenuItem = async (id: string, name: string, priceRub: string) => {
    if (!pickedRestaurantId) return;
    const rub = parseFloat(priceRub.replace(',', '.'));
    if (!Number.isFinite(rub) || rub < 0) {
      setMsg('Некорректная цена');
      return;
    }
    setMsg('');
    try {
      await api.put(
        `/admin/menu/items/${id}`,
        { name, price_kopecks: Math.round(rub * 100) },
        { params: { restaurant_id: pickedRestaurantId } },
      );
      setMsg('Позиция сохранена');
      await loadMenu();
    } catch (ex: unknown) {
      const m = ex as { response?: { data?: { error?: string } } };
      setMsg(m.response?.data?.error || 'Ошибка');
    }
  };

  const saveReservationAdmin = async (
    id: string,
    patch: { guest_count?: number; comment?: string; assigned_waiter_id?: string | null },
  ) => {
    setMsg('');
    try {
      const body: Record<string, unknown> = {};
      if (patch.guest_count != null) body.guest_count = patch.guest_count;
      if (patch.comment !== undefined) body.comment = patch.comment;
      if (patch.assigned_waiter_id !== undefined) body.assigned_waiter_id = patch.assigned_waiter_id;
      await api.put(`/reservations/${id}`, body);
      setMsg('Бронь обновлена');
      await loadReservations();
    } catch (ex: unknown) {
      const m = ex as { response?: { data?: { error?: string } } };
      setMsg(m.response?.data?.error || 'Ошибка');
    }
  };

  return (
    <div className="page-stack">
      <div className="card">
        <h2>Администратор системы</h2>
        <p className="muted">Вкладки: заявки, пользователи, рестораны, брони и меню по выбранному ресторану, просмотр счёта, глобальные настройки.</p>
        {msg && <p className="form-msg">{msg}</p>}
        <div className="btn-row tight" style={{ flexWrap: 'wrap', marginTop: 12 }}>
          {TABS.map((t) => (
            <button
              key={t.id}
              type="button"
              className={tab === t.id ? 'btn btn-sm' : 'secondary btn-sm'}
              onClick={() => {
                setMsg('');
                setTab(t.id);
              }}
            >
              {t.label}
            </button>
          ))}
        </div>
      </div>

      {tab === 'applications' && (
        <div className="card">
          <h3>Заявки владельцев</h3>
          {apps.length === 0 ? (
            <p className="muted">Нет заявок в статусе «ожидает»</p>
          ) : (
            <div className="table-wrap">
              <table className="data-table">
                <thead>
                  <tr>
                    <th>Имя</th>
                    <th>Email</th>
                    <th>Телефон</th>
                    <th></th>
                  </tr>
                </thead>
                <tbody>
                  {apps.map((a) => (
                    <tr key={a.id}>
                      <td>{a.full_name}</td>
                      <td>
                        <code>{a.email}</code>
                      </td>
                      <td>{a.phone}</td>
                      <td>
                        <div className="btn-row tight">
                          <button type="button" className="btn btn-sm" onClick={() => void approve(a.id)}>
                            Одобрить
                          </button>
                          <button type="button" className="secondary btn-sm" onClick={() => void reject(a.id)}>
                            Отклонить
                          </button>
                        </div>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}

      {tab === 'users' && (
        <div className="card">
          <h3>Пользователи</h3>
          <label>Поиск по email или имени</label>
          <input value={userQ} onChange={(e) => setUserQ(e.target.value)} placeholder="фрагмент…" />
          <div className="table-wrap" style={{ marginTop: 12 }}>
            <table className="data-table">
              <thead>
                <tr>
                  <th>ФИО</th>
                  <th>Телефон</th>
                  <th>Email</th>
                  <th>Роль</th>
                  <th>Статус</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {users.map((usr) => (
                  <UserEditRow key={usr.id} u={usr} onSave={saveUser} />
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {tab === 'restaurants' && (
        <div className="card">
          <h3>Рестораны</h3>
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Название</th>
                  <th>Slug</th>
                  <th>Город</th>
                  <th>Владелец (email)</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {restaurants.map((r) => (
                  <RestaurantEditRow key={r.id} r={r} onSave={saveRestaurant} />
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {tab === 'reservations' && (
        <div className="card">
          <h3>Брони</h3>
          {restaurantSelect}
          <p className="hint compact">Список последних броней выбранного ресторана (без фильтра «только сегодня», как у локального админа).</p>
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Время</th>
                  <th>Стол</th>
                  <th>Гость</th>
                  <th>Статус</th>
                  <th>Правка</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {reservations.map((x) => (
                  <ReservationSuperRow
                    key={x.id}
                    x={x}
                    onSave={saveReservationAdmin}
                    onCancel={cancelReservation}
                  />
                ))}
              </tbody>
            </table>
          </div>
          {pickedRestaurantId && reservations.length === 0 && <p className="muted">Нет броней или не загружено.</p>}
        </div>
      )}

      {tab === 'menu' && (
        <div className="card">
          <h3>Меню</h3>
          {restaurantSelect}
          <p className="hint compact">Создание категорий и позиций, правка цен и названий (контекст — выбранный ресторан).</p>
          <div className="field-block" style={{ marginTop: 12 }}>
            <label>Новая категория</label>
            <div className="btn-row tight">
              <input
                value={newCatName}
                onChange={(e) => setNewCatName(e.target.value)}
                placeholder="Название"
                disabled={!pickedRestaurantId}
              />
              <button type="button" className="btn btn-sm" disabled={!pickedRestaurantId} onClick={() => void addCategory()}>
                Добавить
              </button>
            </div>
          </div>
          <h4>Категории</h4>
          <ul className="muted compact">
            {menuCats.map((c) => (
              <li key={c.id}>
                {c.name} <code className="muted">{c.id.slice(0, 8)}…</code>
              </li>
            ))}
          </ul>
          <div className="field-block">
            <label>Новая позиция</label>
            <div className="grid2" style={{ gap: 8 }}>
              <select
                value={newItemCat}
                onChange={(e) => setNewItemCat(e.target.value)}
                disabled={!pickedRestaurantId || menuCats.length === 0}
              >
                <option value="">Категория</option>
                {menuCats.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}
                  </option>
                ))}
              </select>
              <input
                value={newItemName}
                onChange={(e) => setNewItemName(e.target.value)}
                placeholder="Название блюда"
              />
              <input
                value={newItemPriceRub}
                onChange={(e) => setNewItemPriceRub(e.target.value)}
                placeholder="Цена, ₽"
                inputMode="decimal"
              />
            </div>
            <button
              type="button"
              className="btn btn-sm"
              style={{ marginTop: 8 }}
              disabled={!pickedRestaurantId}
              onClick={() => void addMenuItem()}
            >
              Добавить позицию
            </button>
          </div>
          <h4>Позиции</h4>
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th>Название</th>
                  <th>Цена, ₽</th>
                  <th></th>
                </tr>
              </thead>
              <tbody>
                {menuItems.map((i) => (
                  <MenuItemSuperRow key={i.id} item={i} onSave={saveMenuItem} />
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}

      {tab === 'orders' && (
        <div className="card">
          <h3>Заказ по брони</h3>
          <label>ID брони (reservation)</label>
          <input value={orderRid} onChange={(e) => setOrderRid(e.target.value)} placeholder="uuid" />
          <div className="btn-row" style={{ marginTop: 8 }}>
            <button type="button" className="btn btn-sm" onClick={() => void loadOrder()}>
              Загрузить счёт
            </button>
          </div>
          {orderJson && (
            <pre className="muted" style={{ marginTop: 12, overflow: 'auto', maxHeight: 360, fontSize: 12 }}>
              {orderJson}
            </pre>
          )}
        </div>
      )}

      {tab === 'settings' && (
        <div className="card">
          <h3>Глобальные настройки</h3>
          <p className="hint">Изменения влияют на все рестораны (fallback). Редактируйте JSON-объект (ключ → значение JSON).</p>
          <textarea
            rows={16}
            className="settings-json-edit"
            value={settingsEdit}
            onChange={(e) => setSettingsEdit(e.target.value)}
            spellCheck={false}
          />
          <div className="btn-row" style={{ marginTop: 8 }}>
            <button type="button" className="btn btn-sm" onClick={() => void saveSettings()}>
              Сохранить
            </button>
            <button
              type="button"
              className="secondary btn-sm"
              onClick={() => setSettingsEdit(settingsJson)}
            >
              Сбросить
            </button>
          </div>
          <h3 style={{ marginTop: 24 }}>Настройки ресторана</h3>
          {restaurantSelect}
          <p className="hint compact">
            Переопределение для выбранного заведения (avg_check_kopecks, booking_close_hour и др.). Пустые ключи в глобальной таблице
            подставляются автоматически.
          </p>
          <textarea
            rows={12}
            className="settings-json-edit"
            value={restSettingsEdit}
            onChange={(e) => setRestSettingsEdit(e.target.value)}
            spellCheck={false}
            disabled={!pickedRestaurantId}
          />
          <div className="btn-row" style={{ marginTop: 8 }}>
            <button
              type="button"
              className="btn btn-sm"
              disabled={!pickedRestaurantId}
              onClick={() => void saveRestSettings()}
            >
              Сохранить настройки ресторана
            </button>
            <button
              type="button"
              className="secondary btn-sm"
              disabled={!pickedRestaurantId}
              onClick={() => setRestSettingsEdit(restSettingsJson)}
            >
              Сбросить
            </button>
          </div>
        </div>
      )}
    </div>
  );
}

function UserEditRow({
  u,
  onSave,
}: {
  u: UserRow;
  onSave: (u: UserRow, role: string, status: string, full_name: string, phone: string) => void | Promise<void>;
}) {
  const [role, setRole] = useState(u.role);
  const [status, setStatus] = useState(u.status || 'active');
  const [fullName, setFullName] = useState(u.full_name);
  const [phone, setPhone] = useState(u.phone || '');
  useEffect(() => {
    setRole(u.role);
    setStatus(u.status || 'active');
    setFullName(u.full_name);
    setPhone(u.phone || '');
  }, [u.role, u.status, u.id, u.full_name, u.phone]);
  return (
    <tr>
      <td>
        <input className="compact-select" value={fullName} onChange={(e) => setFullName(e.target.value)} />
      </td>
      <td>
        <input className="compact-select" value={phone} onChange={(e) => setPhone(e.target.value)} />
      </td>
      <td>
        <code>{u.email}</code>
      </td>
      <td>
        <select className="compact-select" value={role} onChange={(e) => setRole(e.target.value)}>
          {['client', 'owner', 'admin', 'waiter', 'superadmin'].map((r) => (
            <option key={r} value={r}>
              {userRoleLabelRu(r)}
            </option>
          ))}
        </select>
      </td>
      <td>
        <select className="compact-select" value={status} onChange={(e) => setStatus(e.target.value)}>
          <option value="active">{userAccountStatusLabelRu('active')}</option>
          <option value="blocked">{userAccountStatusLabelRu('blocked')}</option>
        </select>
      </td>
      <td>
        <button type="button" className="btn btn-sm" onClick={() => void onSave(u, role, status, fullName, phone)}>
          Сохранить
        </button>
      </td>
    </tr>
  );
}

function MenuItemSuperRow({
  item,
  onSave,
}: {
  item: { id: string; name: string; price_kopecks: number };
  onSave: (id: string, name: string, priceRub: string) => void | Promise<void>;
}) {
  const [name, setName] = useState(item.name);
  const [rub, setRub] = useState((item.price_kopecks / 100).toFixed(0));
  useEffect(() => {
    setName(item.name);
    setRub((item.price_kopecks / 100).toFixed(0));
  }, [item.id, item.name, item.price_kopecks]);
  return (
    <tr>
      <td>
        <input value={name} onChange={(e) => setName(e.target.value)} />
      </td>
      <td>
        <input value={rub} onChange={(e) => setRub(e.target.value)} style={{ maxWidth: 100 }} inputMode="decimal" />
      </td>
      <td>
        <button type="button" className="btn btn-sm" onClick={() => void onSave(item.id, name, rub)}>
          Сохранить
        </button>
      </td>
    </tr>
  );
}

function ReservationSuperRow({
  x,
  onSave,
  onCancel,
}: {
  x: ResvRow;
  onSave: (
    id: string,
    patch: { guest_count?: number; comment?: string; assigned_waiter_id?: string | null },
  ) => void | Promise<void>;
  onCancel: (id: string) => void | Promise<void>;
}) {
  const [guests, setGuests] = useState(String(x.guest_count));
  const [comment, setComment] = useState(x.comment ?? '');
  const [waiterId, setWaiterId] = useState('');
  useEffect(() => {
    setGuests(String(x.guest_count));
    setComment(x.comment ?? '');
    setWaiterId('');
  }, [x.id, x.guest_count, x.comment]);
  return (
    <tr>
      <td>
        {new Date(x.start_time).toLocaleString('ru-RU')} — {new Date(x.end_time).toLocaleTimeString('ru-RU')}
      </td>
      <td>№{x.table_number}</td>
      <td>
        {x.full_name} <span className="muted">{x.phone}</span>
      </td>
      <td>
        <span className="status-pill">{reservationStatusLabelRu(x.status)}</span>
      </td>
      <td>
        <div className="field-block tight">
          <label className="muted compact">Гостей</label>
          <input
            className="compact-select"
            value={guests}
            onChange={(e) => setGuests(e.target.value)}
            style={{ maxWidth: 56 }}
          />
          <label className="muted compact">Комментарий</label>
          <input value={comment} onChange={(e) => setComment(e.target.value)} placeholder="—" />
          <label className="muted compact">Официант UUID</label>
          <input
            value={waiterId}
            onChange={(e) => setWaiterId(e.target.value)}
            placeholder="пусто = не менять"
            style={{ maxWidth: 200 }}
          />
          <div className="btn-row tight">
            <button
              type="button"
              className="btn btn-sm"
              onClick={() => {
                const g = parseInt(guests, 10);
                const patch: { guest_count?: number; comment?: string; assigned_waiter_id?: string | null } = {
                  comment,
                };
                if (Number.isFinite(g) && g >= 1) patch.guest_count = g;
                if (waiterId.trim() !== '') patch.assigned_waiter_id = waiterId.trim();
                void onSave(x.id, patch);
              }}
            >
              Сохранить
            </button>
            <button
              type="button"
              className="secondary btn-sm"
              onClick={() => void onSave(x.id, { assigned_waiter_id: null, comment })}
            >
              Снять официанта
            </button>
          </div>
        </div>
      </td>
      <td>
        {(x.status === 'pending_payment' || x.status === 'confirmed') && (
          <button type="button" className="secondary btn-sm" onClick={() => void onCancel(x.id)}>
            Отменить
          </button>
        )}
      </td>
    </tr>
  );
}

function RestaurantEditRow({
  r,
  onSave,
}: {
  r: RestRow;
  onSave: (r: RestRow, patch: Partial<RestRow & { owner_user_id: string }>) => void | Promise<void>;
}) {
  const [name, setName] = useState(r.name);
  const [slug, setSlug] = useState(r.slug);
  const [city, setCity] = useState(r.city);
  const [ownerUid, setOwnerUid] = useState('');
  useEffect(() => {
    setName(r.name);
    setSlug(r.slug);
    setCity(r.city);
    setOwnerUid('');
  }, [r.id, r.name, r.slug, r.city]);
  return (
    <tr>
      <td>
        <input value={name} onChange={(e) => setName(e.target.value)} />
      </td>
      <td>
        <input value={slug} onChange={(e) => setSlug(e.target.value)} />
      </td>
      <td>
        <input value={city} onChange={(e) => setCity(e.target.value)} />
      </td>
      <td className="muted">{r.owner_email ?? '—'}</td>
      <td>
        <div className="field-block tight">
          <input
            placeholder="owner_user_id (uuid)"
            value={ownerUid}
            onChange={(e) => setOwnerUid(e.target.value)}
            style={{ maxWidth: 220 }}
          />
          <button
            type="button"
            className="btn btn-sm"
            onClick={() => {
              const patch: Partial<RestRow & { owner_user_id: string }> = {
                name,
                slug,
                city,
              };
              if (ownerUid.trim()) {
                patch.owner_user_id = ownerUid.trim();
              }
              void onSave(r, patch);
            }}
          >
            Сохранить
          </button>
        </div>
      </td>
    </tr>
  );
}
