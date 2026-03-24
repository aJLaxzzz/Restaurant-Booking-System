import { Link } from 'react-router-dom';
import { useAuth } from '../auth';

export default function Home() {
  const { user } = useAuth();
  return (
    <div className="page-stack">
      <section className="hero card hero-card">
        <p className="eyebrow">Онлайн-бронирование</p>
        <h1>Bella Vista</h1>
        <p className="lead">
          Выберите дату, время и число гостей — затем свободный стол на интерактивной схеме зала. Депозит и подтверждение в
          пару кликов.
        </p>
        <div className="btn-row">
          <Link to="/hall" className="btn">
            Забронировать стол
          </Link>
          {!user && (
            <Link to="/login" className="btn secondary">
              Войти
            </Link>
          )}
          {user?.role === 'client' || user?.role === 'owner' ? (
            <Link to="/me" className="btn secondary">
              Мои брони
            </Link>
          ) : null}
        </div>
      </section>

      <div className="grid-features">
        <div className="card feature">
          <h3>Гость</h3>
          <p>Визард: дата → стол → оплата. Роль «Мои брони» только для клиентов и владельца.</p>
        </div>
        <div className="card feature">
          <h3>Админ</h3>
          <p>Список броней, чек-ин, ручная бронь, редактор схемы и блокировка столов — без «Мои брони».</p>
        </div>
        <div className="card feature">
          <h3>Официант</h3>
          <p>Назначенные столы, заметки к брони, смена статусов обслуживания.</p>
        </div>
        <div className="card feature">
          <h3>Владелец</h3>
          <p>Аналитика, график загрузки, настройки, экспорт XLSX, доступ к панели броней.</p>
        </div>
      </div>

      <div className="card demo-card">
        <h3>Демо-аккаунты</h3>
        <p className="muted">
          Пароль для всех: <code>Password1</code>
        </p>
        <ul className="demo-list">
          <li>
            <code>client@demo.ru</code> — гость
          </li>
          <li>
            <code>admin@demo.ru</code> — администратор
          </li>
          <li>
            <code>owner@demo.ru</code> — владелец
          </li>
          <li>
            <code>waiter@demo.ru</code> — официант
          </li>
        </ul>
      </div>
    </div>
  );
}
