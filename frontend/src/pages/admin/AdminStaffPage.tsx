import { useState } from 'react';
import { api } from '../../api';
import { AdminWaitersPanel } from '../../components/admin/AdminWaitersPanel';

export default function AdminStaffPage() {
  const [admStaffEmail, setAdmStaffEmail] = useState('');
  const [admStaffMsg, setAdmStaffMsg] = useState('');

  const assignAdminWaiter = async () => {
    setAdmStaffMsg('');
    if (!admStaffEmail.trim()) {
      setAdmStaffMsg('Введите email');
      return;
    }
    try {
      await api.post('/admin/staff/assign', { email: admStaffEmail.trim(), role: 'waiter' });
      setAdmStaffEmail('');
      setAdmStaffMsg('Официант назначен');
    } catch (ex: unknown) {
      const m = ex as { response?: { data?: { error?: string } } };
      setAdmStaffMsg(m.response?.data?.error || 'Ошибка');
    }
  };

  const revokeAdminWaiter = async () => {
    setAdmStaffMsg('');
    if (!admStaffEmail.trim()) {
      setAdmStaffMsg('Введите email');
      return;
    }
    try {
      await api.post('/admin/staff/assign', { email: admStaffEmail.trim(), role: 'client' });
      setAdmStaffEmail('');
      setAdmStaffMsg('Доступ снят');
    } catch (ex: unknown) {
      const m = ex as { response?: { data?: { error?: string } } };
      setAdmStaffMsg(m.response?.data?.error || 'Ошибка');
    }
  };

  return (
    <div className="page-stack">
      <div className="card">
        <h2>Официанты</h2>
        <AdminWaitersPanel />
      </div>

      <div className="card">
        <h3>Назначение по email</h3>
        <p className="muted compact">
          Назначьте зарегистрированного пользователя официантом по email или снимите доступ (роль «гость»).
        </p>
        <label>Email сотрудника</label>
        <div className="btn-row" style={{ flexWrap: 'wrap', gap: 8 }}>
          <input
            type="email"
            placeholder="user@example.com"
            value={admStaffEmail}
            onChange={(e) => setAdmStaffEmail(e.target.value)}
            style={{ flex: 1, minWidth: 200 }}
          />
          <button type="button" className="btn btn-sm" onClick={() => void assignAdminWaiter()}>
            Назначить официантом
          </button>
          <button type="button" className="secondary btn-sm" onClick={() => void revokeAdminWaiter()}>
            Снять доступ
          </button>
        </div>
        {admStaffMsg && <p className="form-msg">{admStaffMsg}</p>}
      </div>
    </div>
  );
}
