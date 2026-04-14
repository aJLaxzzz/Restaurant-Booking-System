import { useEffect, useState } from 'react';
import { useParams } from 'react-router-dom';
import { api } from '../api';
import { paymentStatusLabelRu } from '../utils/reservationStatus';

type PayInfo = {
  amount_kopecks: number;
  status: string;
  purpose?: string;
  reservation_id?: string;
  order_total_kopecks?: number;
  deposit_credited_kopecks?: number;
};

type FlashTone = 'success' | 'error' | 'info';

export default function PayPage() {
  const { pid } = useParams<{ pid: string }>();
  const [info, setInfo] = useState<PayInfo | null>(null);
  const [flash, setFlash] = useState<{ tone: FlashTone; text: string } | null>(null);
  const [tipRub, setTipRub] = useState('');
  const [tipBaseKopecks, setTipBaseKopecks] = useState<number | null>(null);
  const [ratingRestaurant, setRatingRestaurant] = useState<number>(5);
  const [ratingWaiter, setRatingWaiter] = useState<number>(5);
  const [ratingBusy, setRatingBusy] = useState(false);
  const [ratingDone, setRatingDone] = useState(false);

  useEffect(() => {
    if (!pid) return;
    void (async () => {
      try {
        const { data } = await api.get<PayInfo>(`/payments/${pid}`);
        setInfo(data);
        setFlash(null);
      } catch {
        setFlash({ tone: 'error', text: 'Платёж не найден или нет доступа.' });
      }
    })();
  }, [pid]);

  const isTab = info?.purpose === 'tab';
  const showTipBlock = info?.status === 'succeeded' && isTab && info.reservation_id;
  const showRatingBlock = info?.status === 'succeeded' && isTab && info.reservation_id && !ratingDone;

  useEffect(() => {
    if (!showTipBlock || !info?.reservation_id) {
      setTipBaseKopecks(null);
      return;
    }
    let cancelled = false;
    void api
      .get<{ total_kopecks?: number }>(`/reservations/${info.reservation_id}/order`)
      .then(({ data }) => {
        if (!cancelled) setTipBaseKopecks(typeof data.total_kopecks === 'number' ? data.total_kopecks : null);
      })
      .catch(() => {
        if (!cancelled) setTipBaseKopecks(null);
      });
    return () => {
      cancelled = true;
    };
  }, [showTipBlock, info?.reservation_id]);

  const applyTipPercent = (pct: number) => {
    if (tipBaseKopecks == null || tipBaseKopecks < 1) return;
    const kopecks = Math.round((tipBaseKopecks * pct) / 100);
    const rub = Math.max(1, Math.round(kopecks / 100));
    setTipRub(String(rub));
    setFlash(null);
  };

  const pay = async () => {
    if (!pid) return;
    setFlash(null);
    try {
      await api.post(`/payments/checkout/${pid}/simulate`);
      const purpose = info?.purpose;
      let successText = 'Оплата прошла успешно. Бронь подтверждена.';
      if (purpose === 'tab') {
        successText = 'Оплата счёта прошла успешно. Приятного аппетита!';
      } else if (purpose === 'tip') {
        successText = 'Спасибо! Чаевые успешно оплачены.';
      }
      setFlash({
        tone: 'success',
        text: successText,
      });
      const { data } = await api.get<PayInfo>(`/payments/${pid}`);
      setInfo(data);
    } catch {
      setFlash({ tone: 'error', text: 'Не удалось провести оплату. Попробуйте ещё раз.' });
    }
  };

  const payTip = async () => {
    const rid = info?.reservation_id;
    if (!rid) return;
    const rub = parseFloat(tipRub.replace(',', '.'));
    if (!Number.isFinite(rub) || rub < 1) {
      setFlash({ tone: 'info', text: 'Введите сумму чаевых не менее 1 ₽ или выберите процент от суммы заказа.' });
      return;
    }
    const kopecks = Math.round(rub * 100);
    setFlash(null);
    try {
      const { data } = await api.post<{ payment_id: string }>(`/reservations/${rid}/order/tip-checkout`, {
        tip_kopecks: kopecks,
      });
      window.location.href = `/pay/${data.payment_id}`;
    } catch (ex: unknown) {
      const m = ex as { response?: { data?: { error?: string } } };
      setFlash({ tone: 'error', text: m.response?.data?.error || 'Не удалось создать платёж чаевых.' });
    }
  };

  const submitRating = async () => {
    const rid = info?.reservation_id;
    if (!rid) return;
    if (ratingBusy) return;
    setRatingBusy(true);
    setFlash(null);
    try {
      await api.post('/reviews', {
        reservation_id: rid,
        rating_restaurant: ratingRestaurant,
        rating_waiter: ratingWaiter,
      });
      setRatingDone(true);
      setFlash({ tone: 'success', text: 'Спасибо! Оценка сохранена.' });
    } catch (ex: unknown) {
      const m = ex as { response?: { data?: { error?: string } } };
      setFlash({ tone: 'error', text: m.response?.data?.error || 'Не удалось сохранить оценку.' });
    } finally {
      setRatingBusy(false);
    }
  };

  return (
    <div className="card pay-card">
      <h2>{isTab ? 'Оплата счёта' : info?.purpose === 'tip' ? 'Чаевые' : 'Оплата депозита'}</h2>
      {flash && (
        <div className={`pay-flash pay-flash--${flash.tone}`} role="status">
          {flash.text}
        </div>
      )}
      {info && (
        <>
          {isTab &&
            info.order_total_kopecks != null &&
            info.order_total_kopecks > 0 &&
            (info.deposit_credited_kopecks ?? 0) > 0 && (
              <p className="muted compact" style={{ marginBottom: '0.75rem' }}>
                Сумма заказа: {(info.order_total_kopecks / 100).toLocaleString('ru-RU')} ₽. Зачтён депозит при брони:{' '}
                {(info.deposit_credited_kopecks! / 100).toLocaleString('ru-RU')} ₽. К доплате по счёту — разница.
              </p>
            )}
          <p>
            {isTab && (info.deposit_credited_kopecks ?? 0) > 0 ? 'К доплате' : 'Сумма'}:{' '}
            <strong>{(info.amount_kopecks / 100).toFixed(0)} ₽</strong>
          </p>
          <p>Статус: {paymentStatusLabelRu(info.status)}</p>
          {info.status === 'pending' && (
            <button type="button" className="btn" onClick={() => void pay()}>
              Оплатить
            </button>
          )}
        </>
      )}
      {showRatingBlock && (
        <div style={{ marginTop: '1.25rem', paddingTop: '1rem', borderTop: '1px solid var(--border)' }}>
          <h3 style={{ fontSize: '1.05rem', margin: '0 0 0.5rem' }}>Оцените визит</h3>
          <p className="muted compact" style={{ marginTop: 0 }}>
            Это поможет улучшить сервис и покажется в рейтингах заведения.
          </p>

          <label>Заведение</label>
          <StarRow value={ratingRestaurant} onChange={setRatingRestaurant} disabled={ratingBusy} />

          <label style={{ marginTop: 10 }}>Официант</label>
          <StarRow value={ratingWaiter} onChange={setRatingWaiter} disabled={ratingBusy} />

          <button
            type="button"
            className="btn btn-sm"
            style={{ marginTop: 12 }}
            onClick={() => void submitRating()}
            disabled={ratingBusy}
          >
            Отправить оценку
          </button>
        </div>
      )}
      {ratingDone && isTab && info?.status === 'succeeded' && (
        <div style={{ marginTop: '1.25rem', paddingTop: '1rem', borderTop: '1px solid var(--border)' }}>
          <p className="muted compact" style={{ margin: 0 }}>
            Оценка отправлена. Спасибо!
          </p>
        </div>
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
            placeholder={tipBaseKopecks != null && tipBaseKopecks > 0 ? 'своя сумма' : 'например 300'}
          />
          {tipBaseKopecks != null && tipBaseKopecks > 0 && (
            <div className="pay-tip-presets">
              {[5, 10, 15, 20].map((pct) => (
                <button
                  key={pct}
                  type="button"
                  className="secondary btn-sm"
                  onClick={() => applyTipPercent(pct)}
                  title={`${pct}% от суммы заказа`}
                >
                  {pct}%
                </button>
              ))}
            </div>
          )}
          {tipBaseKopecks === null && showTipBlock && (
            <p className="muted compact" style={{ marginTop: 6 }}>
              Проценты от суммы заказа появятся после загрузки счёта.
            </p>
          )}
          <button type="button" className="btn btn-sm" style={{ marginTop: 10 }} onClick={() => void payTip()}>
            Перейти к оплате чаевых
          </button>
        </div>
      )}
    </div>
  );
}

function StarRow({
  value,
  onChange,
  disabled,
}: {
  value: number;
  onChange: (v: number) => void;
  disabled?: boolean;
}) {
  return (
    <div className="btn-row" style={{ gap: 6, flexWrap: 'wrap' }}>
      {[1, 2, 3, 4, 5].map((n) => (
        <button
          key={n}
          type="button"
          className={n <= value ? 'btn btn-sm' : 'secondary btn-sm'}
          onClick={() => onChange(n)}
          disabled={disabled}
          aria-label={`Оценка ${n} из 5`}
          title={`${n} / 5`}
        >
          {n <= value ? '★' : '☆'}
        </button>
      ))}
      <span className="muted compact" style={{ alignSelf: 'center' }}>
        {value}/5
      </span>
    </div>
  );
}
