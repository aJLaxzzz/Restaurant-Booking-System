import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { api } from '../api';

export default function PayPage() {
  const { pid } = useParams<{ pid: string }>();
  const [info, setInfo] = useState<{ amount_kopecks: number; status: string } | null>(null);
  const [msg, setMsg] = useState('');

  useEffect(() => {
    if (!pid) return;
    void (async () => {
      try {
        const { data } = await api.get<{ amount_kopecks: number; status: string }>(`/payments/${pid}`);
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
      setMsg('Оплата прошла успешно (демо). Бронь подтверждена.');
      const { data } = await api.get<{ amount_kopecks: number; status: string }>(`/payments/${pid}`);
      setInfo(data);
    } catch {
      setMsg('Ошибка оплаты');
    }
  };

  return (
    <div className="card" style={{ maxWidth: 480 }}>
      <h2>Оплата депозита</h2>
      {info && (
        <>
          <p>
            Сумма: <strong>{(info.amount_kopecks / 100).toFixed(0)} ₽</strong> (копейки в ТЗ)
          </p>
          <p>Статус: {info.status}</p>
          {info.status === 'pending' && (
            <button type="button" onClick={() => void pay()}>
              Оплатить (имитация ЮKassa/Stripe)
            </button>
          )}
        </>
      )}
      {msg && <p>{msg}</p>}
    </div>
  );
}
