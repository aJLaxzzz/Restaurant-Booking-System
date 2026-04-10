/** Нормализация к +7XXXXXXXXXX (как `phoneRe` на бэкенде). */
export function normalizeRuPhoneInput(raw: string): string {
  let s = raw.trim().replace(/\s/g, '').replace(/[()-]/g, '');
  if (!s) return '';
  if (s.startsWith('+7')) {
    return '+7' + s.slice(2).replace(/\D/g, '').slice(0, 10);
  }
  const digits = s.replace(/\D/g, '');
  if (digits.startsWith('8') && digits.length >= 11) {
    return '+7' + digits.slice(1, 11);
  }
  if (digits.length === 11 && digits[0] === '7') {
    return '+7' + digits.slice(1, 11);
  }
  if (digits.length === 10 && digits[0] === '9') {
    return '+7' + digits;
  }
  return '+7' + digits.slice(0, 10);
}

export function isValidRuPhoneE164(phone: string): boolean {
  return /^\+7\d{10}$/.test(phone.trim());
}

/** Отображение: +7 XXX XXX XX XX */
export function formatRuPhoneDisplay(e164: string): string {
  const p = normalizeRuPhoneInput(e164);
  if (!/^\+7\d{10}$/.test(p)) return p;
  const d = p.slice(2);
  return `+7 ${d.slice(0, 3)} ${d.slice(3, 6)} ${d.slice(6, 8)} ${d.slice(8, 10)}`;
}
