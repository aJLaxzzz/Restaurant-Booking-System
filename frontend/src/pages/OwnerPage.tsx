import { useEffect, useState } from 'react';
import { Link } from 'react-router-dom';
import { api } from '../api';
import {
  Chart as ChartJS,
  CategoryScale,
  LinearScale,
  PointElement,
  LineElement,
  Title,
  Tooltip,
  Legend,
  Filler,
} from 'chart.js';
import { Line } from 'react-chartjs-2';
ChartJS.register(CategoryScale, LinearScale, PointElement, LineElement, Title, Tooltip, Legend, Filler);

export default function OwnerPage() {
  const [analytics, setAnalytics] = useState<{ labels: string[]; load_percent: number[] } | null>(null);
  const [settings, setSettings] = useState<Record<string, unknown> | null>(null);

  useEffect(() => {
    void (async () => {
      const [a, s] = await Promise.all([api.get('/owner/analytics'), api.get('/settings')]);
      setAnalytics(a.data as { labels: string[]; load_percent: number[] });
      setSettings(s.data as Record<string, unknown>);
    })();
  }, []);

  const downloadFinance = async () => {
    const res = await api.get('/owner/finance/export', { responseType: 'blob' });
    const url = URL.createObjectURL(res.data);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'finance-report.xlsx';
    a.click();
    URL.revokeObjectURL(url);
  };

  const chartData =
    analytics && analytics.labels.length
      ? {
          labels: analytics.labels,
          datasets: [
            {
              label: 'Загрузка зала, % (условно)',
              data: analytics.load_percent,
              borderColor: 'rgb(124, 58, 237)',
              backgroundColor: 'rgba(124, 58, 237, 0.12)',
              fill: true,
              tension: 0.3,
            },
          ],
        }
      : null;

  return (
    <div className="page-stack">
      <div className="card hero-card">
        <h1>Кабинет владельца</h1>
        <p className="muted">Аналитика загрузки и системные настройки. Экспорт финансов — из API (XLSX).</p>
        <div className="btn-row">
          <button type="button" className="btn secondary" onClick={() => void downloadFinance()}>
            Скачать отчёт XLSX
          </button>
          <Link to="/hall?edit=1" className="btn secondary">
            Схема зала
          </Link>
        </div>
      </div>

      <div className="card">
        <h2>Загрузка по дням (30 дней)</h2>
        {chartData ? (
          <div className="chart-wrap">
            <Line
              data={chartData}
              options={{
                responsive: true,
                maintainAspectRatio: false,
                plugins: {
                  legend: { position: 'top' as const },
                },
                scales: {
                  y: { min: 0, suggestedMax: 100 },
                },
              }}
            />
          </div>
        ) : (
          <p className="muted">Недостаточно данных для графика</p>
        )}
      </div>

      {settings && (
        <div className="card">
          <h2>Настройки</h2>
          <pre className="settings-pre">{JSON.stringify(settings, null, 2)}</pre>
        </div>
      )}
    </div>
  );
}
