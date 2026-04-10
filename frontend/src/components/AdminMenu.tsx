import { useEffect, useRef, useState } from 'react';
import { Link } from 'react-router-dom';
import axios from 'axios';
import { api } from '../api';

function menuLoadErrorMessage(e: unknown): string {
  if (axios.isAxiosError(e)) {
    if (e.response?.status === 403) {
      return 'Нет привязки к заведению. Попросите владельца добавить вас в «Команда» в кабинете владельца или войдите под демо-админом.';
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

export function AdminMenu() {
  const [cats, setCats] = useState<Cat[]>([]);
  const [items, setItems] = useState<Item[]>([]);
  const [newCat, setNewCat] = useState('');
  const [newItem, setNewItem] = useState({
    name: '',
    category_id: '',
    price_rub: 10,
    description: '',
    photoFile: null as File | null,
  });
  const newPhotoRef = useRef<HTMLInputElement>(null);
  const [msg, setMsg] = useState('');

  const load = async () => {
    const [c, i] = await Promise.all([api.get<Cat[]>('/admin/menu/categories'), api.get<Item[]>('/admin/menu/items')]);
    setCats(c.data);
    setItems(i.data);
    if (!newItem.category_id && c.data.length) {
      setNewItem((s) => ({ ...s, category_id: c.data[0].id }));
    }
  };

  useEffect(() => {
    void load().catch((e) => setMsg(menuLoadErrorMessage(e)));
  }, []);

  const addCategory = async () => {
    if (!newCat.trim()) return;
    setMsg('');
    await api.post('/admin/menu/categories', { name: newCat.trim(), sort_order: cats.length });
    setNewCat('');
    await load();
  };

  const addItem = async () => {
    if (!newItem.name.trim() || !newItem.category_id) return;
    setMsg('');
    const fd = new FormData();
    fd.append('category_id', newItem.category_id);
    fd.append('name', newItem.name.trim());
    fd.append('description', newItem.description.trim());
    fd.append('price_rub', String(newItem.price_rub));
    fd.append('sort_order', '0');
    if (newItem.photoFile) {
      fd.append('photo', newItem.photoFile);
    }
    await api.post('/admin/menu/items', fd);
    setNewItem((s) => ({ ...s, name: '', description: '', photoFile: null }));
    if (newPhotoRef.current) newPhotoRef.current.value = '';
    await load();
  };

  return (
    <div className="card">
      <h2>Меню заведения</h2>
      <p className="muted">
        Категории и новые блюда. Редактирование существующих цен и названий — во вкладке{' '}
        <Link to="/admin/menu/positions">Позиции меню</Link>. Фото — JPEG/PNG/WebP до 5 МБ.
      </p>

      <div className="menu-admin-grid">
        <div>
          <h3>Категории</h3>
          <div className="btn-row">
            <input placeholder="Новая категория" value={newCat} onChange={(e) => setNewCat(e.target.value)} />
            <button type="button" className="btn btn-sm" onClick={() => void addCategory()}>
              Добавить
            </button>
          </div>
          <ul className="menu-cat-list">
            {cats.map((c) => (
              <li key={c.id}>
                {c.name} {c.parent_id ? `(подкатегория)` : ''}
              </li>
            ))}
          </ul>
        </div>
        <div>
          <h3>Позиции</h3>
          <div className="grid2">
            <div>
              <label>Категория</label>
              <select value={newItem.category_id} onChange={(e) => setNewItem((s) => ({ ...s, category_id: e.target.value }))}>
                {cats.map((c) => (
                  <option key={c.id} value={c.id}>
                    {c.name}
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label>Название</label>
              <input value={newItem.name} onChange={(e) => setNewItem((s) => ({ ...s, name: e.target.value }))} />
            </div>
            <div>
              <label>Цена, ₽</label>
              <input
                type="number"
                min={0}
                step={0.01}
                value={newItem.price_rub}
                onChange={(e) => setNewItem((s) => ({ ...s, price_rub: Number(e.target.value) }))}
              />
            </div>
            <div style={{ gridColumn: '1 / -1' }}>
              <label>Описание (необязательно)</label>
              <input
                value={newItem.description}
                onChange={(e) => setNewItem((s) => ({ ...s, description: e.target.value }))}
                placeholder="Кратко о составе или подаче"
              />
            </div>
            <div style={{ gridColumn: '1 / -1' }}>
              <label>Фото при создании</label>
              <input
                ref={newPhotoRef}
                type="file"
                accept="image/jpeg,image/png,image/webp"
                onChange={(e) => setNewItem((s) => ({ ...s, photoFile: e.target.files?.[0] ?? null }))}
              />
              {newItem.photoFile && (
                <p className="muted compact" style={{ marginTop: 6 }}>
                  Файл: {newItem.photoFile.name}
                </p>
              )}
            </div>
          </div>
          <button type="button" className="btn" onClick={() => void addItem()}>
            Добавить блюдо
          </button>
          <p className="muted compact" style={{ marginTop: '1rem' }}>
            В списке сейчас <strong>{items.length}</strong> позиций. Управление ими — в{' '}
            <Link to="/admin/menu/positions">Позиции меню</Link>.
          </p>
        </div>
      </div>
      {msg && <p className="form-msg">{msg}</p>}
    </div>
  );
}
