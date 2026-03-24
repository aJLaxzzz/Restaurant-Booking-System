import { useEffect, useState } from 'react';
import { api } from '../api';
import { format, parseISO } from 'date-fns';
import { ru } from 'date-fns/locale';

type Row = {
  reservation_id: string;
  table_number: number;
  start_time: string;
  guest_count: number;
  status: string;
  client_name: string;
  phone?: string;
};

export default function WaiterPage() {
  const [rows, setRows] = useState<Row[]>([]);
  const [noteFor, setNoteFor] = useState<string | null>(null);
  const [noteText, setNoteText] = useState('');
  const [msg, setMsg] = useState('');

  const load = async () => {
    const { data } = await api.get<Row[]>('/waiter/my-tables');
    setRows(data);
  };

  useEffect(() => {
    void load();
  }, []);

  const start = async (id: string) => {
    await api.post(`/reservations/${id}/start-service`, {});
    await load();
  };

  const done = async (id: string) => {
    await api.post(`/reservations/${id}/complete`, {});
    await load();
  };

  const sendNote = async () => {
    if (!noteFor || !noteText.trim()) return;
    setMsg('');
    try {
      await api.post('/waiter/notes', { reservation_id: noteFor, note: noteText.trim() });
      setNoteText('');
      setNoteFor(null);
      setMsg('Заметка сохранена');
    } catch {
      setMsg('Не удалось сохранить');
    }
  };

  return (
    <div className="page-stack">
      <div className="card hero-card">
        <h1>Столы официанта</h1>
        <p className="muted">Назначенные брони: посадка, обслуживание, завершение и заметки для кухни.</p>
      </div>

      <div className="card">
        {rows.map((r) => (
          <div key={r.reservation_id} className="waiter-row">
            <div className="waiter-row-head">
              <strong>Стол №{r.table_number}</strong>
              <span className="status-pill">{r.status}</span>
            </div>
            <p className="waiter-meta">
              {r.client_name}
              {r.phone && <> · {r.phone}</>} · {r.guest_count}{' '}
              {r.guest_count === 1 ? 'гость' : 'гостей'} ·{' '}
              {format(parseISO(r.start_time), 'd MMMM HH:mm', { locale: ru })}
            </p>
            <div className="btn-row">
              {r.status === 'seated' && (
                <button type="button" className="btn" onClick={() => void start(r.reservation_id)}>
                  Начать обслуживание
                </button>
              )}
              {r.status === 'in_service' && (
                <button type="button" className="btn" onClick={() => void done(r.reservation_id)}>
                  Стол освобождён
                </button>
              )}
              <button type="button" className="secondary btn-sm" onClick={() => setNoteFor(noteFor === r.reservation_id ? null : r.reservation_id)}>
                Заметка
              </button>
            </div>
            {noteFor === r.reservation_id && (
              <div className="note-box">
                <textarea
                  rows={2}
                  placeholder="Аллергии, торт, задержка…"
                  value={noteText}
                  onChange={(e) => setNoteText(e.target.value)}
                />
                <button type="button" className="btn btn-sm" onClick={() => void sendNote()}>
                  Сохранить заметку
                </button>
              </div>
            )}
          </div>
        ))}
        {rows.length === 0 && <p className="muted">Нет назначенных столов на сейчас</p>}
        {msg && <p className="form-msg success">{msg}</p>}
      </div>
    </div>
  );
}
