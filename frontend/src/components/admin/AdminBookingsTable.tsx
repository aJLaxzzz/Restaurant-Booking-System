import { parseISO } from 'date-fns';
import { ru } from 'date-fns/locale';
import { formatInTimeZone } from 'date-fns-tz';

const moscowTZ = 'Europe/Moscow';

export type AdminBookingRow = {
  id: string;
  client_name: string;
  phone: string;
  table_number: number;
  start_time: string;
  status: string;
};

type Props = {
  rows: AdminBookingRow[];
  onCheckin: (id: string) => void | Promise<void>;
};

/** Время начала визита в таймзоне Москвы (как на бэкенде для списка «на сегодня»). */
function formatStartMoscow(iso: string) {
  return formatInTimeZone(parseISO(iso), moscowTZ, 'd MMM HH:mm', { locale: ru });
}

export function AdminBookingsTable({ rows, onCheckin }: Props) {
  return (
    <>
      <p className="muted compact">
        Только брони на сегодняшний календарный день (время по Москве). Сортировка: от раннего слота к позднему.
      </p>
      <div className="table-wrap">
        <table className="data-table">
          <thead>
            <tr>
              <th>Клиент</th>
              <th>Телефон</th>
              <th>Стол</th>
              <th>Время (МСК)</th>
              <th>Статус</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {rows.map((r) => (
              <tr key={r.id}>
                <td>{r.client_name}</td>
                <td>{r.phone}</td>
                <td>№{r.table_number}</td>
                <td>{formatStartMoscow(r.start_time)}</td>
                <td>
                  <span className="status-pill">{r.status}</span>
                </td>
                <td>
                  {r.status === 'confirmed' && (
                    <button type="button" className="secondary btn-sm" onClick={() => void onCheckin(r.id)}>
                      Гость прибыл
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      {rows.length === 0 && <p className="muted">На сегодня нет броней</p>}
    </>
  );
}
