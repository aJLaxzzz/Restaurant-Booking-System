import { Navigate, Route, Routes, Link, useLocation } from 'react-router-dom';
import { useAuth } from './auth';
import Home from './pages/Home';
import RestaurantPage from './pages/RestaurantPage';
import Login from './pages/Login';
import Register from './pages/Register';
import HallPage from './pages/HallPage';
import MyReservations from './pages/MyReservations';
import PayPage from './pages/PayPage';
import AdminLayout from './pages/admin/AdminLayout';
import AdminBookingsPage from './pages/admin/AdminBookingsPage';
import AdminMenuPage from './pages/admin/AdminMenuPage';
import AdminStaffPage from './pages/admin/AdminStaffPage';
import AdminManualPage from './pages/admin/AdminManualPage';
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
          Restobook
        </Link>
        <nav className="nav-links">
          <Link to="/" className={isActive('/') && loc.pathname === '/' ? 'active' : ''}>
            Главная
          </Link>

          {(user?.role === 'client' || !user) && (
            <Link to="/hall" className={isActive('/hall') ? 'active' : ''}>
              Бронирование
            </Link>
          )}

          {user?.role === 'admin' && (
            <>
              <Link
                to="/admin"
                className={loc.pathname.startsWith('/admin') ? 'active' : ''}
              >
                Админ
              </Link>
              <Link to="/hall?edit=1" className={loc.search.includes('edit=1') ? 'active' : ''}>
                Схема зала
              </Link>
            </>
          )}

          {user?.role === 'owner' && (
            <Link to="/owner" className={isActive('/owner') ? 'active' : ''}>
              Владелец
            </Link>
          )}

          {user?.role === 'client' && (
            <Link to="/me" className={isActive('/me') ? 'active' : ''}>
              Мои брони
            </Link>
          )}

          {user?.role === 'waiter' && (
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

/** /me только для гостей; остальные роли — на рабочие экраны */
function MeOrRedirect() {
  const { user } = useAuth();
  if (!user) return <Navigate to="/login" replace />;
  if (user.role === 'owner') return <Navigate to="/owner" replace />;
  if (user.role === 'admin') return <Navigate to="/admin" replace />;
  if (user.role === 'waiter') return <Navigate to="/waiter" replace />;
  return <MyReservations />;
}

export default function App() {
  return (
    <div className="app-shell">
      <NavBar />
      <main className="main-content">
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/restaurant/:id" element={<RestaurantPage />} />
          <Route path="/login" element={<Login />} />
          <Route path="/register" element={<Register />} />
          <Route path="/hall" element={<HallPage />} />
          <Route
            path="/me"
            element={
              <Private>
                <MeOrRedirect />
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
                <RoleGate allow={['admin']}>
                  <AdminLayout />
                </RoleGate>
              </Private>
            }
          >
            <Route index element={<AdminBookingsPage />} />
            <Route path="menu" element={<AdminMenuPage />} />
            <Route path="staff" element={<AdminStaffPage />} />
            <Route path="manual" element={<AdminManualPage />} />
          </Route>
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
                <RoleGate allow={['waiter']}>
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
