import { useCallback, useEffect, useMemo, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { HallCanvas, TableShape } from '../components/HallCanvas';
import { tableStatusLabelRu } from '../utils/reservationStatus';
import { api } from '../api';
import { useAuth } from '../auth';
import { addHours, format, isValid, parse } from 'date-fns';
import { ru } from 'date-fns/locale';
import { formatInTimeZone, toDate } from 'date-fns-tz';

const MSK_TZ = 'Europe/Moscow';

type BookingDefaults = {
  default_slot_duration_hours: number;
  booking_open_hour: number;
  booking_close_hour: number;
  slot_minutes: number;
};

const FALLBACK_DEFAULTS: BookingDefaults = {
  default_slot_duration_hours: 2,
  booking_open_hour: 10,
  booking_close_hour: 23,
  slot_minutes: 30,
};

function pad2(n: number) {
  return n.toString().padStart(2, '0');
}

/** Сетка времени начала визита из настроек (booking_open_hour … booking_close_hour, шаг slot_minutes). */
function buildTimeSlots(openH: number, closeH: number, slotMinutes: number): string[] {
  const startM = openH * 60;
  let endM = closeH === 0 || closeH === 24 ? 24 * 60 : closeH * 60;
  if (endM < startM) endM = 24 * 60;
  const out: string[] = [];
  for (let t = startM; t <= endM; t += slotMinutes) {
    if (t >= 24 * 60) break;
    const hh = Math.floor(t / 60);
    const mm = t % 60;
    out.push(`${pad2(hh)}:${pad2(mm)}`);
  }
  return out;
}

/** Начало слота в ISO UTC: дата и время интерпретируются как московские (как на бэкенде). */
function buildStartISOMoscow(dateStr: string, timeStr: string): string {
  const d = toDate(`${dateStr}T${timeStr}:00`, { timeZone: MSK_TZ });
  if (!isValid(d)) {
    throw new Error('Некорректная дата и время');
  }
  return d.toISOString();
}

/** Сообщение об ошибке или null, если слот можно запрашивать. */
function validateBookableSlot(dateStr: string, timeStr: string): string | null {
  let start: Date;
  try {
    start = toDate(`${dateStr}T${timeStr}:00`, { timeZone: MSK_TZ });
  } catch {
    return 'Некорректная дата или время';
  }
  if (!isValid(start)) return 'Некорректная дата или время';
  const todayMsk = formatInTimeZone(new Date(), MSK_TZ, 'yyyy-MM-dd');
  if (dateStr < todayMsk) {
    return 'Нельзя бронировать на прошедшую дату или время';
  }
  if (start.getTime() < Date.now() - 90_000) {
    return 'Нельзя бронировать на прошедшую дату или время';
  }
  return null;
}

type HallOpt = { id: string; name: string; restaurant: string; restaurant_id: string };

export default function HallPage() {
  const { user } = useAuth();
  const [searchParams, setSearchParams] = useSearchParams();
  const [hallId, setHallId] = useState<string | null>(null);
  const [hallsList, setHallsList] = useState<HallOpt[]>([]);
  const [hallsLoading, setHallsLoading] = useState(true);
  const editMode = Boolean(
    user &&
      (user.role === 'admin' || user.role === 'owner' || user.role === 'superadmin') &&
      searchParams.get('edit') === '1'
  );

  const [step, setStep] = useState<1 | 2 | 3>(1);
  const [date, setDate] = useState(() => formatInTimeZone(new Date(), MSK_TZ, 'yyyy-MM-dd'));
  const [time, setTime] = useState('19:00');
  const [availabilityById, setAvailabilityById] = useState<Record<string, boolean> | null>(null);
  const [selected, setSelected] = useState<TableShape | null>(null);
  const [comment, setComment] = useState('');
  const [wizardBanner, setWizardBanner] = useState<{ text: string; tone: 'success' | 'error' | 'info' } | null>(null);
  const [payId, setPayId] = useState<string | null>(null);
  const [loadingAvail, setLoadingAvail] = useState(false);
  const [bookingDefs, setBookingDefs] = useState<BookingDefaults | null>(null);
  const [guestsStr, setGuestsStr] = useState('2');

  const restaurantIdForBookingDefaults = useMemo(
    () => (hallId ? hallsList.find((h) => h.id === hallId)?.restaurant_id : undefined),
    [hallId, hallsList],
  );

  useEffect(() => {
    void (async () => {
      try {
        const { data } = await api.get<BookingDefaults>('/booking-defaults', {
          params: restaurantIdForBookingDefaults ? { restaurant_id: restaurantIdForBookingDefaults } : {},
        });
        setBookingDefs({
          default_slot_duration_hours: data.default_slot_duration_hours ?? FALLBACK_DEFAULTS.default_slot_duration_hours,
          booking_open_hour: data.booking_open_hour ?? FALLBACK_DEFAULTS.booking_open_hour,
          booking_close_hour: data.booking_close_hour ?? FALLBACK_DEFAULTS.booking_close_hour,
          slot_minutes: data.slot_minutes ?? FALLBACK_DEFAULTS.slot_minutes,
        });
      } catch {
        setBookingDefs(FALLBACK_DEFAULTS);
      }
    })();
  }, [restaurantIdForBookingDefaults]);

  const defs = bookingDefs ?? FALLBACK_DEFAULTS;
  const slotHours = defs.default_slot_duration_hours;
  const timeSlots = useMemo(
    () => buildTimeSlots(defs.booking_open_hour, defs.booking_close_hour, defs.slot_minutes),
    [defs.booking_open_hour, defs.booking_close_hour, defs.slot_minutes]
  );

  const moscowTodayStr = formatInTimeZone(new Date(), MSK_TZ, 'yyyy-MM-dd');

  useEffect(() => {
    if (date < moscowTodayStr) setDate(moscowTodayStr);
  }, [date, moscowTodayStr]);

  const visibleTimeSlots = useMemo(() => {
    if (date !== moscowTodayStr) return timeSlots;
    const nowMs = Date.now();
    return timeSlots.filter((ts) => {
      try {
        const iso = buildStartISOMoscow(date, ts);
        return new Date(iso).getTime() >= nowMs - 60_000;
      } catch {
        return false;
      }
    });
  }, [timeSlots, date, moscowTodayStr]);

  const guests = useMemo(() => {
    const t = guestsStr.trim();
    if (t === '') return 1;
    const n = parseInt(t, 10);
    if (!Number.isFinite(n)) return 1;
    return Math.min(20, Math.max(1, n));
  }, [guestsStr]);

  useEffect(() => {
    if (visibleTimeSlots.length === 0) return;
    if (!visibleTimeSlots.includes(time)) setTime(visibleTimeSlots[0]);
  }, [visibleTimeSlots, time]);

  useEffect(() => {
    void (async () => {
      setHallsLoading(true);
      const rParam = searchParams.get('restaurant_id');
      const hParam = searchParams.get('hall_id');
      let restaurantFilter = rParam || undefined;
      if (hParam && !restaurantFilter) {
        try {
          const { data: hg } = await api.get<{ restaurant_id: string }>(`/halls/${hParam}`);
          restaurantFilter = hg.restaurant_id;
        } catch {
          /* ignore */
        }
      }
      try {
        const { data: halls } = await api.get<HallOpt[]>('/halls', {
          params: restaurantFilter ? { restaurant_id: restaurantFilter } : {},
        });
        setHallsList(halls);
        if (halls.length === 0) {
          setHallId(null);
          return;
        }
        const pick = hParam && halls.some((x) => x.id === hParam) ? hParam : halls[0].id;
        setHallId(pick);
      } catch {
        setHallsList([]);
        setHallId(null);
      } finally {
        setHallsLoading(false);
      }
    })();
  }, [searchParams]);

  const onHallChange = (id: string) => {
    setHallId(id);
    const next = new URLSearchParams(searchParams);
    next.set('hall_id', id);
    const r = hallsList.find((h) => h.id === id)?.restaurant_id;
    if (r) next.set('restaurant_id', r);
    setSearchParams(next);
  };

  const unlockIfNeeded = useCallback(async () => {
    if (!hallId || !selected || !user) return;
    try {
      await api.delete(`/halls/${hallId}/tables/${selected.id}/lock`);
    } catch {
      /* ignore */
    }
  }, [hallId, selected, user]);

  useEffect(() => {
    return () => {
      void unlockIfNeeded();
    };
  }, [unlockIfNeeded]);

  const loadAvailability = async () => {
    if (!hallId) return;
    setLoadingAvail(true);
    setWizardBanner(null);
    const bad = validateBookableSlot(date, time);
    if (bad) {
      setWizardBanner({ text: bad, tone: 'info' });
      setLoadingAvail(false);
      return;
    }
    try {
      const start = buildStartISOMoscow(date, time);
      const end = addHours(new Date(start), slotHours).toISOString();
      const { data } = await api.get<{ tables: { id: string; available_for_slot: boolean }[] | null }>(
        `/halls/${hallId}/availability?start=${encodeURIComponent(start)}&end=${encodeURIComponent(end)}&guests=${guests}`
      );
      const m: Record<string, boolean> = {};
      const list = Array.isArray(data.tables) ? data.tables : [];
      for (const t of list) {
        m[t.id] = t.available_for_slot;
      }
      setAvailabilityById(m);
      setStep(2);
    } catch (ex: unknown) {
      const ax = ex as { response?: { status?: number; data?: { error?: string } }; message?: string };
      const apiErr = ax.response?.data?.error;
      setWizardBanner({
        text: apiErr || ax.message || 'Не удалось загрузить доступность столов',
        tone: 'error',
      });
      if (ax.response?.status === 400) {
        setStep(1);
        setAvailabilityById(null);
      }
    } finally {
      setLoadingAvail(false);
    }
  };

  const onSelect = async (t: TableShape | null) => {
    setWizardBanner(null);
    setPayId(null);
    if (!t || !hallId) {
      setSelected(null);
      return;
    }
    if (!user) {
      setWizardBanner({ text: 'Войдите, чтобы забронировать стол', tone: 'info' });
      return;
    }
    if (user.role !== 'client' && user.role !== 'owner') {
      setWizardBanner({ text: 'Бронирование доступно только гостям и владельцу.', tone: 'info' });
      return;
    }
    if (selected && selected.id !== t.id) {
      await unlockIfNeeded();
    }
    try {
      await api.post(`/halls/${hallId}/tables/${t.id}/lock`);
      setSelected(t);
      setStep(3);
    } catch {
      setWizardBanner({ text: 'Не удалось заблокировать стол (занят другим пользователем)', tone: 'error' });
    }
  };

  const book = async () => {
    if (!selected || !user) return;
    setWizardBanner(null);
    const bad = validateBookableSlot(date, time);
    if (bad) {
      setWizardBanner({ text: bad, tone: 'info' });
      return;
    }
    const start = buildStartISOMoscow(date, time);
    const idem = crypto.randomUUID();
    try {
      const { data } = await api.post<{
        reservation_id: string;
        payment_id?: string;
        checkout_url?: string;
        no_payment_required?: boolean;
      }>('/reservations', {
        table_id: selected.id,
        start_time: start,
        guest_count: guests,
        comment,
        idempotency_key: idem,
      });
      if (data.no_payment_required) {
        setPayId(null);
        setWizardBanner({ text: 'Бронь подтверждена. Депозит не требуется.', tone: 'success' });
      } else if (data.payment_id) {
        setPayId(data.payment_id);
        setWizardBanner({ text: 'Бронь создана. Перейдите по ссылке ниже, чтобы оплатить депозит.', tone: 'success' });
      } else {
        setWizardBanner({ text: 'Бронь создана.', tone: 'success' });
      }
    } catch (ex: unknown) {
      const m = ex as { response?: { data?: { error?: string } } };
      setWizardBanner({ text: m.response?.data?.error || 'Не удалось создать бронь.', tone: 'error' });
    }
  };

  const goBackStep = async () => {
    if (step === 3) {
      await unlockIfNeeded();
      setSelected(null);
      setComment('');
      setStep(2);
      return;
    }
    if (step === 2) {
      await unlockIfNeeded();
      setSelected(null);
      setAvailabilityById(null);
      setStep(1);
    }
  };

  const resetWizard = async () => {
    await unlockIfNeeded();
    setSelected(null);
    setAvailabilityById(null);
    setComment('');
    setPayId(null);
    setWizardBanner(null);
    setStep(1);
  };

  if (hallsLoading) {
    return <p className="muted">Загрузка зала…</p>;
  }

  if (!hallId) {
    return hallsList.length === 0 ? (
      <div className="page-stack">
        <div className="card">
          <p className="muted">Нет залов для выбранного заведения.</p>
          <Link to="/" className="btn">
            К каталогу ресторанов
          </Link>
        </div>
      </div>
    ) : (
      <p className="muted">Загрузка зала…</p>
    );
  }

  /* ——— Редактор зала ——— */
  if (editMode) {
    return (
      <div className="page-stack">
        <div className="card hero-card">
          <h1>Редактор схемы зала</h1>
          <p className="muted">Перетаскивайте столы, добавляйте новые и сохраняйте. Ниже можно заблокировать стол для гостей.</p>
          <Link to="/admin" className="link-back">
            ← В панель администратора
          </Link>
        </div>
        <HallEditorPanel hallId={hallId} />
      </div>
    );
  }

  /* ——— Служебные роли без бронирования ——— */
  if (user?.role === 'admin') {
    return (
      <div className="page-stack">
        <div className="card role-gate-card">
          <h2>Управление залом</h2>
          <p>
            Администратор не оформляет брони как гость. Используйте панель: список броней, ручное создание, чек-ин гостей.
          </p>
          <div className="btn-row">
            <Link to="/admin" className="btn">
              Панель администратора
            </Link>
            <Link to="/hall?edit=1" className="btn secondary">
              Редактор схемы зала
            </Link>
          </div>
        </div>
      </div>
    );
  }

  if (user?.role === 'waiter') {
    return (
      <div className="page-stack">
        <div className="card role-gate-card">
          <h2>Зона официанта</h2>
          <p>Бронирование столов недоступно с этой учётной записи. Откройте назначенные столы и заметки.</p>
          <Link to="/waiter" className="btn">
            К столам
          </Link>
        </div>
      </div>
    );
  }

  /* ——— Мастер бронирования (гость, владелец, неавторизованный просмотр) ——— */
  return (
    <div className="page-stack">
      <div className="card hero-card">
        <h1>Забронировать стол</h1>
        <p className="muted">Сначала выберите дату, время и число гостей — затем свободный стол на схеме.</p>
        <div className="wizard-steps">
          <span className={step === 1 ? 'active' : ''}>1. Дата и гости</span>
          <span className="sep">→</span>
          <span className={step === 2 ? 'active' : ''}>2. Стол</span>
          <span className="sep">→</span>
          <span className={step === 3 ? 'active' : ''}>3. Подтверждение</span>
        </div>
      </div>

      {step === 1 && (
        <div className="card">
          <h2>Когда и сколько гостей</h2>
          {hallsList.length > 1 && hallId && (
            <div className="field-block">
              <label>Зал</label>
              <select value={hallId} onChange={(e) => onHallChange(e.target.value)}>
                {hallsList.map((h) => (
                  <option key={h.id} value={h.id}>
                    {h.restaurant} — {h.name}
                  </option>
                ))}
              </select>
            </div>
          )}
          {hallsList.length === 1 && hallId && (
            <p className="muted compact">
              {hallsList[0].restaurant} · {hallsList[0].name}
            </p>
          )}
          <div className="grid2">
            <div>
              <label>Дата</label>
              <input
                type="date"
                value={date}
                min={moscowTodayStr}
                onChange={(e) => setDate(e.target.value)}
              />
            </div>
            <div>
              <label>Время начала</label>
              <select
                value={visibleTimeSlots.includes(time) ? time : visibleTimeSlots[0] ?? ''}
                onChange={(e) => setTime(e.target.value)}
                disabled={visibleTimeSlots.length === 0}
              >
                {visibleTimeSlots.length === 0 ? (
                  <option value="">Нет доступных слотов</option>
                ) : (
                  visibleTimeSlots.map((s) => (
                    <option key={s} value={s}>
                      {s}
                    </option>
                  ))
                )}
              </select>
              {visibleTimeSlots.length === 0 && (
                <p className="hint">На выбранную дату не осталось доступных слотов по времени.</p>
              )}
            </div>
          </div>
          <label>Гости</label>
          <input
            type="text"
            inputMode="numeric"
            autoComplete="off"
            value={guestsStr}
            onChange={(e) => {
              const v = e.target.value;
              if (v === '' || /^\d{0,2}$/.test(v)) setGuestsStr(v);
            }}
            onBlur={() => {
              const n = parseInt(guestsStr, 10);
              if (!Number.isFinite(n) || n < 1) setGuestsStr('1');
              else setGuestsStr(String(Math.min(20, n)));
            }}
          />
          <p className="hint">
            Длительность визита: {slotHours} ч. Показываются столы с достаточной вместимостью.
          </p>
          <button
            type="button"
            className="btn"
            disabled={loadingAvail || visibleTimeSlots.length === 0}
            onClick={() => void loadAvailability()}
          >
            {loadingAvail ? 'Загрузка…' : 'Показать свободные столы'}
          </button>
          {wizardBanner && step === 1 && (
            <div className={`pay-flash pay-flash--${wizardBanner.tone}`} role="status" style={{ marginTop: '0.75rem' }}>
              {wizardBanner.text}
            </div>
          )}
        </div>
      )}

      {(step === 2 || step === 3) && (
        <>
          <div className="card">
            <div className="hall-card-head">
              <div>
                <h2>Схема зала</h2>
                <p className="muted compact">
                  {format(parse(date, 'yyyy-MM-dd', new Date()), 'd MMMM', { locale: ru })}, {time} · {guests}{' '}
                  {guests === 1 ? 'гость' : guests < 5 ? 'гостя' : 'гостей'}
                </p>
              </div>
              <button type="button" className="secondary btn-sm" onClick={() => void goBackStep()}>
                ← Назад
              </button>
            </div>
            <div className="legend">
              <span>
                <i className="dot" style={{ background: '#2d8f47' }} />
                подходит по слоту
              </span>
              <span>
                <i className="dot" style={{ background: '#94a3b8' }} />
                занят / не подходит
              </span>
              <span>
                <i className="dot" style={{ background: '#888' }} />
                заблокирован
              </span>
            </div>
            <HallCanvas
              hallId={hallId}
              editMode={false}
              onTableSelect={onSelect}
              selectedId={selected?.id}
              availabilityById={availabilityById}
            />
            {!user && (
              <p className="hint">
                <Link to="/login">Войдите</Link>, чтобы выбрать стол и забронировать.
              </p>
            )}
            {user && (user.role === 'client' || user.role === 'owner') && step === 2 && (
              <p className="hint">Нажмите на подходящий стол, чтобы перейти к подтверждению.</p>
            )}
          </div>

          {step === 3 && selected && (
            <div className="card confirm-card">
              <h3>Стол №{selected.number}</h3>
              <p>
                До <strong>{selected.capacity}</strong> гостей · слот {slotHours} ч
              </p>
              <label>Комментарий для ресторана</label>
              <textarea rows={3} value={comment} onChange={(e) => setComment(e.target.value)} placeholder="Особые пожелания" />
              <div className="btn-row">
                <button type="button" className="btn" onClick={() => void book()}>
                  Забронировать
                </button>
                <button type="button" className="secondary" onClick={() => void resetWizard()}>
                  Начать сначала
                </button>
              </div>
              {payId && (
                <div className="pay-deposit-link">
                  <Link to={`/pay/${payId}`}>Перейти к оплате депозита →</Link>
                </div>
              )}
              {wizardBanner && (
                <div className={`pay-flash pay-flash--${wizardBanner.tone}`} role="status" style={{ marginTop: '0.75rem' }}>
                  {wizardBanner.text}
                </div>
              )}
            </div>
          )}
        </>
      )}
    </div>
  );
}

function HallEditorPanel({ hallId }: { hallId: string }) {
  const [selected, setSelected] = useState<TableShape | null>(null);
  const [msg, setMsg] = useState('');
  const [reloadNonce, setReloadNonce] = useState(0);
  const [tblNumStr, setTblNumStr] = useState('1');
  const [tblCapStr, setTblCapStr] = useState('4');
  const [tblShape, setTblShape] = useState('rect');
  const [tblWStr, setTblWStr] = useState('88');
  const [tblHStr, setTblHStr] = useState('64');
  const [tblRotStr, setTblRotStr] = useState('0');

  const deleteTable = async () => {
    if (!selected) return;
    if (!confirm(`Удалить стол №${selected.number}? Это действие необратимо.`)) return;
    setMsg('');
    try {
      await api.delete(`/halls/${hallId}/tables/${selected.id}`);
      setSelected(null);
      setReloadNonce((n) => n + 1);
    } catch (e: unknown) {
      const ax = e as { response?: { data?: { error?: string } } };
      setMsg(ax.response?.data?.error || 'Не удалось удалить стол');
    }
  };

  const toggleBlock = async () => {
    if (!selected) return;
    setMsg('');
    const next = selected.status === 'blocked' ? 'available' : 'blocked';
    try {
      await api.put(`/halls/${hallId}/tables/${selected.id}`, { status: next });
      setSelected({ ...selected, status: next });
    } catch {
      setMsg('Не удалось обновить стол');
    }
  };

  useEffect(() => {
    if (!selected) return;
    const w = selected.width && selected.width > 0 ? selected.width : (selected.radius ?? 28) * 2;
    const h = selected.height && selected.height > 0 ? selected.height : (selected.radius ?? 28) * 2;
    setTblNumStr(String(selected.number));
    setTblCapStr(String(selected.capacity));
    setTblShape((selected.shape || 'rect').toLowerCase());
    setTblWStr(String(Math.round(w)));
    setTblHStr(String(Math.round(h)));
    setTblRotStr(String(Math.round(selected.rotation_deg ?? 0)));
  }, [selected]);

  const applyTableGeometry = async () => {
    if (!selected) return;
    setMsg('');
    const num = parseInt(tblNumStr, 10);
    const cap = parseInt(tblCapStr, 10);
    const tw = parseInt(tblWStr, 10);
    const th = parseInt(tblHStr, 10);
    const rot = parseFloat(tblRotStr.replace(',', '.'));
    if (!Number.isFinite(num) || num < 1) {
      setMsg('Некорректный номер стола');
      return;
    }
    if (!Number.isFinite(cap) || cap < 1 || cap > 32) {
      setMsg('Некорректная вместимость');
      return;
    }
    if (!Number.isFinite(tw) || tw < 24 || !Number.isFinite(th) || th < 24) {
      setMsg('Некорректные размеры');
      return;
    }
    if (!Number.isFinite(rot) || rot < -180 || rot > 180) {
      setMsg('Некорректный поворот');
      return;
    }
    try {
      await api.put(`/halls/${hallId}/tables/${selected.id}`, {
        table_number: num,
        capacity: cap,
        shape: tblShape,
        width: tw,
        height: th,
        rotation_deg: rot,
      });
      setReloadNonce((n) => n + 1);
    } catch {
      setMsg('Не удалось сохранить параметры стола');
    }
  };

  return (
    <>
      <div className="card">
        <HallCanvas
          hallId={hallId}
          editMode
          selectedId={selected?.id}
          onTableSelect={(t) => setSelected(t)}
          reloadNonce={reloadNonce}
        />
      </div>
      {selected && (
        <div className="card">
          <h3>Стол №{selected.number}</h3>
          <p>
            Статус: <strong>{tableStatusLabelRu(selected.status)}</strong>
          </p>
          <div className="hall-editor-table-form">
            <label>
              Номер стола
              <input
                type="text"
                inputMode="numeric"
                value={tblNumStr}
                onChange={(e) => {
                  const v = e.target.value;
                  if (v === '' || /^\d{1,3}$/.test(v)) setTblNumStr(v);
                }}
                onBlur={() => {
                  const n = parseInt(tblNumStr, 10);
                  if (!Number.isFinite(n) || n < 1) setTblNumStr('1');
                  else setTblNumStr(String(n));
                }}
              />
            </label>
            <label>
              Вместимость
              <input
                type="text"
                inputMode="numeric"
                value={tblCapStr}
                onChange={(e) => {
                  const v = e.target.value;
                  if (v === '' || /^\d{1,2}$/.test(v)) setTblCapStr(v);
                }}
                onBlur={() => {
                  const n = parseInt(tblCapStr, 10);
                  if (!Number.isFinite(n) || n < 1) setTblCapStr('1');
                  else setTblCapStr(String(Math.min(32, n)));
                }}
              />
            </label>
            <label>
              Форма
              <select value={tblShape} onChange={(e) => setTblShape(e.target.value)}>
                <option value="round">Круг</option>
                <option value="rect">Прямоугольник</option>
                <option value="ellipse">Эллипс</option>
              </select>
            </label>
            <label>
              Ширина, px
              <input
                type="text"
                inputMode="numeric"
                value={tblWStr}
                onChange={(e) => {
                  const v = e.target.value;
                  if (v === '' || /^\d{1,3}$/.test(v)) setTblWStr(v);
                }}
                onBlur={() => {
                  const n = parseInt(tblWStr, 10);
                  if (!Number.isFinite(n) || n < 24) setTblWStr('24');
                  else setTblWStr(String(Math.min(400, n)));
                }}
              />
            </label>
            <label>
              Высота, px
              <input
                type="text"
                inputMode="numeric"
                value={tblHStr}
                onChange={(e) => {
                  const v = e.target.value;
                  if (v === '' || /^\d{1,3}$/.test(v)) setTblHStr(v);
                }}
                onBlur={() => {
                  const n = parseInt(tblHStr, 10);
                  if (!Number.isFinite(n) || n < 24) setTblHStr('24');
                  else setTblHStr(String(Math.min(400, n)));
                }}
              />
            </label>
            <label>
              Поворот, °
              <input
                type="text"
                inputMode="decimal"
                value={tblRotStr}
                onChange={(e) => {
                  const v = e.target.value;
                  if (v === '' || v === '-' || /^-?\d{0,3}$/.test(v)) setTblRotStr(v);
                }}
                onBlur={() => {
                  const n = parseFloat(tblRotStr.replace(',', '.'));
                  if (!Number.isFinite(n)) setTblRotStr('0');
                  else setTblRotStr(String(Math.max(-180, Math.min(180, Math.round(n)))));
                }}
              />
            </label>
          </div>
          <div className="btn-row" style={{ marginTop: 12 }}>
            <button type="button" className="btn btn-sm" onClick={() => void applyTableGeometry()}>
              Применить форму и размер
            </button>
          </div>
          <div className="btn-row">
            <button type="button" className={selected.status === 'blocked' ? 'btn' : 'secondary'} onClick={() => void toggleBlock()}>
              {selected.status === 'blocked' ? 'Снять блокировку' : 'Заблокировать стол'}
            </button>
            <button type="button" className="secondary" onClick={() => void deleteTable()}>
              Удалить стол
            </button>
          </div>
          {msg && <p className="form-msg">{msg}</p>}
          <p className="hint">Заблокированные столы не доступны для онлайн-брони. После изменения геометрии нажмите «Сохранить схему» на полотне при необходимости.</p>
        </div>
      )}
    </>
  );
}
