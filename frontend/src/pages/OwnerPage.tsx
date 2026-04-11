import { useEffect, useState } from 'react';
import type { FormEvent } from 'react';
import axios from 'axios';
import { format, subDays } from 'date-fns';
import { api } from '../api';
import { useAuth } from '../auth';
import { normalizeRuPhoneInput, isValidRuPhoneE164 } from '../utils/phone';
import { resolvePublicImageUrl } from '../utils/publicAssetUrl';
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler,
} from 'chart.js';
import { Line } from 'react-chartjs-2';
ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Title, Tooltip, Legend, Filler);

const SETTING_FORM_KEYS = [
  'avg_check_kopecks',
  'deposit_percent',
  'slot_minutes',
  'booking_open_hour',
  'booking_close_hour',
  'default_slot_duration_hours',
  'refund_more_than_2h_percent',
  'refund_within_2h_percent',
  'refund_more_than_24h_percent',
  'refund_12_to_24h_percent',
  'refund_less_than_12h_percent',
  'no_show_grace_minutes',
] as const;

const OWNER_SETTING_META: Record<(typeof SETTING_FORM_KEYS)[number], { title: string; hint: string }> = {
  avg_check_kopecks: {
    title: 'Средний чек (копейки)',
    hint: 'Оценка среднего чека; участвует в расчёте депозита при брони.',
  },
  deposit_percent: {
    title: 'Депозит (%)',
    hint: 'Доля от оценки чека, удерживаемая при бронировании.',
  },
  slot_minutes: {
    title: 'Шаг времени (минуты)',
    hint: 'Интервал между слотами на странице брони (например 30 — каждые полчаса).',
  },
  booking_open_hour: {
    title: 'Час открытия брони',
    hint: 'С какого часа гость может выбрать время визита (0–23).',
  },
  booking_close_hour: {
    title: 'Час закрытия брони',
    hint: 'До какого часа можно начать визит; 0 — до полуночи.',
  },
  default_slot_duration_hours: {
    title: 'Длительность визита (часы)',
    hint: 'Стандартная длина брони для проверки занятости столов.',
  },
  refund_more_than_2h_percent: {
    title: 'Возврат при отмене > 2 ч до визита (%)',
    hint: 'Какую долю депозита вернуть, если отмена раньше чем за 2 часа до начала.',
  },
  refund_within_2h_percent: {
    title: 'Возврат при отмене в последние 2 ч (%)',
    hint: 'Доля возврата при поздней отмене.',
  },
  refund_more_than_24h_percent: {
    title: 'Возврат при отмене раньше чем за 24 ч (%)',
    hint: 'Дополнительный порог в таблице настроек; отмена депозита в API в первую очередь использует правила «2 ч» выше.',
  },
  refund_12_to_24h_percent: {
    title: 'Возврат при отмене за 12–24 ч (%)',
    hint: 'Хранится в БД; при отмене через админку применяются ключи с порогом 2 ч, если они заданы.',
  },
  refund_less_than_12h_percent: {
    title: 'Возврат при отмене менее чем за 12 ч (%)',
    hint: 'Используется как запасной вариант в логике возврата, если нет отдельного ключа «последние 2 ч».',
  },
  no_show_grace_minutes: {
    title: 'Минут после начала брони до авто-неявки',
    hint: 'Подтверждённая бронь без посадки гостя переводится в «Не пришёл», если прошло столько минут после start_time.',
  },
};

function settingToInputString(v: unknown): string {
  if (v === null || v === undefined) return '';
  if (typeof v === 'object' && v !== null && 'minutes' in v) {
    const m = (v as { minutes?: unknown }).minutes;
    if (typeof m === 'number' && Number.isFinite(m)) return String(Math.round(m));
  }
  if (typeof v === 'number' && Number.isFinite(v)) return String(v);
  if (typeof v === 'string') {
    try {
      const p = JSON.parse(v) as unknown;
      if (typeof p === 'number' && Number.isFinite(p)) return String(p);
    } catch {
      /* ignore */
    }
    const n = Number(v.replace(',', '.'));
    if (Number.isFinite(n)) return String(n);
    return v;
  }
  return '';
}

type Analytics = {
  period_from?: string;
  period_to?: string;
  labels: string[];
  load_percent: number[];
  bookings_labels?: string[];
  bookings_count?: number[];
  no_show_count?: number[];
  top_dishes?: { name: string; quantity: number; revenue_kopecks: number }[];
  flop_dishes?: { name: string; quantity: number }[];
  total_revenue_kopecks_90d?: number;
  deposit_revenue_kopecks_90d?: number;
  tab_revenue_kopecks_90d?: number;
  completed_visits_90d?: number;
  avg_closed_check_kopecks_90d?: number;
  revenue_by_day_labels?: string[];
  revenue_by_day_kopecks?: number[];
};

type StaffRow = {
  id: string;
  email: string;
  full_name: string;
  phone: string;
  role: string;
  status: string;
};

export default function OwnerPage() {
  const { user, refreshMe } = useAuth();
  const [analytics, setAnalytics] = useState<Analytics | null>(null);
  const [settings, setSettings] = useState<Record<string, unknown> | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [restPhoto, setRestPhoto] = useState<string | null>(null);
  const [restGallery, setRestGallery] = useState<string[]>([]);
  const [restGalleryMsg, setRestGalleryMsg] = useState('');
  const [staff, setStaff] = useState<StaffRow[]>([]);
  const [staffEmail, setStaffEmail] = useState('');
  const [staffRole, setStaffRole] = useState<'waiter' | 'admin' | 'client'>('waiter');
  const [staffMsg, setStaffMsg] = useState('');

  const [rangeFrom, setRangeFrom] = useState(() => format(subDays(new Date(), 89), 'yyyy-MM-dd'));
  const [rangeTo, setRangeTo] = useState(() => format(new Date(), 'yyyy-MM-dd'));

  const [restPhone, setRestPhone] = useState('');
  const [restOpens, setRestOpens] = useState('');
  const [restCloses, setRestCloses] = useState('');
  const [restAddress, setRestAddress] = useState('');
  const [restCity, setRestCity] = useState('');
  const [restDescription, setRestDescription] = useState('');
  const [restContactMsg, setRestContactMsg] = useState('');
  const [restContactEmail, setRestContactEmail] = useState('');
  const [settingsForm, setSettingsForm] = useState<Record<string, string>>({});
  const [settingsSaveMsg, setSettingsSaveMsg] = useState('');

  const [setupName, setSetupName] = useState('');
  const [setupAddress, setSetupAddress] = useState('');
  const [setupCity, setSetupCity] = useState('');
  const [setupSlug, setSetupSlug] = useState('');
  const [setupDesc, setSetupDesc] = useState('');
  const [setupErr, setSetupErr] = useState('');

  useEffect(() => {
    if (!user?.restaurant_id || user.role !== 'owner') {
      setAnalytics(null);
      setSettings(null);
      setLoadError(null);
      return;
    }
    void (async () => {
      setLoadError(null);
      try {
        const [a, s] = await Promise.all([
          api.get<Analytics>('/owner/analytics', { params: { from: rangeFrom, to: rangeTo } }),
          api.get('/settings'),
        ]);
        setAnalytics(a.data);
        setSettings(s.data as Record<string, unknown>);
      } catch (e: unknown) {
        if (axios.isAxiosError(e)) {
          const d = e.response?.data as { error?: string } | undefined;
          setLoadError(d?.error ?? e.message ?? 'Ошибка загрузки');
        } else if (e instanceof Error) {
          setLoadError(e.message);
        } else {
          setLoadError('Ошибка загрузки');
        }
      }
    })();
  }, [user?.restaurant_id, user?.role, rangeFrom, rangeTo]);

  useEffect(() => {
    if (!user?.restaurant_id) {
      setRestPhoto(null);
      setRestGallery([]);
      setStaff([]);
      return;
    }
    void (async () => {
      try {
        const { data } = await api.get<{
          photo_url?: string;
          photo_gallery_urls?: string[];
          phone?: string;
          opens_at?: string;
          closes_at?: string;
          address?: string;
          city?: string;
          description?: string;
          extra_json?: Record<string, unknown>;
        }>(`/restaurants/${user.restaurant_id}`);
        setRestPhoto(data.photo_url || null);
        setRestGallery(
          Array.isArray(data.photo_gallery_urls)
            ? data.photo_gallery_urls.filter((u): u is string => typeof u === 'string' && u.trim() !== '')
            : [],
        );
        setRestPhone(data.phone ?? '');
        setRestOpens(data.opens_at ?? '');
        setRestCloses(data.closes_at ?? '');
        setRestAddress(data.address ?? '');
        setRestCity(data.city ?? '');
        setRestDescription(data.description ?? '');
        const em = data.extra_json?.contact_email;
        setRestContactEmail(typeof em === 'string' ? em : '');
      } catch {
        setRestPhoto(null);
        setRestGallery([]);
      }
    })();
  }, [user?.restaurant_id]);

  useEffect(() => {
    if (!settings) return;
    const next: Record<string, string> = {};
    for (const k of SETTING_FORM_KEYS) {
      next[k] = settingToInputString(settings[k]);
    }
    setSettingsForm(next);
  }, [settings]);

  useEffect(() => {
    if (!user?.restaurant_id || user.role !== 'owner') {
      setStaff([]);
      return;
    }
    void (async () => {
      try {
        const { data } = await api.get<StaffRow[]>('/owner/users');
        setStaff(Array.isArray(data) ? data : []);
      } catch {
        setStaff([]);
      }
    })();
  }, [user?.restaurant_id, user?.role]);

  const createRestaurant = async (e: FormEvent) => {
    e.preventDefault();
    setSetupErr('');
    try {
      await api.post('/owner/restaurant', {
        name: setupName.trim(),
        address: setupAddress.trim(),
        city: setupCity.trim(),
        slug: setupSlug.trim() || undefined,
        description: setupDesc.trim(),
      });
      await refreshMe();
    } catch (ex: unknown) {
      const m = ex as { response?: { data?: { error?: string } } };
      setSetupErr(m.response?.data?.error || 'Ошибка');
    }
  };

  const uploadRestaurantPhoto = async (f: File | null) => {
    if (!f) return;
    const fd = new FormData();
    fd.append('photo', f);
    const { data } = await api.post<{ url: string }>('/upload/restaurant-photo', fd);
    setRestPhoto(data.url);
  };

  const uploadRestaurantGalleryPhoto = async (f: File | null) => {
    if (!f) return;
    setRestGalleryMsg('');
    try {
      const fd = new FormData();
      fd.append('photo', f);
      const { data } = await api.post<{ url: string }>('/upload/restaurant-photo', fd, {
        params: { target: 'gallery' },
      });
      setRestGallery((prev) => (prev.includes(data.url) ? prev : [...prev, data.url]));
    } catch (ex: unknown) {
      const m = ex as { response?: { data?: { error?: string } } };
      setRestGalleryMsg(m.response?.data?.error || 'Ошибка загрузки');
    }
  };

  const removeGalleryItem = async (url: string) => {
    setRestGalleryMsg('');
    const next = restGallery.filter((u) => u !== url);
    try {
      await api.put('/owner/restaurant', { photo_gallery_urls: next });
      setRestGallery(next);
    } catch (ex: unknown) {
      const m = ex as { response?: { data?: { error?: string } } };
      setRestGalleryMsg(m.response?.data?.error || 'Ошибка');
    }
  };

  const saveRestaurantContact = async (e: FormEvent) => {
    e.preventDefault();
    setRestContactMsg('');
    try {
      const phoneNorm = normalizeRuPhoneInput(restPhone);
      if (!isValidRuPhoneE164(phoneNorm)) {
        setRestContactMsg('Телефон: +7 и 10 цифр');
        return;
      }
      await api.put('/owner/restaurant', {
        phone: phoneNorm,
        opens_at: restOpens.trim(),
        closes_at: restCloses.trim(),
        address: restAddress.trim(),
        city: restCity.trim(),
        description: restDescription.trim(),
        contact_email: restContactEmail.trim(),
      });
      setRestContactMsg('Сохранено');
    } catch (ex: unknown) {
      const m = ex as { response?: { data?: { error?: string } } };
      setRestContactMsg(m.response?.data?.error || 'Ошибка');
    }
  };

  const saveSettingsForm = async (e: FormEvent) => {
    e.preventDefault();
    setSettingsSaveMsg('');
    const body: Record<string, unknown> = {};
    for (const k of SETTING_FORM_KEYS) {
      const raw = (settingsForm[k] ?? '').trim();
      if (raw === '') continue;
      const n = Number(raw.replace(',', '.'));
      if (!Number.isFinite(n)) {
        setSettingsSaveMsg(`Некорректное число: ${k}`);
        return;
      }
      if (k === 'no_show_grace_minutes') {
        body[k] = { minutes: Math.max(1, Math.round(n)) };
      } else {
        body[k] = n;
      }
    }
    if (Object.keys(body).length === 0) {
      setSettingsSaveMsg('Нет значений для сохранения');
      return;
    }
    try {
      await api.put('/settings', body);
      const { data } = await api.get<Record<string, unknown>>('/settings');
      setSettings(data);
      setSettingsSaveMsg('Сохранено');
    } catch (ex: unknown) {
      const m = ex as { response?: { data?: { error?: string } } };
      setSettingsSaveMsg(m.response?.data?.error || 'Ошибка сохранения');
    }
  };

  const setPresetDays = (days: number) => {
    const to = new Date();
    const from = subDays(to, days - 1);
    setRangeFrom(format(from, 'yyyy-MM-dd'));
    setRangeTo(format(to, 'yyyy-MM-dd'));
  };

  const assignStaff = async () => {
    setStaffMsg('');
    try {
      await api.post('/owner/staff/assign', { email: staffEmail.trim(), role: staffRole });
      setStaffEmail('');
      const { data } = await api.get<StaffRow[]>('/owner/users');
      setStaff(Array.isArray(data) ? data : []);
    } catch (ex: unknown) {
      const m = ex as { response?: { data?: { error?: string } } };
      setStaffMsg(m.response?.data?.error || 'Ошибка');
    }
  };

  const downloadFinance = async () => {
    const res = await api.get('/owner/finance/export', {
      params: { from: rangeFrom, to: rangeTo },
      responseType: 'blob',
    });
    const url = URL.createObjectURL(res.data);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'finance-report.xlsx';
    a.click();
    URL.revokeObjectURL(url);
  };

  if (user?.role === 'owner' && !user.restaurant_id) {
    return (
      <div className="page-stack">
        <div className="card hero-card">
          <h1>Создайте ресторан</h1>
          <p className="muted">Укажите данные заведения — появится зал и вы сможете настроить схему, меню и команду.</p>
          <form onSubmit={(e) => void createRestaurant(e)} className="owner-setup-form">
            <label>Название</label>
            <input value={setupName} onChange={(e) => setSetupName(e.target.value)} required />
            <label>Город</label>
            <input value={setupCity} onChange={(e) => setSetupCity(e.target.value)} />
            <label>Адрес</label>
            <input value={setupAddress} onChange={(e) => setSetupAddress(e.target.value)} />
            <label>Slug в URL (латиница, необязательно)</label>
            <input value={setupSlug} onChange={(e) => setSetupSlug(e.target.value)} placeholder="moya-trattoria" />
            <label>Описание</label>
            <textarea rows={3} value={setupDesc} onChange={(e) => setSetupDesc(e.target.value)} />
            {setupErr && <p className="form-msg">{setupErr}</p>}
            <button type="submit" className="btn">
              Создать заведение
            </button>
          </form>
        </div>
      </div>
    );
  }

  const chartData =
    analytics && analytics.labels.length
      ? {
          labels: analytics.labels,
          datasets: [
            {
              label: 'Загрузка столов, %',
              data: analytics.load_percent,
              borderColor: 'rgb(129, 140, 248)',
              backgroundColor: 'rgba(129, 140, 248, 0.12)',
              fill: true,
              tension: 0.3,
            },
          ],
        }
      : null;

  const bookingsChart =
    analytics && analytics.bookings_labels && analytics.bookings_labels.length
      ? {
          labels: analytics.bookings_labels,
          datasets: [
            {
              label: 'Брони',
              data: analytics.bookings_count || [],
              borderColor: 'rgb(52, 211, 153)',
              backgroundColor: 'rgba(52, 211, 153, 0.1)',
              fill: true,
              tension: 0.2,
            },
          ],
        }
      : null;

  const rev = analytics?.total_revenue_kopecks_90d ?? 0;
  const dep = analytics?.deposit_revenue_kopecks_90d ?? 0;
  const tab = analytics?.tab_revenue_kopecks_90d ?? 0;
  const visits = analytics?.completed_visits_90d ?? 0;
  const avgCheck = analytics?.avg_closed_check_kopecks_90d ?? 0;

  const revByDay =
    analytics &&
    analytics.revenue_by_day_labels &&
    analytics.revenue_by_day_labels.length &&
    analytics.revenue_by_day_kopecks &&
    analytics.revenue_by_day_kopecks.length
      ? {
          labels: analytics.revenue_by_day_labels,
          datasets: [
            {
              label: 'Платежи, ₽',
              data: analytics.revenue_by_day_kopecks!.map((k) => k / 100),
              borderColor: 'rgb(251, 191, 36)',
              backgroundColor: 'rgba(251, 191, 36, 0.1)',
              fill: true,
              tension: 0.2,
            },
          ],
        }
      : null;

  return (
    <div className="page-stack">
      {loadError && (
        <div className="card" style={{ borderColor: '#b91c1c' }}>
          <p>Не удалось загрузить кабинет: {loadError}</p>
        </div>
      )}
      <div className="card hero-card">
        <h1>Кабинет владельца</h1>
        <p className="muted">Аналитика по вашему заведению, топ блюд и экспорт XLSX.</p>
        <div className="owner-period-row">
          <label className="owner-period-label">Период (МСК)</label>
          <input type="date" value={rangeFrom} onChange={(e) => setRangeFrom(e.target.value)} />
          <span className="muted">—</span>
          <input type="date" value={rangeTo} onChange={(e) => setRangeTo(e.target.value)} />
          <button type="button" className="secondary btn-sm" onClick={() => setPresetDays(7)}>
            7 дн.
          </button>
          <button type="button" className="secondary btn-sm" onClick={() => setPresetDays(30)}>
            30 дн.
          </button>
          <button type="button" className="secondary btn-sm" onClick={() => setPresetDays(90)}>
            90 дн.
          </button>
        </div>
        {analytics?.period_from && (
          <p className="muted compact" style={{ marginTop: 8 }}>
            Данные: {analytics.period_from} — {analytics.period_to}
          </p>
        )}
        <div className="owner-kpi">
          <div className="owner-kpi-card">
            <span className="owner-kpi-label">Выручка (период)</span>
            <strong>{(rev / 100).toFixed(0)} ₽</strong>
          </div>
          <div className="owner-kpi-card">
            <span className="owner-kpi-label">Депозиты</span>
            <strong>{(dep / 100).toFixed(0)} ₽</strong>
          </div>
          <div className="owner-kpi-card">
            <span className="owner-kpi-label">Счета (tab)</span>
            <strong>{(tab / 100).toFixed(0)} ₽</strong>
          </div>
          <div className="owner-kpi-card">
            <span className="owner-kpi-label">Завершённых визитов</span>
            <strong>{visits}</strong>
          </div>
          <div className="owner-kpi-card">
            <span className="owner-kpi-label">Средний чек (закрытые)</span>
            <strong>{(avgCheck / 100).toFixed(0)} ₽</strong>
          </div>
        </div>
        <div className="btn-row">
          <button type="button" className="btn secondary" onClick={() => void downloadFinance()}>
            Скачать отчёт XLSX
          </button>
        </div>
      </div>

      {user?.restaurant_id && (
        <div className="grid2">
          <div className="card">
            <h2>Контакты и карточка</h2>
            <p className="muted compact">
              Адрес, город, описание, телефон и часы видны гостям на главной и на странице ресторана.
            </p>
            <form onSubmit={(e) => void saveRestaurantContact(e)} className="owner-setup-form">
              <label>Город</label>
              <input value={restCity} onChange={(e) => setRestCity(e.target.value)} placeholder="Москва" />
              <label>Адрес</label>
              <input value={restAddress} onChange={(e) => setRestAddress(e.target.value)} placeholder="ул. Примерная, 1" />
              <label>Краткое описание</label>
              <textarea
                rows={3}
                value={restDescription}
                onChange={(e) => setRestDescription(e.target.value)}
                placeholder="Кухня, атмосфера, особенности"
              />
              <label>Телефон</label>
              <input
                inputMode="tel"
                value={restPhone}
                onChange={(e) => setRestPhone(normalizeRuPhoneInput(e.target.value))}
                placeholder="+7 9XX XXX XX XX"
              />
              <label>Email для гостей (публичная страница)</label>
              <input
                type="email"
                value={restContactEmail}
                onChange={(e) => setRestContactEmail(e.target.value)}
                placeholder="hello@restaurant.ru"
              />
              <label>Открытие (например 10:00)</label>
              <input value={restOpens} onChange={(e) => setRestOpens(e.target.value)} placeholder="10:00" />
              <label>Закрытие (например 23:00)</label>
              <input value={restCloses} onChange={(e) => setRestCloses(e.target.value)} placeholder="23:00" />
              <button type="submit" className="btn btn-sm">
                Сохранить
              </button>
              {restContactMsg && <p className="form-msg">{restContactMsg}</p>}
            </form>
          </div>
          <div className="card">
            <h2>Фото ресторана</h2>
            <p className="muted compact">
              Обложка — в каталоге на главной. Галерея — на публичной странице ресторана (несколько снимков). JPEG, PNG или
              WebP, до 5 МБ.
            </p>
            <p className="muted compact" style={{ marginTop: 8 }}>
              Обложка
            </p>
            {restPhoto && (
              <img
                src={resolvePublicImageUrl(restPhoto)}
                alt=""
                className="owner-photo-preview"
                style={{ maxWidth: '100%', borderRadius: 8 }}
              />
            )}
            <input
              type="file"
              accept="image/jpeg,image/png,image/webp"
              onChange={(e) => void uploadRestaurantPhoto(e.target.files?.[0] ?? null)}
            />
            <p className="muted compact" style={{ marginTop: 12 }}>
              Галерея (страница ресторана)
            </p>
            {restGallery.length > 0 && (
              <ul className="owner-gallery-list">
                {restGallery.map((url) => (
                  <li key={url} className="owner-gallery-row">
                    <img src={resolvePublicImageUrl(url)} alt="" className="owner-gallery-thumb" />
                    <button type="button" className="secondary btn-sm" onClick={() => void removeGalleryItem(url)}>
                      Убрать
                    </button>
                  </li>
                ))}
              </ul>
            )}
            <input
              type="file"
              accept="image/jpeg,image/png,image/webp"
              onChange={(e) => void uploadRestaurantGalleryPhoto(e.target.files?.[0] ?? null)}
            />
            {restGalleryMsg && <p className="form-msg">{restGalleryMsg}</p>}
          </div>
          <div className="card">
            <h2>Команда</h2>
            <p className="muted compact">
              Укажите email уже зарегистрированного пользователя. Роль «Гость» снимает доступ к заведению.
            </p>
            <label>Email</label>
            <input type="email" value={staffEmail} onChange={(e) => setStaffEmail(e.target.value)} placeholder="waiter@mail.ru" />
            <label>Роль</label>
            <select value={staffRole} onChange={(e) => setStaffRole(e.target.value as 'waiter' | 'admin' | 'client')}>
              <option value="waiter">Официант</option>
              <option value="admin">Администратор</option>
              <option value="client">Снять доступ (гость)</option>
            </select>
            <button type="button" className="btn btn-sm" style={{ marginTop: 8 }} onClick={() => void assignStaff()}>
              Применить
            </button>
            {staffMsg && <p className="form-msg">{staffMsg}</p>}
            <ul className="owner-list" style={{ marginTop: 12 }}>
              {staff.map((s) => (
                <li key={s.id}>
                  {s.full_name || s.email} — {s.role}
                </li>
              ))}
            </ul>
          </div>
        </div>
      )}

      <div className="card">
        <h2>Загрузка по дням</h2>
        {chartData ? (
          <div className="chart-wrap">
            <Line
              data={chartData}
              options={{
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { position: 'top' as const, labels: { color: '#94a3b8' } } },
                scales: {
                  y: { min: 0, suggestedMax: 100, ticks: { color: '#94a3b8' }, grid: { color: '#334155' } },
                  x: { ticks: { color: '#94a3b8' }, grid: { color: '#334155' } },
                },
              }}
            />
          </div>
        ) : (
          <p className="muted">Недостаточно данных для графика</p>
        )}
      </div>

      {revByDay && (
        <div className="card">
          <h2>Платежи по дням</h2>
          <div className="chart-wrap">
            <Line
              data={revByDay}
              options={{
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { position: 'top' as const, labels: { color: '#94a3b8' } } },
                scales: {
                  y: { min: 0, ticks: { color: '#94a3b8' }, grid: { color: '#334155' } },
                  x: { ticks: { color: '#94a3b8' }, grid: { color: '#334155' } },
                },
              }}
            />
          </div>
        </div>
      )}

      {bookingsChart && (
        <div className="card">
          <h2>Брони по дням</h2>
          <div className="chart-wrap">
            <Line
              data={bookingsChart}
              options={{
                responsive: true,
                maintainAspectRatio: false,
                plugins: { legend: { position: 'top' as const, labels: { color: '#94a3b8' } } },
                scales: {
                  y: { min: 0, ticks: { color: '#94a3b8' }, grid: { color: '#334155' } },
                  x: { ticks: { color: '#94a3b8' }, grid: { color: '#334155' } },
                },
              }}
            />
          </div>
        </div>
      )}

      <div className="grid2 owner-dishes">
        <div className="card">
          <h3>Популярные блюда</h3>
          <ul className="owner-list">
            {(analytics?.top_dishes || []).map((d, i) => (
              <li key={i}>
                {d.name} — {d.quantity} шт., {(d.revenue_kopecks / 100).toFixed(0)} ₽
              </li>
            ))}
            {(!analytics?.top_dishes || analytics.top_dishes.length === 0) && (
              <li className="muted">Пока нет заказов по меню</li>
            )}
          </ul>
        </div>
        <div className="card">
          <h3>Редко заказывают</h3>
          <ul className="owner-list">
            {(analytics?.flop_dishes || []).map((d, i) => (
              <li key={i}>
                {d.name} — {d.quantity} шт.
              </li>
            ))}
            {(!analytics?.flop_dishes || analytics.flop_dishes.length === 0) && (
              <li className="muted">Нет данных</li>
            )}
          </ul>
        </div>
      </div>

      {settings && (
        <div className="card">
          <h2>Настройки бронирования</h2>
          <p className="muted compact">
            Значения из таблицы настроек ресторана: они влияют на слоты, депозит и возврат при отмене гостем.
          </p>
          <form onSubmit={(e) => void saveSettingsForm(e)} className="owner-settings-form">
            {SETTING_FORM_KEYS.map((k) => (
              <div key={k} className="owner-settings-field">
                <label>
                  {OWNER_SETTING_META[k].title}
                  <span className="owner-settings-hint">{OWNER_SETTING_META[k].hint}</span>
                </label>
                <input
                  type="text"
                  inputMode="decimal"
                  value={settingsForm[k] ?? ''}
                  onChange={(e) => setSettingsForm((s) => ({ ...s, [k]: e.target.value }))}
                />
              </div>
            ))}
            <button type="submit" className="btn btn-sm">
              Сохранить настройки бронирования
            </button>
            {settingsSaveMsg && <p className="form-msg">{settingsSaveMsg}</p>}
          </form>
        </div>
      )}
    </div>
  );
}
