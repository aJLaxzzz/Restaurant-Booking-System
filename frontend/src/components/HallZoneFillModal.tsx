import { useEffect, useState } from 'react';

export const ZONE_FILL_PRESETS = [
  { label: 'Мята', fill: 'rgba(52,211,153,0.22)', stroke: 'rgba(16,185,129,0.7)' },
  { label: 'Лазурь', fill: 'rgba(56,189,248,0.22)', stroke: 'rgba(14,165,233,0.7)' },
  { label: 'Янтарь', fill: 'rgba(251,191,36,0.22)', stroke: 'rgba(245,158,11,0.75)' },
  { label: 'Роза', fill: 'rgba(244,114,182,0.22)', stroke: 'rgba(236,72,153,0.7)' },
  { label: 'Фиолет', fill: 'rgba(167,139,250,0.22)', stroke: 'rgba(139,92,246,0.7)' },
  { label: 'Коралл', fill: 'rgba(248,113,113,0.22)', stroke: 'rgba(239,68,68,0.65)' },
  { label: 'Лайм', fill: 'rgba(163,230,53,0.22)', stroke: 'rgba(101,163,13,0.75)' },
  { label: 'Бирюза', fill: 'rgba(45,212,191,0.22)', stroke: 'rgba(13,148,136,0.75)' },
  { label: 'Небо', fill: 'rgba(125,211,252,0.22)', stroke: 'rgba(2,132,199,0.75)' },
  { label: 'Индиго', fill: 'rgba(129,140,248,0.22)', stroke: 'rgba(67,56,202,0.75)' },
  { label: 'Персик', fill: 'rgba(253,186,116,0.25)', stroke: 'rgba(234,88,12,0.75)' },
  { label: 'Слива', fill: 'rgba(216,180,254,0.22)', stroke: 'rgba(126,34,206,0.7)' },
  { label: 'Графит', fill: 'rgba(148,163,184,0.2)', stroke: 'rgba(71,85,105,0.85)' },
  { label: 'Оливка', fill: 'rgba(190,242,100,0.2)', stroke: 'rgba(77,124,15,0.75)' },
];

type Props = {
  open: boolean;
  onClose: () => void;
  onConfirm: (label: string, fill: string, stroke: string) => void;
};

export function HallZoneFillModal({ open, onClose, onConfirm }: Props) {
  const [text, setText] = useState('Зона');
  const [idx, setIdx] = useState(0);

  useEffect(() => {
    if (open) {
      setText('Зона');
      setIdx(0);
    }
  }, [open]);

  if (!open) return null;

  const preset = ZONE_FILL_PRESETS[idx] ?? ZONE_FILL_PRESETS[0];

  return (
    <div className="modal-backdrop" role="dialog" aria-modal="true" aria-labelledby="zone-fill-title">
      <div className="modal-card hall-add-table-modal">
        <h3 id="zone-fill-title">Зона в контуре</h3>
        <p className="muted compact">Название и цвет заливки для выбранного замкнутого контура.</p>
        <div className="field-block">
          <label>Название</label>
          <input type="text" value={text} onChange={(e) => setText(e.target.value)} maxLength={48} />
        </div>
        <div className="field-block">
          <label>Цвет</label>
          <div className="hall-zone-color-row">
            {ZONE_FILL_PRESETS.map((p, i) => (
              <button
                key={p.label}
                type="button"
                className={`hall-zone-swatch${i === idx ? ' hall-zone-swatch--active' : ''}`}
                style={{ background: p.fill, borderColor: p.stroke }}
                title={p.label}
                onClick={() => setIdx(i)}
              />
            ))}
          </div>
        </div>
        <div className="btn-row" style={{ marginTop: '1rem' }}>
          <button type="button" className="secondary" onClick={onClose}>
            Отмена
          </button>
          <button
            type="button"
            className="btn"
            onClick={() => {
              const t = text.trim() || 'Зона';
              onConfirm(t, preset.fill, preset.stroke);
              onClose();
            }}
          >
            Добавить зону
          </button>
        </div>
      </div>
    </div>
  );
}
