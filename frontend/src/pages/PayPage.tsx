import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { api } from '../api';

export default function PayPage() {
  const { pid } = useParams<{ pid: string }>();
  const [info, setInfo] = useState<{ amount_kopecks: number; status: string; purpose?: string } | null>(null);
  const [msg, setMsg] = useState('');

  useEffect(() => {
    if (!pid) return;
    void (async () => {
      try {
        const { data } = await api.get<{ amount_kopecks: number; status: string; purpose?: string }>(`/payments/${pid}`);
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
          : 'Оплата прошла успешно (демо). Бронь подтверждена.'
      );
      const { data } = await api.get<{ amount_kopecks: number; status: string; purpose?: string }>(`/payments/${pid}`);
      setInfo(data);
    } catch {
      setMsg('Ошибка оплаты');
    }
  };

  const isTab = info?.purpose === 'tab';

  return (
    <div className="card pay-card">
      <h2>{isTab ? 'Оплата счёта' : 'Оплата депозита'}</h2>
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
      {msg && <p className="form-msg">{msg}</p>}
    </div>
  );
}
