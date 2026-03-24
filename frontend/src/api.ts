import axios from 'axios';

export const api = axios.create({
  baseURL: '/api',
  withCredentials: true,
});

api.interceptors.request.use((config) => {
  const t = localStorage.getItem('access_token');
  if (t) config.headers.Authorization = `Bearer ${t}`;
  return config;
});

let refreshPromise: Promise<string | null> | null = null;

api.interceptors.response.use(
  (r) => r,
  async (err) => {
    const orig = err.config;
    const o = orig as { _retry?: boolean };
    if (err.response?.status === 401 && !o._retry) {
      o._retry = true;
      if (!refreshPromise) {
        refreshPromise = axios
          .post('/api/auth/refresh', {}, { withCredentials: true })
          .then((r) => {
            const tok = r.data.access_token as string;
            localStorage.setItem('access_token', tok);
            return tok;
          })
          .catch(() => {
            localStorage.removeItem('access_token');
            return null;
          })
          .finally(() => {
            refreshPromise = null;
          });
      }
      const tok = await refreshPromise;
      if (tok) {
        orig.headers.Authorization = `Bearer ${tok}`;
        return api(orig);
      }
    }
    return Promise.reject(err);
  }
);
