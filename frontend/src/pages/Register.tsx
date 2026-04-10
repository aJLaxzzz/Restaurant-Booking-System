import { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { api } from '../api';
import { normalizeRuPhoneInput, isValidRuPhoneE164 } from '../utils/phone';

export default function Register() {
  const nav = useNavigate();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [fullName, setFullName] = useState('');
  const [phone, setPhone] = useState('+7');
  const [err, setErr] = useState('');
  const [asOwner, setAsOwner] = useState(false);

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setErr('');
    const phoneNorm = normalizeRuPhoneInput(phone);
    if (!isValidRuPhoneE164(phoneNorm)) {
      setErr('Телефон: +7 и 10 цифр (например +79161234567)');
      return;
    }
    try {
      await api.post('/auth/register', {
        email,
        password,
        full_name: fullName,
        phone: phoneNorm,
        register_as_owner: asOwner,
      });
      nav('/login');
    } catch (ex: unknown) {
      const m = ex as { response?: { data?: { error?: string } } };
      setErr(m.response?.data?.error || 'Ошибка');
    }
  };

  return (
    <div className="card" style={{ maxWidth: 400 }}>
      <h2>Регистрация</h2>
      <form onSubmit={(e) => void submit(e)}>
        <label>Имя</label>
        <input value={fullName} onChange={(e) => setFullName(e.target.value)} required />
        <label>Email</label>
        <input type="email" value={email} onChange={(e) => setEmail(e.target.value)} required />
        <label>Телефон</label>
        <input
          inputMode="tel"
          autoComplete="tel"
          placeholder="+7 9XX XXX XX XX"
          value={phone}
          onChange={(e) => setPhone(normalizeRuPhoneInput(e.target.value))}
          required
        />
        <label>Пароль (буквы и цифры, от 8 символов)</label>
        <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required />
        <label className="checkbox-row">
          <input type="checkbox" checked={asOwner} onChange={(e) => setAsOwner(e.target.checked)} />
          Регистрация как владелец — заявка уходит на модерацию; после одобрения суперадмином можно создать ресторан
        </label>
        {err && <p style={{ color: 'coral' }}>{err}</p>}
        <button type="submit">Создать аккаунт</button>
      </form>
      <p>
        <Link to="/login">Уже есть аккаунт</Link>
      </p>
    </div>
  );
}
