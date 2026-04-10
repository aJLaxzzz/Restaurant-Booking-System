import { useCallback, useEffect, useState } from 'react';
import { api } from '../../api';
import { AdminBookingsTable, type AdminBookingRow } from '../../components/admin/AdminBookingsTable';

export default function AdminBookingsPage() {
  const [rows, setRows] = useState<AdminBookingRow[]>([]);

  const load = useCallback(async () => {
    const { data } = await api.get<AdminBookingRow[]>('/reservations');
    setRows(Array.isArray(data) ? data : []);
  }, []);

  useEffect(() => {
    void load();
  }, [load]);

  const checkin = async (id: string) => {
    await api.post(`/reservations/${id}/checkin`, {});
    await load();
  };

  return (
    <div className="card">
      <h2>Бронирования на сегодня</h2>
      <AdminBookingsTable rows={rows} onCheckin={checkin} />
    </div>
  );
}
