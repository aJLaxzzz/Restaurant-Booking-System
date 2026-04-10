import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { api } from '../api';

export default function PayPage() {
  const { pid } = useParams<{ pid: string }>();
  const [info, setInfo] = useState<{
    amount_kopecks: number;
    status: string;
    purpose?: string;
    reservation_id?: string;
  } | null>(null);
  const [msg, setMsg] = useState('');
  const [tipRub, setTipRub] = useState('');

  useEffect(() => {
    if (!pid) return;
    void (async () => {
      try {
        const { data } = await api.get<{
          amount_kopecks: number;
          status: string;
          purpose?: string;
          reservation_id?: string;
        }>(`/payments/${pid}`);
        setInfo(data);
      } catch {
        setMsg('Платёж не найден');
      }
    })();
  }, [pid]);

  const pay = async () => {
    if (!pid) return;
    setMsg('');
    try {
      await api.post(`/payments/checkout/${pid}/simulate`);
      const isTab = info?.purpose === 'tab';
      setMsg(
        isTab
          ? 'Оплата счёта прошла успешно (демо).'
          : 'Оплата прошла успешно (демо). Бронь подтверждена.',
      );
      const { data } = await api.get<typeof info>(`/payments/${pid}`);
      setInfo(data);
    } catch {
      setMsg('Ошибка оплаты');
    }
  };

  const payTip = async () => {
    const rid = info?.reservation_id;
    if (!rid) return;
    const rub = parseFloat(tipRub.replace(',', '.'));
    if (!Number.isFinite(rub) || rub < 1) {
      setMsg('Введите сумму чаевых от 1 ₽');
      return;
    }
    const kopecks = Math.round(rub * 100);
    setMsg('');
    try {
      const { data } = await api.post<{ payment_id: string }>(`/reservations/${rid}/order/tip-checkout`, {
        tip_kopecks: kopecks,
      });
      window.location.href = `/pay/${data.payment_id}`;
    } catch (ex: unknown) {
      const m = ex as { response?: { data?: { error?: string } } };
      setMsg(m.response?.data?.error || 'Не удалось создать платёж чаевых');
    }
  };

  const isTab = info?.purpose === 'tab';
  const showTipBlock = info?.status === 'succeeded' && isTab && info.reservation_id;

  return (
    <div className="card pay-card">
      <h2>{isTab ? 'Оплата счёта' : info?.purpose === 'tip' ? 'Чаевые' : 'Оплата депозита'}</h2>
      {info && (
        <>
          <p>
            Сумма: <strong>{(info.amount_kopecks / 100).toFixed(0)} ₽</strong>
          </p>
          <p>Статус: {info.status}</p>
          {info.status === 'pending' && (
            <button type="button" className="btn" onClick={() => void pay()}>
              Оплатить (имитация)
            </button>
          )}
        </>
      )}
      {showTipBlock && (
        <div style={{ marginTop: '1.25rem', paddingTop: '1rem', borderTop: '1px solid var(--border)' }}>
          <h3 style={{ fontSize: '1.05rem', margin: '0 0 0.5rem' }}>Чаевые официанту</h3>
          <p className="muted compact">Необязательно — отдельный платёж после закрытия счёта.</p>
          <label>Сумма, ₽</label>
          <input
            inputMode="decimal"
            value={tipRub}
            onChange={(e) => setTipRub(e.target.value)}
            placeholder="например 300"
          />
          <button type="button" className="btn btn-sm" style={{ marginTop: 8 }} onClick={() => void payTip()}>
            Перейти к оплате чаевых
          </button>
        </div>
      )}
      {msg && <p className="form-msg">{msg}</p>}
    </div>
  );
}
