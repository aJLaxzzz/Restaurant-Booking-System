/**
 * Нагрузка на API (k6). Метрики → InfluxDB → Grafana.
 * Переменные: BASE_URL (по умолчанию http://127.0.0.1:8080), TOKEN (опционально — Bearer для защищённых ручек).
 */
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';

const errorRate = new Rate('errors');
const healthDuration = new Trend('health_duration');

export const options = {
  stages: [
    { duration: '30s', target: 10 },
    { duration: '1m', target: 30 },
    { duration: '30s', target: 0 },
  ],
  thresholds: {
    http_req_failed: ['rate<0.05'],
    http_req_duration: ['p(95)<800'],
    errors: ['rate<0.05'],
  },
};

const BASE = __ENV.BASE_URL || 'http://127.0.0.1:8080';
const TOKEN = __ENV.TOKEN || '';

function authHeaders() {
  const h = { 'Content-Type': 'application/json' };
  if (TOKEN) h.Authorization = `Bearer ${TOKEN}`;
  return h;
}

export default function () {
  let res = http.get(`${BASE}/health`, { tags: { name: 'health' } });
  check(res, { 'health 200': (r) => r.status === 200 });
  errorRate.add(res.status !== 200);
  healthDuration.add(res.timings.duration);

  res = http.get(`${BASE}/api/booking-defaults`, { headers: authHeaders(), tags: { name: 'booking-defaults' } });
  check(res, {
    'booking-defaults 2xx': (r) => r.status >= 200 && r.status < 300,
  });
  errorRate.add(res.status < 200 || res.status >= 300);

  sleep(0.3 + Math.random() * 0.4);
}

export function handleSummary(data) {
  return {
    stdout: JSON.stringify(data.metrics, null, 2),
  };
}
