import { useEffect, useState } from 'react';

export type AddTableForm = {
  capacity: number;
  shape: string;
  width: number;
  height: number;
  tableNumber: number | null;
};

type Props = {
  open: boolean;
  onClose: () => void;
  onSubmit: (f: AddTableForm) => void | Promise<void>;
};

const DEFAULT_TABLE_W = 88;
const DEFAULT_TABLE_H = 64;

export function HallAddTableModal({ open, onClose, onSubmit }: Props) {
  const [capacity, setCapacity] = useState(4);
  const [shape, setShape] = useState('rect');

  useEffect(() => {
    if (open) {
      setCapacity(4);
      setShape('rect');
    }
  }, [open]);

  if (!open) return null;

  const submit = async () => {
    const w = shape === 'circle' ? Math.max(DEFAULT_TABLE_W, DEFAULT_TABLE_H) : DEFAULT_TABLE_W;
    const h = shape === 'circle' ? Math.max(DEFAULT_TABLE_W, DEFAULT_TABLE_H) : DEFAULT_TABLE_H;
    try {
      await onSubmit({
        capacity: Math.max(1, capacity),
        shape,
        width: Math.max(20, w),
        height: Math.max(20, h),
        tableNumber: null,
      });
      onClose();
    } catch {
      /* окно остаётся открытым */
    }
  };

  return (
    <div className="modal-backdrop" role="dialog" aria-modal="true" aria-labelledby="add-table-title">
      <div className="modal-card hall-add-table-modal">
        <h3 id="add-table-title">Добавить стол</h3>
        <p className="muted compact">Номер стола назначит сервер автоматически.</p>
        <div className="field-block">
          <label>Вместимость (гостей)</label>
          <input
            type="number"
            min={1}
            max={32}
            value={capacity}
            onChange={(e) => setCapacity(parseInt(e.target.value, 10) || 1)}
          />
        </div>
        <div className="field-block">
          <label>Форма</label>
          <select value={shape} onChange={(e) => setShape(e.target.value)}>
            <option value="rect">Прямоугольник</option>
            <option value="square">Квадрат</option>
            <option value="ellipse">Эллипс</option>
            <option value="circle">Круг</option>
          </select>
        </div>
        <p className="muted compact" style={{ marginTop: 0 }}>
          Размер на схеме можно изменить рамкой после выбора стола в режиме редактирования.
        </p>
        <div className="btn-row" style={{ marginTop: '1rem' }}>
          <button type="button" className="secondary" onClick={onClose}>
            Отмена
          </button>
          <button type="button" className="btn" onClick={() => void submit()}>
            Добавить
          </button>
        </div>
      </div>
    </div>
  );
}
