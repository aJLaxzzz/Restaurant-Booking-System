import { Navigate, Route, Routes, Link, useLocation } from 'react-router-dom';
import { useAuth } from './auth';
import Home from './pages/Home';
import Login from './pages/Login';
import Register from './pages/Register';
import HallPage from './pages/HallPage';
import MyReservations from './pages/MyReservations';
import PayPage from './pages/PayPage';
import AdminPage from './pages/AdminPage';
import OwnerPage from './pages/OwnerPage';
import WaiterPage from './pages/WaiterPage';

function Private({ children }: { children: React.ReactNode }) {
  const { user, loading } = useAuth();
  if (loading) return <div className="loading-screen">Загрузка…</div>;
  if (!user) return <Navigate to="/login" replace />;
  return <>{children}</>;
}

function RoleGate({ allow, children }: { allow: string[]; children: React.ReactNode }) {
  const { user } = useAuth();
  if (!user) return <Navigate to="/login" replace />;
  if (!allow.includes(user.role)) return <Navigate to="/" replace />;
  return <>{children}</>;
}

function NavBar() {
  const { user, logout } = useAuth();
  const loc = useLocation();

  const isActive = (path: string) => loc.pathname === path || (path !== '/' && loc.pathname.startsWith(path));

  return (
    <header className="site-header">
      <div className="nav-inner">
        <Link to="/" className="nav-brand">
          Bella Vista
        </Link>
        <nav className="nav-links">
          <Link to="/" className={isActive('/') && loc.pathname === '/' ? 'active' : ''}>
            Главная
          </Link>

          {(user?.role === 'client' || !user || user.role === 'owner') && (
            <Link to="/hall" className={isActive('/hall') ? 'active' : ''}>
              Бронирование
            </Link>
          )}

          {user?.role === 'admin' && (
            <>
              <Link to="/admin" className={isActive('/admin') ? 'active' : ''}>
                Админ
              </Link>
              <Link to="/hall?edit=1" className={loc.search.includes('edit=1') ? 'active' : ''}>
                Схема зала
              </Link>
            </>
          )}

          {user?.role === 'owner' && (
            <>
              <Link to="/owner" className={isActive('/owner') ? 'active' : ''}>
                Владелец
              </Link>
              <Link to="/admin" className={isActive('/admin') ? 'active' : ''}>
                Брони
              </Link>
              <Link to="/hall?edit=1" className={loc.search.includes('edit=1') ? 'active' : ''}>
                Схема зала
              </Link>
            </>
          )}

          {(user?.role === 'client' || user?.role === 'owner') && (
            <Link to="/me" className={isActive('/me') ? 'active' : ''}>
              Мои брони
            </Link>
          )}

          {(user?.role === 'waiter' || (user?.role === 'admin' || user?.role === 'owner')) && (
            <Link to="/waiter" className={isActive('/waiter') ? 'active' : ''}>
              Официант
            </Link>
          )}
        </nav>
        <div className="nav-user">
          {user ? (
            <>
              <span className="role-badge" title={user.email}>
                {roleLabel(user.role)}
              </span>
              <button type="button" className="secondary btn-sm" onClick={() => void logout()}>
                Выйти
              </button>
            </>
          ) : (
            <>
              <Link to="/login" className="btn btn-sm">
                Вход
              </Link>
              <Link to="/register" className="btn secondary btn-sm">
                Регистрация
              </Link>
            </>
          )}
        </div>
      </div>
    </header>
  );
}

function roleLabel(role: string) {
  const m: Record<string, string> = {
    client: 'Гость',
    admin: 'Админ',
    owner: 'Владелец',
    waiter: 'Официант',
  };
  return m[role] || role;
}

export default function App() {
  return (
    <div className="app-shell">
      <NavBar />
      <main className="main-content">
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/login" element={<Login />} />
          <Route path="/register" element={<Register />} />
          <Route path="/hall" element={<HallPage />} />
          <Route
            path="/me"
            element={
              <Private>
                <RoleGate allow={['client', 'owner']}>
                  <MyReservations />
                </RoleGate>
              </Private>
            }
          />
          <Route
            path="/pay/:pid"
            element={
              <Private>
                <PayPage />
              </Private>
            }
          />
          <Route
            path="/admin"
            element={
              <Private>
                <RoleGate allow={['admin', 'owner']}>
                  <AdminPage />
                </RoleGate>
              </Private>
            }
          />
          <Route
            path="/owner"
            element={
              <Private>
                <RoleGate allow={['owner']}>
                  <OwnerPage />
                </RoleGate>
              </Private>
            }
          />
          <Route
            path="/waiter"
            element={
              <Private>
                <RoleGate allow={['waiter', 'admin', 'owner']}>
                  <WaiterPage />
                </RoleGate>
              </Private>
            }
          />
          <Route path="*" element={<Navigate to="/" replace />} />
        </Routes>
      </main>
    </div>
  );
}
