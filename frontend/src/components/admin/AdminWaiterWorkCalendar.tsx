import { useCallback, useEffect, useState } from 'react';
import axios from 'axios';
import {
  addMonths,
  eachDayOfInterval,
  endOfMonth,
  endOfWeek,
  format,
  isSameMonth,
  startOfMonth,
  startOfWeek,
} from 'date-fns';
import { ru } from 'date-fns/locale';
import { api } from '../../api';

type WaiterBrief = { id: string; full_name: string; email: string };

type Props = { waiters: WaiterBrief[] };

export function AdminWaiterWorkCalendar({ waiters }: Props) {
  const [month, setMonth] = useState(() => startOfMonth(new Date()));
  const [waiterId, setWaiterId] = useState('');
  const [dates, setDates] = useState<Set<string>>(() => new Set());
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [msg, setMsg] = useState<string | null>(null);

  useEffect(() => {
    if (waiters.length === 0) return;
    if (!waiterId || !waiters.some((w) => w.id === waiterId)) {
      setWaiterId(waiters[0].id);
    }
  }, [waiters, waiterId]);

  const monthFrom = format(startOfMonth(month), 'yyyy-MM-dd');
  const monthTo = format(endOfMonth(month), 'yyyy-MM-dd');

  const loadMonth = useCallback(async () => {
    if (!waiterId) return;
    setLoading(true);
    setMsg(null);
    try {
      const { data } = await api.get<{ dates: string[] }>(
        `/admin/waiters/${waiterId}/work-dates?from=${monthFrom}&to=${monthTo}`
      );
      setDates(new Set(Array.isArray(data.dates) ? data.dates : []));
    } catch (e: unknown) {
      if (axios.isAxiosError(e)) {
        const err = (e.response?.data as { error?: string } | undefined)?.error;
        setMsg(err || e.message || 'Не удалось загрузить график');
      } else {
        setMsg('Не удалось загрузить график');
      }
      setDates(new Set());
    } finally {
      setLoading(false);
    }
  }, [waiterId, monthFrom, monthTo]);

  useEffect(() => {
    void loadMonth();
  }, [loadMonth]);

  const toggleDay = (dayStr: string) => {
    setDates((prev) => {
      const next = new Set(prev);
      if (next.has(dayStr)) next.delete(dayStr);
      else next.add(dayStr);
      return next;
    });
    setMsg(null);
  };

  const save = async () => {
    if (!waiterId) return;
    setSaving(true);
    setMsg(null);
    try {
      const inMonth = Array.from(dates).filter((d) => d >= monthFrom && d <= monthTo);
      await api.put(`/admin/waiters/${waiterId}/work-dates`, {
        from: monthFrom,
        to: monthTo,
        dates: inMonth,
      });
      setMsg('Сохранено');
      await loadMonth();
    } catch (e: unknown) {
      if (axios.isAxiosError(e)) {
        const err = (e.response?.data as { error?: string } | undefined)?.error;
        setMsg(err || 'Ошибка сохранения');
      } else {
        setMsg('Ошибка сохранения');
      }
    } finally {
      setSaving(false);
    }
  };

  if (waiters.length === 0) return null;

  const mStart = startOfMonth(month);
  const mEnd = endOfMonth(month);
  const gridStart = startOfWeek(mStart, { weekStartsOn: 1 });
  const gridEnd = endOfWeek(mEnd, { weekStartsOn: 1 });
  const days = eachDayOfInterval({ start: gridStart, end: gridEnd });
  const weekDays = ['Пн', 'Вт', 'Ср', 'Чт', 'Пт', 'Сб', 'Вс'];

  return (
    <div className="admin-work-cal">
      <h3>Рабочие дни по календарю</h3>
      <p className="muted compact">
        Отметьте дни, когда официант выходит на смену (плановый график). На назначение столов и брони это пока не влияет.
      </p>
      <div className="admin-work-cal-toolbar">
        <label className="admin-work-cal-label">
          Официант
          <select value={waiterId} onChange={(e) => setWaiterId(e.target.value)} disabled={loading || saving}>
            {waiters.map((w) => (
              <option key={w.id} value={w.id}>
                {w.full_name} ({w.email})
              </option>
            ))}
          </select>
        </label>
        <div className="btn-row" style={{ flexWrap: 'wrap', gap: 8 }}>
          <button
            type="button"
            className="secondary btn-sm"
            disabled={loading || saving}
            onClick={() => setMonth((m) => addMonths(m, -1))}
          >
            ← Месяц
          </button>
          <span className="admin-work-cal-month">{format(month, 'LLLL yyyy', { locale: ru })}</span>
          <button
            type="button"
            className="secondary btn-sm"
            disabled={loading || saving}
            onClick={() => setMonth((m) => addMonths(m, 1))}
          >
            Месяц →
          </button>
          <button type="button" className="btn btn-sm" disabled={loading || saving} onClick={() => void save()}>
            {saving ? 'Сохранение…' : 'Сохранить месяц'}
          </button>
        </div>
      </div>
      {msg && <p className="form-msg">{msg}</p>}
      {loading ? (
        <p className="muted">Загрузка…</p>
      ) : (
        <div className="admin-work-cal-grid" role="grid" aria-label="Календарь рабочих дней">
          {weekDays.map((wd) => (
            <div key={wd} className="admin-work-cal-weekday">
              {wd}
            </div>
          ))}
          {days.map((d) => {
            const key = format(d, 'yyyy-MM-dd');
            const inMonth = isSameMonth(d, month);
            const on = dates.has(key);
            return (
              <button
                key={key}
                type="button"
                role="gridcell"
                disabled={!inMonth || saving}
                className={`admin-work-cal-cell${!inMonth ? ' admin-work-cal-cell--muted' : ''}${on ? ' admin-work-cal-cell--on' : ''}`}
                onClick={() => inMonth && toggleDay(key)}
              >
                {format(d, 'd')}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}
