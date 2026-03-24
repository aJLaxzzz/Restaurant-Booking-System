import { useEffect, useState } from 'react';
import { api } from '../api';
import { format, parseISO } from 'date-fns';

type Row = {
  id: string;
  table_number: number;
  start_time: string;
  end_time: string;
  guest_count: number;
  status: string;
};

export default function MyReservations() {
  const [rows, setRows] = useState<Row[]>([]);

  const load = async () => {
    const { data } = await api.get<Row[]>('/reservations/my');
    setRows(data);
  };

  useEffect(() => {
    void load();
  }, []);

  const cancel = async (id: string) => {
    if (!confirm('Отменить бронь?')) return;
    await api.delete(`/reservations/${id}`);
    await load();
  };

  return (
    <div className="card">
      <h2>Мои брони</h2>
      <table style={{ width: '100%', borderCollapse: 'collapse' }}>
        <thead>
          <tr style={{ textAlign: 'left', borderBottom: '1px solid #ddd' }}>
            <th>Стол</th>
            <th>Начало</th>
            <th>Гости</th>
            <th>Статус</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {rows.map((r) => (
            <tr key={r.id} style={{ borderBottom: '1px solid #eee' }}>
              <td>№{r.table_number}</td>
              <td>{format(parseISO(r.start_time), 'dd.MM.yyyy HH:mm')}</td>
              <td>{r.guest_count}</td>
              <td>{r.status}</td>
              <td>
                {(r.status === 'pending_payment' || r.status === 'confirmed') && (
                  <button type="button" className="secondary" onClick={() => void cancel(r.id)}>
                    Отменить
                  </button>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
      {rows.length === 0 && <p>Нет активных броней</p>}
    </div>
  );
}
