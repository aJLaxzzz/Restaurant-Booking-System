import { useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useAuth } from '../auth';
import { api } from '../api';

function pathAfterLogin(role: string): string {
  switch (role) {
    case 'admin':
      return '/admin';
    case 'waiter':
      return '/waiter';
    case 'owner':
      return '/owner';
    default:
      return '/';
  }
}

export default function Login() {
  const { login } = useAuth();
  const nav = useNavigate();
  const [email, setEmail] = useState('client@demo.ru');
  const [password, setPassword] = useState('Password1');
  const [err, setErr] = useState('');

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setErr('');
    try {
      await login(email, password);
      const { data } = await api.get<{ role: string }>('/auth/me');
      nav(pathAfterLogin(data.role));
    } catch {
      setErr('Неверный email или пароль');
    }
  };

  return (
    <div className="card" style={{ maxWidth: 400 }}>
      <h2>Вход</h2>
      <form onSubmit={(e) => void submit(e)}>
        <label>Email</label>
        <input value={email} onChange={(e) => setEmail(e.target.value)} autoComplete="email" />
        <label>Пароль</label>
        <input
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          autoComplete="current-password"
        />
        {err && <p style={{ color: 'coral' }}>{err}</p>}
        <button type="submit">Войти</button>
      </form>
      <p>
        <Link to="/register">Регистрация</Link>
      </p>
    </div>
  );
}
