import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { api } from '../api';
import { format, parseISO } from 'date-fns';
import { ru } from 'date-fns/locale';

type Row = {
  id: string;
  client_name: string;
  phone: string;
  table_number: number;
  start_time: string;
  status: string;
};

type ClientOpt = { id: string; email: string; full_name: string; phone: string };

export default function AdminPage() {
  const [rows, setRows] = useState<Row[]>([]);
  const [clients, setClients] = useState<ClientOpt[]>([]);
  const [manualUser, setManualUser] = useState('');
  const [manualTable, setManualTable] = useState('');
  const [manualStart, setManualStart] = useState(format(new Date(), "yyyy-MM-dd'T'HH:mm"));
  const [manualGuests, setManualGuests] = useState(2);
  const [manualComment, setManualComment] = useState('');
  const [manualMsg, setManualMsg] = useState('');

  const load = async () => {
    const { data } = await api.get<Row[]>('/reservations');
    setRows(data);
  };

  useEffect(() => {
    void load();
  }, []);

  useEffect(() => {
    void (async () => {
      try {
        const { data } = await api.get<ClientOpt[]>('/admin/clients');
        setClients(data);
      } catch {
        setClients([]);
      }
    })();
  }, []);

  const checkin = async (id: string) => {
    await api.post(`/reservations/${id}/checkin`, {});
    await load();
  };

  const submitManual = async () => {
    setManualMsg('');
    if (!manualUser || !manualTable) {
      setManualMsg('Выберите клиента и укажите UUID стола');
      return;
    }
    try {
      await api.post('/admin/reservations', {
        user_id: manualUser,
        table_id: manualTable,
        start_time: new Date(manualStart).toISOString(),
        guest_count: manualGuests,
        comment: manualComment,
      });
      setManualMsg('Бронь создана');
      await load();
    } catch (ex: unknown) {
      const m = ex as { response?: { data?: { error?: string } } };
      setManualMsg(m.response?.data?.error || 'Ошибка');
    }
  };

  return (
    <div className="page-stack">
      <div className="card hero-card">
        <h1>Панель администратора</h1>
        <p className="muted">Все брони ресторана, чек-ин гостей и ручное создание брони от имени клиента.</p>
        <div className="btn-row">
          <Link to="/hall?edit=1" className="btn secondary">
            Редактор схемы и блокировка столов
          </Link>
        </div>
      </div>

      <div className="card">
        <h2>Ручная бронь</h2>
        <p className="hint">Выберите клиента из базы. UUID стола можно скопировать из редактора зала (данные в БД) или взять из списка броней.</p>
        <div className="grid2">
          <div>
            <label>Клиент</label>
            <select value={manualUser} onChange={(e) => setManualUser(e.target.value)}>
              <option value="">— выберите —</option>
              {clients.map((c) => (
                <option key={c.id} value={c.id}>
                  {c.full_name} ({c.email})
                </option>
              ))}
            </select>
          </div>
          <div>
            <label>UUID стола (table_id)</label>
            <input
              placeholder="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
              value={manualTable}
              onChange={(e) => setManualTable(e.target.value)}
            />
          </div>
        </div>
        <div className="grid2">
          <div>
            <label>Начало визита</label>
            <input type="datetime-local" value={manualStart} onChange={(e) => setManualStart(e.target.value)} />
          </div>
          <div>
            <label>Гости</label>
            <input
              type="number"
              min={1}
              max={20}
              value={manualGuests}
              onChange={(e) => setManualGuests(Number(e.target.value))}
            />
          </div>
        </div>
        <label>Комментарий</label>
        <textarea rows={2} value={manualComment} onChange={(e) => setManualComment(e.target.value)} />
        <button type="button" className="btn" onClick={() => void submitManual()}>
          Создать бронь
        </button>
        {manualMsg && <p className="form-msg">{manualMsg}</p>}
      </div>

      <div className="card">
        <h2>Бронирования</h2>
        <div className="table-wrap">
          <table className="data-table">
            <thead>
              <tr>
                <th>Клиент</th>
                <th>Телефон</th>
                <th>Стол</th>
                <th>Время</th>
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
                  <td>{format(parseISO(r.start_time), 'd MMM HH:mm', { locale: ru })}</td>
                  <td>
                    <span className="status-pill">{r.status}</span>
                  </td>
                  <td>
                    {r.status === 'confirmed' && (
                      <button type="button" className="secondary btn-sm" onClick={() => void checkin(r.id)}>
                        Гость прибыл
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
        {rows.length === 0 && <p className="muted">Пока нет броней</p>}
      </div>
    </div>
  );
}
