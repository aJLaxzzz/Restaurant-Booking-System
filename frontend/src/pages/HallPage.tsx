import { useCallback, useEffect, useState } from 'react';
import { Link, useSearchParams } from 'react-router-dom';
import { HallCanvas, TableShape } from '../components/HallCanvas';
import { api } from '../api';
import { useAuth } from '../auth';
import { addHours, format, parse } from 'date-fns';
import { ru } from 'date-fns/locale';

const SLOT_HOURS = 2;

function slots(): string[] {
  const out: string[] = [];
  for (let h = 10; h < 23; h++) {
    out.push(`${h.toString().padStart(2, '0')}:00`);
    out.push(`${h.toString().padStart(2, '0')}:30`);
  }
  out.push('23:00');
  return out;
}

function buildStartISO(dateStr: string, timeStr: string): string {
  const day = parse(dateStr, 'yyyy-MM-dd', new Date());
  const [hh, mm] = timeStr.split(':').map(Number);
  day.setHours(hh, mm, 0, 0);
  return day.toISOString();
}

export default function HallPage() {
  const { user } = useAuth();
  const [searchParams] = useSearchParams();
  const [hallId, setHallId] = useState<string | null>(null);
  const editMode = Boolean(
    user && (user.role === 'admin' || user.role === 'owner') && searchParams.get('edit') === '1'
  );

  const [step, setStep] = useState<1 | 2 | 3>(1);
  const [date, setDate] = useState(format(new Date(), 'yyyy-MM-dd'));
  const [time, setTime] = useState('19:00');
  const [guests, setGuests] = useState(2);
  const [availabilityById, setAvailabilityById] = useState<Record<string, boolean> | null>(null);
  const [selected, setSelected] = useState<TableShape | null>(null);
  const [comment, setComment] = useState('');
  const [msg, setMsg] = useState('');
  const [payId, setPayId] = useState<string | null>(null);
  const [loadingAvail, setLoadingAvail] = useState(false);

  useEffect(() => {
    void (async () => {
      const { data } = await api.get<{ id: string }[]>('/halls');
      if (data.length) setHallId(data[0].id);
    })();
  }, []);

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
    setMsg('');
    try {
      const start = buildStartISO(date, time);
      const end = addHours(new Date(start), SLOT_HOURS).toISOString();
      const { data } = await api.get<{ tables: { id: string; available_for_slot: boolean }[] }>(
        `/halls/${hallId}/availability?start=${encodeURIComponent(start)}&end=${encodeURIComponent(end)}&guests=${guests}`
      );
      const m: Record<string, boolean> = {};
      for (const t of data.tables) {
        m[t.id] = t.available_for_slot;
      }
      setAvailabilityById(m);
      setStep(2);
    } catch {
      setMsg('Не удалось загрузить доступность столов');
    } finally {
      setLoadingAvail(false);
    }
  };

  const onSelect = async (t: TableShape | null) => {
    setMsg('');
    setPayId(null);
    if (!t || !hallId) {
      setSelected(null);
      return;
    }
    if (!user) {
      setMsg('Войдите, чтобы забронировать стол');
      return;
    }
    if (user.role !== 'client' && user.role !== 'owner') {
      setMsg('Бронирование доступно только гостям и владельцу (тест)');
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
      setMsg('Не удалось заблокировать стол (занят другим пользователем)');
    }
  };

  const book = async () => {
    if (!selected || !user) return;
    setMsg('');
    const start = buildStartISO(date, time);
    const idem = crypto.randomUUID();
    try {
      const { data } = await api.post<{
        reservation_id: string;
        payment_id: string;
        checkout_url: string;
      }>('/reservations', {
        table_id: selected.id,
        start_time: start,
        guest_count: guests,
        comment,
        idempotency_key: idem,
      });
      setPayId(data.payment_id);
      setMsg('Бронь создана. Оплатите депозит.');
    } catch (ex: unknown) {
      const m = ex as { response?: { data?: { error?: string } } };
      setMsg(m.response?.data?.error || 'Ошибка');
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
    setMsg('');
    setStep(1);
  };

  if (!hallId) return <p className="muted">Загрузка зала…</p>;

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
          <div className="grid2">
            <div>
              <label>Дата</label>
              <input type="date" value={date} onChange={(e) => setDate(e.target.value)} />
            </div>
            <div>
              <label>Время начала</label>
              <select value={time} onChange={(e) => setTime(e.target.value)}>
                {slots().map((s) => (
                  <option key={s} value={s}>
                    {s}
                  </option>
                ))}
              </select>
            </div>
          </div>
          <label>Гости</label>
          <input
            type="number"
            min={1}
            max={20}
            value={guests}
            onChange={(e) => setGuests(Number(e.target.value))}
          />
          <p className="hint">Длительность визита в демо: {SLOT_HOURS} ч. Показываются столы с достаточной вместимостью.</p>
          <button type="button" className="btn" disabled={loadingAvail} onClick={() => void loadAvailability()}>
            {loadingAvail ? 'Загрузка…' : 'Показать свободные столы'}
          </button>
          {msg && step === 1 && <p className="form-msg">{msg}</p>}
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
                До <strong>{selected.capacity}</strong> гостей · слот {SLOT_HOURS} ч
              </p>
              <label>Комментарий для ресторана</label>
              <textarea rows={3} value={comment} onChange={(e) => setComment(e.target.value)} placeholder="Особые пожелания" />
              <div className="btn-row">
                <button type="button" className="btn" onClick={() => void book()}>
                  Забронировать и перейти к оплате
                </button>
                <button type="button" className="secondary" onClick={() => void resetWizard()}>
                  Начать сначала
                </button>
              </div>
              {payId && (
                <p className="success-msg">
                  <Link to={`/pay/${payId}`}>Перейти к оплате депозита</Link>
                </p>
              )}
              {msg && <p className="form-msg">{msg}</p>}
            </div>
          )}
        </>
      )}
    </div>
  );
}

function HallEditorPanel({ hallId }: { hallId: string }) {
  const { user } = useAuth();
  const [selected, setSelected] = useState<TableShape | null>(null);
  const [msg, setMsg] = useState('');

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

  return (
    <>
      <div className="card">
        <HallCanvas hallId={hallId} editMode selectedId={selected?.id} onTableSelect={(t) => setSelected(t)} />
      </div>
      {selected && user && (
        <div className="card">
          <h3>Стол №{selected.number}</h3>
          <p>Вместимость: {selected.capacity}</p>
          <p>
            Статус: <strong>{selected.status}</strong>
          </p>
          <div className="btn-row">
            <button type="button" className={selected.status === 'blocked' ? 'btn' : 'secondary'} onClick={() => void toggleBlock()}>
              {selected.status === 'blocked' ? 'Снять блокировку' : 'Заблокировать стол'}
            </button>
          </div>
          {msg && <p className="form-msg">{msg}</p>}
          <p className="hint">Заблокированные столы не доступны для онлайн-брони.</p>
        </div>
      )}
    </>
  );
}
