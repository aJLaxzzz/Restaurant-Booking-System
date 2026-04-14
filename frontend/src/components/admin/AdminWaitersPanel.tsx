import { useCallback, useEffect, useState } from 'react';
import { parseISO } from 'date-fns';
import { ru } from 'date-fns/locale';
import { formatInTimeZone } from 'date-fns-tz';
import { api } from '../../api';
import { reservationStatusLabelRu } from '../../utils/reservationStatus';
import { AdminWaiterWorkCalendar } from './AdminWaiterWorkCalendar';

const moscowTZ = 'Europe/Moscow';

type ResBrief = {
  id: string;
  table_number: number;
  status: string;
  start_time: string;
};

type WaiterRow = {
  id: string;
  email: string;
  full_name: string;
  phone: string;
  scheduled_today?: boolean;
  today_reservations: ResBrief[] | null;
};

function formatStart(iso: string) {
  return formatInTimeZone(parseISO(iso), moscowTZ, 'HH:mm', { locale: ru });
}

export function AdminWaitersPanel() {
  const [rows, setRows] = useState<WaiterRow[]>([]);
  const [err, setErr] = useState<string | null>(null);
  const [busy, setBusy] = useState<string | null>(null);
  const [ratingById, setRatingById] = useState<Record<string, { avg?: number | null; count: number }>>({});

  const load = useCallback(async () => {
    setErr(null);
    try {
      const [w, r] = await Promise.all([
        api.get<WaiterRow[]>('/admin/waiters'),
        api.get<{ waiters?: { id: string; avg?: number | null; count: number }[] }>('/admin/waiters/ratings'),
      ]);
      const data = w.data;
      setRows(Array.isArray(data) ? data : []);
      const list = r.data?.waiters;
      const map: Record<string, { avg?: number | null; count: number }> = {};
      if (Array.isArray(list)) {
        for (const x of list) {
          if (x && typeof x.id === 'string') map[x.id] = { avg: x.avg ?? null, count: Number(x.count) || 0 };
        }
      }
      setRatingById(map);
    } catch {
      setErr('Не удалось загрузить список официантов');
      setRows([]);
      setRatingById({});
    }
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const dismiss = async (email: string) => {
    setBusy(email);
    setErr(null);
    try {
      await api.post('/admin/staff/assign', { email, role: 'client' });
      await load();
    } catch (ex: unknown) {
      const m = ex as { response?: { data?: { error?: string } } };
      setErr(m.response?.data?.error || 'Не удалось снять доступ');
    } finally {
      setBusy(null);
    }
  };

  if (err && rows.length === 0) {
    return <p className="form-msg">{err}</p>;
  }

  return (
    <>
      {err && <p className="form-msg">{err}</p>}
      <p className="muted compact">
        Брони в таблице — назначения на сегодня (МСК), где официант указан ответственным.
      </p>
      <div className="table-wrap">
        <table className="data-table">
          <thead>
            <tr>
              <th>Имя</th>
              <th>Email</th>
              <th>Телефон</th>
              <th>Рейтинг</th>
              <th>Сегодня (столы)</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {rows.map((w) => {
              const list = w.today_reservations ?? [];
              const scheduled = w.scheduled_today === true;
              const brief = !scheduled
                ? 'выходной'
                : list.length === 0
                  ? 'нет назначенных столов'
                  : list
                      .map(
                        (r) =>
                          `№${r.table_number} ${formatStart(r.start_time)} (${reservationStatusLabelRu(r.status)})`,
                      )
                      .join('; ');
              const rr = ratingById[w.id];
              const ratingText =
                rr && rr.count > 0 && typeof rr.avg === 'number' ? `★ ${rr.avg.toFixed(1)} (${rr.count})` : '—';
              return (
                <tr key={w.id}>
                  <td>{w.full_name}</td>
                  <td>
                    <code>{w.email}</code>
                  </td>
                  <td>{w.phone || '—'}</td>
                  <td className="muted">{ratingText}</td>
                  <td className="muted" style={{ maxWidth: 360, whiteSpace: 'normal', fontSize: '0.88rem' }}>
                    {brief}
                  </td>
                  <td>
                    <button
                      type="button"
                      className="secondary btn-sm"
                      disabled={busy === w.email}
                      onClick={() => void dismiss(w.email)}
                    >
                      Снять доступ
                    </button>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
      {rows.length === 0 && !err && <p className="muted">Нет официантов в этом заведении</p>}
      {rows.length > 0 && (
        <AdminWaiterWorkCalendar
          waiters={rows.map((w) => ({ id: w.id, full_name: w.full_name, email: w.email }))}
          onSaved={() => void load()}
        />
      )}
    </>
  );
}
