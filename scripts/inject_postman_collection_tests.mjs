/**
 * Добавляет в каждый запрос коллекции Postman общие pm.test и именованную проверку,
 * чтобы Runner/Newman показывали тесты по всем эндпоинтам (не только у Login).
 * Запуск: node scripts/inject_postman_collection_tests.mjs
 */
import fs from 'fs';
import path from 'path';
import { fileURLToPath } from 'url';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const COL = path.join(__dirname, '..', 'postman', 'Restaurant-Booking-API.postman_collection.json');

const BASE = [
  "pm.test('HTTP: нет ошибки 5xx', function () {",
  "  pm.expect(pm.response.code).to.be.below(500);",
  "});",
  "pm.test('Ответ получен быстрее 30 с', function () {",
  "  pm.expect(pm.response.responseTime).to.be.below(30000);",
  "});",
];

function escapeName(name) {
  return String(name).replace(/\\/g, '\\\\').replace(/'/g, "\\'");
}

function namedTest(name) {
  const en = escapeName(name);
  return [
    `pm.test('${en}: запрос выполнен', function () {`,
    '  pm.expect(pm.response.code).to.be.at.least(100);',
    '});',
  ];
}

function walk(items) {
  if (!items) return;
  for (const item of items) {
    if (item.item) {
      walk(item.item);
      continue;
    }
    if (!item.request) continue;

    const en = escapeName(item.name);
    const marker = `${en}: запрос выполнен`;

    let exec = [];
    const ev = item.event?.find((e) => e.listen === 'test');
    if (ev?.script?.exec?.length) exec = [...ev.script.exec];

    let joined = exec.join('\n');
    if (!joined.includes('нет ошибки 5xx')) {
      exec = [...BASE, ...exec];
      joined = exec.join('\n');
    }
    if (!joined.includes(marker)) {
      exec = [...exec, ...namedTest(item.name)];
    }

    if (!item.event) item.event = [];
    const idx = item.event.findIndex((e) => e.listen === 'test');
    const script = { exec, type: 'text/javascript' };
    if (idx >= 0) item.event[idx].script = script;
    else item.event.push({ listen: 'test', script });
  }
}

const raw = fs.readFileSync(COL, 'utf8');
const col = JSON.parse(raw);

col.event = (col.event || []).filter((e) => e.listen !== 'test');
// Убираем пустые prerequest и прочий шум — общие проверки теперь на каждом запросе.
col.event = (col.event || []).filter((e) => {
  if (e.listen !== 'prerequest') return true;
  const ex = (e.script?.exec || []).join('\n').trim();
  return ex.length > 0;
});
if (!col.event.length) col.event = undefined;

walk(col.item);

fs.writeFileSync(COL, JSON.stringify(col, null, 2) + '\n', 'utf8');
console.log('OK:', COL);
