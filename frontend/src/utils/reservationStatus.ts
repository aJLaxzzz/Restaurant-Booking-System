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

/** Статус платежа (payments.status). */
export function paymentStatusLabelRu(code: string): string {
  const c = (code || '').trim();
  const map: Record<string, string> = {
    pending: 'Ожидает оплаты',
    succeeded: 'Оплачено',
    refunded: 'Возврат оформлен',
    failed: 'Оплата не прошла',
  };
  return map[c] ?? c;
}

/** Статус стола на схеме зала (tables.status). */
export function tableStatusLabelRu(code: string): string {
  const c = (code || '').trim();
  const map: Record<string, string> = {
    available: 'Свободен',
    occupied: 'Занят',
    selected: 'Выбран',
    blocked: 'Заблокирован',
    locked_by_other: 'Занят другим гостем',
  };
  return map[c] ?? c;
}

/** Статус учётной записи (users.status). */
export function userAccountStatusLabelRu(code: string): string {
  const c = (code || '').trim();
  const map: Record<string, string> = {
    active: 'Активен',
    blocked: 'Заблокирован',
  };
  return map[c] ?? c;
}

/** Роль пользователя для подписей в интерфейсе (значение в API без изменений). */
export function userRoleLabelRu(role: string): string {
  const r = (role || '').trim();
  const map: Record<string, string> = {
    client: 'Гость',
    owner: 'Владелец',
    admin: 'Администратор',
    waiter: 'Официант',
    superadmin: 'Администратор системы',
  };
  return map[r] ?? r;
}
