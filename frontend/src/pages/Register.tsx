import { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { api } from '../api';

export default function Register() {
  const nav = useNavigate();
  const [email, setEmail] = useState('');
  const [password, setPassword] = useState('');
  const [fullName, setFullName] = useState('');
  const [phone, setPhone] = useState('+7');
  const [err, setErr] = useState('');

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setErr('');
    try {
      await api.post('/auth/register', { email, password, full_name: fullName, phone });
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
        <label>Телефон (+7XXXXXXXXXX)</label>
        <input value={phone} onChange={(e) => setPhone(e.target.value)} required />
        <label>Пароль (буквы и цифры, от 8 символов)</label>
        <input type="password" value={password} onChange={(e) => setPassword(e.target.value)} required />
        {err && <p style={{ color: 'coral' }}>{err}</p>}
        <button type="submit">Создать аккаунт</button>
      </form>
      <p>
        <Link to="/login">Уже есть аккаунт</Link>
      </p>
    </div>
  );
}
