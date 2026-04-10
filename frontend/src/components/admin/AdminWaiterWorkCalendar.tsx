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

const PALETTE = ['#818cf8', '#34d399', '#fbbf24', '#f472b6', '#38bdf8', '#a78bfa'];

function colorFor(waiters: WaiterBrief[], id: string): string {
  const idx = waiters.findIndex((w) => w.id === id);
  return PALETTE[(idx >= 0 ? idx : 0) % PALETTE.length];
}

export function AdminWaiterWorkCalendar({ waiters }: Props) {
  const [month, setMonth] = useState(() => startOfMonth(new Date()));
  const [selectedIds, setSelectedIds] = useState<Set<string>>(() => new Set());
  const [byWaiter, setByWaiter] = useState<Record<string, Set<string>>>({});
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [msg, setMsg] = useState<string | null>(null);

  useEffect(() => {
    if (waiters.length === 0) return;
    setSelectedIds((prev) => {
      if (prev.size > 0) {
        const next = new Set<string>();
        for (const id of prev) {
          if (waiters.some((w) => w.id === id)) next.add(id);
        }
        return next.size > 0 ? next : new Set(waiters.map((w) => w.id));
      }
      return new Set(waiters.map((w) => w.id));
    });
  }, [waiters]);

  const monthFrom = format(startOfMonth(month), 'yyyy-MM-dd');
  const monthTo = format(endOfMonth(month), 'yyyy-MM-dd');

  const loadMonth = useCallback(async () => {
    if (waiters.length === 0) return;
    setLoading(true);
    setMsg(null);
    try {
      const { data } = await api.get<{ by_waiter?: Record<string, string[]> }>(
        `/admin/waiters/work-dates?from=${monthFrom}&to=${monthTo}`,
      );
      const bw = data.by_waiter ?? {};
      const next: Record<string, Set<string>> = {};
      for (const w of waiters) {
        next[w.id] = new Set(Array.isArray(bw[w.id]) ? bw[w.id] : []);
      }
      setByWaiter(next);
    } catch (e: unknown) {
      if (axios.isAxiosError(e)) {
        const err = (e.response?.data as { error?: string } | undefined)?.error;
        setMsg(err || e.message || 'Не удалось загрузить график');
      } else {
        setMsg('Не удалось загрузить график');
      }
      setByWaiter({});
    } finally {
      setLoading(false);
    }
  }, [waiters, monthFrom, monthTo]);

  useEffect(() => {
    void loadMonth();
  }, [loadMonth]);

  const toggleSelectWaiter = (id: string) => {
    setSelectedIds((prev) => {
      const n = new Set(prev);
      if (n.has(id)) n.delete(id);
      else n.add(id);
      return n;
    });
    setMsg(null);
  };

  const toggleDay = (dayStr: string) => {
    if (selectedIds.size === 0) {
      setMsg('Отметьте хотя бы одного официанта слева');
      return;
    }
    setByWaiter((prev) => {
      const out = { ...prev };
      for (const wid of selectedIds) {
        const cur = new Set(out[wid] ?? []);
        if (cur.has(dayStr)) cur.delete(dayStr);
        else cur.add(dayStr);
        out[wid] = cur;
      }
      return out;
    });
    setMsg(null);
  };

  const save = async () => {
    if (waiters.length === 0) return;
    setSaving(true);
    setMsg(null);
    try {
      for (const w of waiters) {
        const set = byWaiter[w.id] ?? new Set();
        const inMonth = Array.from(set).filter((d) => d >= monthFrom && d <= monthTo);
        await api.put(`/admin/waiters/${w.id}/work-dates`, {
          from: monthFrom,
          to: monthTo,
          dates: inMonth,
        });
      }
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

  const waitersOnDay = useCallback(
    (dayStr: string) => waiters.filter((w) => byWaiter[w.id]?.has(dayStr)),
    [waiters, byWaiter],
  );

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
        Слева отметьте официантов — клик по дню включает/выключает смену для всех выбранных. Цвета в ячейке
        совпадают с легендой (до 6 цветов).
      </p>
      <div className="admin-work-cal-main">
        <div className="admin-work-cal-side">
          <span className="admin-work-cal-side-title">Официанты</span>
          {waiters.map((w) => (
            <label key={w.id} className="admin-work-cal-waiter-chk admin-menu-pos-check">
              <input
                type="checkbox"
                className="admin-checkbox-input"
                checked={selectedIds.has(w.id)}
                onChange={() => toggleSelectWaiter(w.id)}
                disabled={loading || saving}
              />
              <span className="admin-work-cal-waiter-dot" style={{ background: colorFor(waiters, w.id) }} />
              <span>
                {w.full_name} <span className="muted">({w.email})</span>
              </span>
            </label>
          ))}
        </div>
        <div className="admin-work-cal-calendar">
          <div className="admin-work-cal-toolbar">
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
                {saving ? 'Сохранение…' : 'Сохранить всех'}
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
                const onList = waitersOnDay(key);
                const on = onList.length > 0;
                return (
                  <button
                    key={key}
                    type="button"
                    role="gridcell"
                    disabled={!inMonth || saving}
                    className={`admin-work-cal-cell${!inMonth ? ' admin-work-cal-cell--muted' : ''}${on ? ' admin-work-cal-cell--on' : ''}`}
                    onClick={() => inMonth && toggleDay(key)}
                  >
                    <span className="admin-work-cal-cell-day">{format(d, 'd')}</span>
                    {onList.length > 0 && (
                      <div className="admin-work-cal-cell-strips" aria-hidden>
                        {onList.slice(0, 3).map((w) => (
                          <span
                            key={w.id}
                            className="admin-work-cal-strip"
                            style={{ background: colorFor(waiters, w.id) }}
                          />
                        ))}
                      </div>
                    )}
                  </button>
                );
              })}
            </div>
          )}
          <div className="admin-work-cal-legend">
            {waiters.map((w) => (
              <span key={w.id} className="admin-work-cal-legend-item">
                <span className="admin-work-cal-strip" style={{ background: colorFor(waiters, w.id) }} />
                {w.full_name}
              </span>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}
