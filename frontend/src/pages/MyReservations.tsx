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
    <div className="page-stack">
      <div className="card">
        <h2>Мои брони</h2>
        <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th>Стол</th>
                <th>Начало</th>
                <th>Гости</th>
                <th>Статус</th>
                <th>Счёт</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {rows.map((r) => (
                <ReservationRow key={r.id} r={r} onCancel={cancel} />
              ))}
            </tbody>
          </table>
        </div>
        {rows.length === 0 && <p className="muted">Нет активных броней</p>}
      </div>
    </div>
  );
}

function ReservationRow({ r, onCancel }: { r: Row; onCancel: (id: string) => void }) {
  const [orderTotal, setOrderTotal] = useState<number | null>(null);
  const [orderOpen, setOrderOpen] = useState(false);

  useEffect(() => {
    void (async () => {
      try {
        const { data } = await api.get<{ total_kopecks: number; order: { status: string } | null }>(
          `/reservations/${r.id}/order`
        );
        setOrderTotal(data.total_kopecks);
        setOrderOpen(data.order != null && data.order.status === 'open');
      } catch {
        setOrderTotal(null);
      }
    })();
  }, [r.id, r.status]);

  const payTab = async () => {
    const { data } = await api.post<{ payment_id: string }>(`/reservations/${r.id}/order/checkout`);
    window.location.href = `/pay/${data.payment_id}`;
  };

  const showPay = orderTotal != null && orderTotal > 0 && orderOpen && (r.status === 'seated' || r.status === 'in_service');

  return (
    <tr>
      <td>№{r.table_number}</td>
      <td>{format(parseISO(r.start_time), 'dd.MM.yyyy HH:mm')}</td>
      <td>{r.guest_count}</td>
      <td>
        <span className="status-pill">{r.status}</span>
      </td>
      <td>
        {orderTotal != null && orderTotal > 0 ? (
          <span>{(orderTotal / 100).toFixed(0)} ₽</span>
        ) : (
          <span className="muted">—</span>
        )}
      </td>
      <td>
        <div className="btn-row tight">
          {showPay && (
            <button type="button" className="btn btn-sm" onClick={() => void payTab()}>
              Оплатить счёт
            </button>
          )}
          {(r.status === 'pending_payment' || r.status === 'confirmed') && (
            <button type="button" className="secondary btn-sm" onClick={() => void onCancel(r.id)}>
              Отменить
            </button>
          )}
        </div>
      </td>
    </tr>
  );
}
