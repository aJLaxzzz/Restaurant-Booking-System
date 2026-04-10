import React, { createContext, useCallback, useContext, useEffect, useState } from 'react';
import { api } from './api';

export type User = {
  id: string;
  email: string;
  full_name: string;
  phone: string;
  role: string;
  status: string;
  restaurant_id?: string;
};

type AuthCtx = {
  user: User | null;
  loading: boolean;
  login: (email: string, password: string) => Promise<void>;
  logout: () => Promise<void>;
  refreshMe: () => Promise<void>;
};

const Ctx = createContext<AuthCtx | null>(null);

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [loading, setLoading] = useState(true);

  const refreshMe = useCallback(async () => {
    const t = localStorage.getItem('access_token');
    if (!t) {
      setUser(null);
      setLoading(false);
      return;
    }
    try {
      const { data } = await api.get<User>('/auth/me');
      setUser(data);
    } catch {
      setUser(null);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refreshMe();
  }, [refreshMe]);

  const login = async (email: string, password: string) => {
    const { data } = await api.post<{ access_token: string }>('/auth/login', { email, password });
    localStorage.setItem('access_token', data.access_token);
    await refreshMe();
  };

  const logout = async () => {
    await api.post('/auth/logout');
    localStorage.removeItem('access_token');
    setUser(null);
  };

  return <Ctx.Provider value={{ user, loading, login, logout, refreshMe }}>{children}</Ctx.Provider>;
}

export function useAuth() {
  const v = useContext(Ctx);
  if (!v) throw new Error('AuthProvider');
  return v;
}
