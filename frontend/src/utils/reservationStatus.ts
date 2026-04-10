/** Метки статусов брони для UI (значения из БД / API). */
export function reservationStatusLabelRu(code: string): string {
  const c = (code || '').trim();
  const map: Record<string, string> = {
    pending_payment: 'Ожидает оплаты',
    confirmed: 'Подтверждена',
    seated: 'Гость за столом',
    in_service: 'Обслуживается',
    pending_payment_tab: 'Ожидает оплаты счёта',
    cancelled_by_client: 'Отменена гостем',
    cancelled_by_admin: 'Отменена администратором',
    cancelled: 'Отменена',
    completed: 'Завершена',
    no_show: 'Не пришёл',
  };
  return map[c] ?? c;
}
