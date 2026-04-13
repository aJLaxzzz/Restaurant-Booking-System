/**
 * Нагрузка на API (k6). Метрики → InfluxDB → Grafana.
 * Переменные: BASE_URL (по умолчанию http://127.0.0.1:8080), TOKEN (опционально — Bearer).
 *
 * Локальные отчёты (HTML + JSON): задайте K6_REPORT_DIR — каталог для записи (в Docker обычно /out).
 * Скрипт: scripts/run-k6-loadtest-with-report.sh
 */
import http from 'k6/http';
import { check, sleep } from 'k6';
import { Rate, Trend } from 'k6/metrics';
import { htmlReport } from './vendor/k6-reporter-bundle.js';
import { textSummary } from 'https://jslib.k6.io/k6-summary/0.0.1/index.js';

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

function buildAggregates(data) {
  const metrics = data.metrics || {};
  const slim = {};
  for (const name of Object.keys(metrics).sort()) {
    const m = metrics[name];
    if (!m || !m.values) continue;
    slim[name] = {
      type: m.type,
      values: m.values,
      contains: m.contains,
    };
    if (m.thresholds) {
      slim[name].thresholds = {};
      for (const th of Object.keys(m.thresholds)) {
        slim[name].thresholds[th] = { ok: m.thresholds[th].ok };
      }
    }
  }
  return {
    generated_at: new Date().toISOString(),
    base_url: BASE,
    state: data.state,
    metrics: slim,
  };
}

export function handleSummary(data) {
  const reportDir = __ENV.K6_REPORT_DIR;
  const out = {
    stdout: textSummary(data, { indent: ' ', enableColors: true }),
  };
  if (reportDir && reportDir.length > 0) {
    const dir = reportDir.replace(/\/+$/, '');
    out[`${dir}/report.html`] = htmlReport(data, {
      title: 'Restobook — отчёт k6 (нагрузка)',
      theme: 'default',
    });
    out[`${dir}/aggregates.json`] = JSON.stringify(buildAggregates(data), null, 2);
  }
  return out;
}
