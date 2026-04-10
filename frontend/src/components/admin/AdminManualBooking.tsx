import { useEffect, useState } from 'react';
import { format } from 'date-fns';
import { api } from '../../api';
import { useAuth } from '../../auth';
import { normalizeRuPhoneInput, isValidRuPhoneE164 } from '../../utils/phone';

type HallOpt = { id: string; name: string; restaurant: string; restaurant_id: string };

type Props = {
  onCreated?: () => void | Promise<void>;
};

export function AdminManualBooking({ onCreated }: Props) {
  const { user } = useAuth();
  const [halls, setHalls] = useState<HallOpt[]>([]);
  const [restaurantId, setRestaurantId] = useState<string | null>(null);

  const [phoneInput, setPhoneInput] = useState('');
  const [manualUserId, setManualUserId] = useState('');
  const [manualUserLabel, setManualUserLabel] = useState('');
  const [manualHall, setManualHall] = useState('');
  const [manualStart, setManualStart] = useState(format(new Date(), "yyyy-MM-dd'T'HH:mm"));
  const [manualGuests, setManualGuests] = useState(2);
  const [manualComment, setManualComment] = useState('');
  const [manualMsg, setManualMsg] = useState('');
  const [lookupMsg, setLookupMsg] = useState('');

  useEffect(() => {
    void (async () => {
      try {
        const { data: me } = await api.get<{ restaurant_id?: string }>('/auth/me');
        const rid = me.restaurant_id;
        setRestaurantId(rid || null);
        if (!rid) {
          setHalls([]);
          return;
        }
        const { data: hl } = await api.get<HallOpt[]>('/halls', { params: { restaurant_id: rid } });
        const list = Array.isArray(hl) ? hl : [];
        setHalls(list);
        if (list.length && !manualHall) {
          setManualHall(list[0].id);
        }
      } catch {
        setHalls([]);
      }
    })();
  }, []);

  const lookupClient = async () => {
    setLookupMsg('');
    setManualUserId('');
    setManualUserLabel('');
    const q = normalizeRuPhoneInput(phoneInput);
    if (!q) {
      setLookupMsg('Введите телефон');
      return;
    }
    if (!isValidRuPhoneE164(q)) {
      setLookupMsg('Формат: +7 и 10 цифр');
      return;
    }
    try {
      const { data } = await api.get<{ id: string; full_name: string }>('/admin/users/lookup', {
        params: { phone: q },
      });
      setManualUserId(data.id);
      setManualUserLabel(data.full_name);
    } catch (ex: unknown) {
      const m = ex as { response?: { data?: { error?: string } } };
      setLookupMsg(m.response?.data?.error || 'Не найдено');
    }
  };

  const submitManual = async () => {
    setManualMsg('');
    if (!manualUserId || !manualHall) {
      setManualMsg('Найдите клиента по телефону и выберите зал');
      return;
    }
    try {
      await api.post('/admin/reservations', {
        user_id: manualUserId,
        hall_id: manualHall,
        start_time: new Date(manualStart).toISOString(),
        guest_count: manualGuests,
        comment: manualComment,
      });
      setManualMsg('Бронь создана (стол выбран автоматически в зале)');
      await onCreated?.();
    } catch (ex: unknown) {
      const m = ex as { response?: { data?: { error?: string } } };
      setManualMsg(m.response?.data?.error || 'Ошибка');
    }
  };

  return (
    <>
      <p className="hint">
        Введите телефон клиента в формате +7… — после поиска выберите зал; стол подберётся автоматически среди
        свободных.
      </p>
      <div className="grid2">
        <div>
          <label>Телефон клиента</label>
          <div className="btn-row" style={{ flexWrap: 'wrap', gap: 8 }}>
            <input
              inputMode="tel"
              placeholder="+79161234567"
              value={phoneInput}
              onChange={(e) => setPhoneInput(normalizeRuPhoneInput(e.target.value))}
              style={{ flex: 1, minWidth: 180 }}
            />
            <button type="button" className="secondary" onClick={() => void lookupClient()}>
              Найти
            </button>
          </div>
          {lookupMsg && <p className="form-msg">{lookupMsg}</p>}
          {manualUserLabel && (
            <p className="muted compact">
              Клиент: <strong>{manualUserLabel}</strong>
            </p>
          )}
        </div>
        <div>
          <label>Зал</label>
          <select value={manualHall} onChange={(e) => setManualHall(e.target.value)}>
            <option value="">— выберите зал —</option>
            {halls.map((h) => (
              <option key={h.id} value={h.id}>
                {h.name}
              </option>
            ))}
          </select>
          {!restaurantId && user && (
            <p className="hint">Нет привязки к ресторану в профиле — обратитесь к владельцу.</p>
          )}
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
    </>
  );
}
