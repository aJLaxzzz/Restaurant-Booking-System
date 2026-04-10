import { useEffect, useMemo, useState } from 'react';
import type { FormEvent } from 'react';
import axios from 'axios';
import { api } from '../api';

function menuLoadErrorMessage(e: unknown): string {
  if (axios.isAxiosError(e)) {
    if (e.response?.status === 403) {
      return 'Нет привязки к заведению. Попросите владельца добавить вас в «Команда» в кабинете владельца или войдите под демо-админом (см. docs/DEMO_USERS.md).';
    }
    if (e.response?.status === 401) {
      return 'Сессия истекла — войдите снова.';
    }
    const err = (e.response?.data as { error?: string } | undefined)?.error;
    if (err) return err;
  }
  return 'Ошибка загрузки меню';
}

type Cat = { id: string; name: string; parent_id?: string; sort_order: number; is_active: boolean };
type Item = {
  id: string;
  category_id: string;
  name: string;
  description: string;
  price_kopecks: number;
  is_available: boolean;
  sort_order: number;
  image_url?: string;
};

export function AdminMenuPositions() {
  const [cats, setCats] = useState<Cat[]>([]);
  const [items, setItems] = useState<Item[]>([]);
  const [msg, setMsg] = useState('');
  const [loadedOk, setLoadedOk] = useState(false);
  const [modal, setModal] = useState<Item | null>(null);
  const [draft, setDraft] = useState({
    name: '',
    priceRub: '',
    category_id: '',
    description: '',
    is_available: true,
  });

  const load = async () => {
    const [c, i] = await Promise.all([api.get<Cat[]>('/admin/menu/categories'), api.get<Item[]>('/admin/menu/items')]);
    setCats(c.data);
    setItems(i.data);
    setLoadedOk(true);
    setMsg('');
  };

  useEffect(() => {
    void load().catch((e) => {
      setLoadedOk(false);
      setMsg(menuLoadErrorMessage(e));
    });
  }, []);

  const sortedCats = useMemo(
    () => [...cats].sort((a, b) => a.sort_order - b.sort_order || a.name.localeCompare(b.name, 'ru')),
    [cats]
  );

  const itemsByCategory = useMemo(() => {
    const m = new Map<string, Item[]>();
    for (const it of items) {
      const arr = m.get(it.category_id) ?? [];
      arr.push(it);
      m.set(it.category_id, arr);
    }
    for (const arr of m.values()) {
      arr.sort((a, b) => a.sort_order - b.sort_order || a.name.localeCompare(b.name, 'ru'));
    }
    return m;
  }, [items]);

  const openModal = (it: Item) => {
    setModal(it);
    setDraft({
      name: it.name,
      priceRub: (it.price_kopecks / 100).toFixed(2),
      category_id: it.category_id,
      description: it.description || '',
      is_available: it.is_available,
    });
    setMsg('');
  };

  const saveModal = async (e: FormEvent) => {
    e.preventDefault();
    if (!modal) return;
    setMsg('');
    const rub = parseFloat(draft.priceRub.replace(',', '.'));
    if (Number.isNaN(rub) || rub < 0) {
      setMsg('Некорректная цена');
      return;
    }
    const kopecks = Math.round(rub * 100);
    await api.put(`/admin/menu/items/${modal.id}`, {
      name: draft.name.trim(),
      price_kopecks: kopecks,
      category_id: draft.category_id,
      description: draft.description.trim(),
      is_available: draft.is_available,
    });
    await load();
    setModal(null);
  };

  const toggleAvailQuick = async (it: Item, e: React.MouseEvent) => {
    e.stopPropagation();
    setMsg('');
    await api.put(`/admin/menu/items/${it.id}`, { is_available: !it.is_available });
    await load();
  };

  const delItem = async () => {
    if (!modal) return;
    if (!confirm('Удалить позицию?')) return;
    setMsg('');
    await api.delete(`/admin/menu/items/${modal.id}`);
    setModal(null);
    await load();
  };

  const uploadDishPhoto = async (itemId: string, file: File | null) => {
    if (!file) return;
    const fd = new FormData();
    fd.append('photo', file);
    await api.post(`/upload/menu-item-photo?item_id=${encodeURIComponent(itemId)}`, fd);
    await load();
  };

  return (
    <div className="card">
      <h2>Редактирование позиций</h2>
      <p className="muted">
        Карточки сгруппированы по категориям, как на странице ресторана. Клик — редактирование; переключатель «в меню» на
        карточке.
      </p>
      {loadedOk && items.length === 0 && (
        <p className="muted">Пока нет позиций — добавьте их во вкладке «Меню».</p>
      )}
      {sortedCats.map((cat) => {
        const list = itemsByCategory.get(cat.id);
        if (!list?.length) return null;
        return (
          <section key={cat.id} className="admin-menu-pos-section">
            <h3 className="admin-menu-pos-section-title">{cat.name}</h3>
            <div className="admin-menu-cards-grid">
              {list.map((it) => (
                <button
                  key={it.id}
                  type="button"
                  className="admin-menu-pos-card"
                  onClick={() => openModal(it)}
                >
                  <div
                    className={`admin-menu-pos-thumb${it.image_url ? '' : ' admin-menu-pos-thumb--empty'}`}
                    style={it.image_url ? { backgroundImage: `url(${it.image_url})` } : undefined}
                    aria-hidden
                  />
                  <div className="admin-menu-pos-card-body">
                    <span className="admin-menu-pos-card-name">{it.name}</span>
                    <span className="admin-menu-pos-card-price">{(it.price_kopecks / 100).toFixed(0)} ₽</span>
                    <label className="admin-menu-pos-card-avail" onClick={(e) => void toggleAvailQuick(it, e)}>
                      <input type="checkbox" className="admin-checkbox-input" checked={it.is_available} readOnly />
                      <span>{it.is_available ? 'в меню' : 'скрыто'}</span>
                    </label>
                  </div>
                </button>
              ))}
            </div>
          </section>
        );
      })}
      {items.length === 0 && <p className="muted">Пока нет позиций — добавьте их в разделе «Меню».</p>}
      {msg && <p className="form-msg">{msg}</p>}

      {modal && (
        <div className="modal-backdrop" role="dialog" aria-modal="true" aria-labelledby="admin-menu-modal-title">
          <div className="modal-panel admin-menu-modal">
            <h3 id="admin-menu-modal-title">Редактирование позиции</h3>
            <form onSubmit={(e) => void saveModal(e)} className="owner-setup-form">
              <label>Название</label>
              <input value={draft.name} onChange={(e) => setDraft((d) => ({ ...d, name: e.target.value }))} required />
              <label>Цена, ₽</label>
              <input
                inputMode="decimal"
                value={draft.priceRub}
                onChange={(e) => setDraft((d) => ({ ...d, priceRub: e.target.value }))}
                required
              />
              <label>Категория</label>
              <select
                value={draft.category_id}
                onChange={(e) => setDraft((d) => ({ ...d, category_id: e.target.value }))}
              >
                {cats.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}
                  </option>
                ))}
              </select>
              <label>Описание</label>
              <textarea
                rows={3}
                value={draft.description}
                onChange={(e) => setDraft((d) => ({ ...d, description: e.target.value }))}
              />
              <label className="admin-menu-pos-check">
                <input
                  type="checkbox"
                  className="admin-checkbox-input"
                  checked={draft.is_available}
                  onChange={(e) => setDraft((d) => ({ ...d, is_available: e.target.checked }))}
                />
                <span>Показывать в меню</span>
              </label>
              <label className="btn-sm secondary" style={{ cursor: 'pointer' }}>
                Загрузить фото блюда
                <input
                  type="file"
                  accept="image/jpeg,image/png,image/webp"
                  hidden
                  onChange={(e) => void uploadDishPhoto(modal.id, e.target.files?.[0] ?? null)}
                />
              </label>
              <div className="btn-row">
                <button type="submit" className="btn">
                  Сохранить
                </button>
                <button type="button" className="secondary" onClick={() => setModal(null)}>
                  Закрыть
                </button>
                <button type="button" className="secondary" onClick={() => void delItem()}>
                  Удалить
                </button>
              </div>
            </form>
          </div>
        </div>
      )}
    </div>
  );
}
