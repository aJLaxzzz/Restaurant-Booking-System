import { Link, NavLink, Outlet } from 'react-router-dom';

export default function AdminLayout() {
  return (
    <div className="page-stack">
      <div className="card hero-card">
        <h1>Панель администратора</h1>
        <p className="muted">
          Брони на сегодня (по Москве), меню зала, персонал и ручное создание брони по телефону клиента.
        </p>
        <div className="btn-row">
          <Link to="/hall?edit=1" className="btn secondary">
            Редактор схемы и блокировка столов
          </Link>
        </div>
      </div>

      <nav className="admin-tabs" aria-label="Разделы админки">
        <NavLink to="/admin" end className={({ isActive }) => (isActive ? 'active' : undefined)}>
          Брони
        </NavLink>
        <NavLink to="/admin/menu" end className={({ isActive }) => (isActive ? 'active' : undefined)}>
          Меню
        </NavLink>
        <NavLink to="/admin/menu/positions" className={({ isActive }) => (isActive ? 'active' : undefined)}>
          Позиции меню
        </NavLink>
        <NavLink to="/admin/staff" className={({ isActive }) => (isActive ? 'active' : undefined)}>
          Персонал
        </NavLink>
        <NavLink to="/admin/manual" className={({ isActive }) => (isActive ? 'active' : undefined)}>
          Ручная бронь
        </NavLink>
      </nav>

      <Outlet />
    </div>
  );
}
