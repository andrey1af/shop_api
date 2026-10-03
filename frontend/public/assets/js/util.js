const GENDER_LABELS = { male: 'Мужской', female: 'Женский', other: 'Другой' };
const LOW_STOCK_THRESHOLD = 3;

function escapeHtml(str) {
  return String(str).replace(/[&<>"']/g, (c) => ({
    '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;',
  }[c]));
}

function formatPrice(price) {
  return `${new Intl.NumberFormat('ru-RU', { maximumFractionDigits: 0 }).format(Math.round(price))} ₽`;
}

function formatDateTime(iso) {
  const d = new Date(iso);
  const date = d.toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' });
  const time = d.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' });
  return `${date} ${time}`;
}

function formatDateTimeLong(iso) {
  const d = new Date(iso);
  const date = d.toLocaleDateString('ru-RU', { day: 'numeric', month: 'long', year: 'numeric' });
  const time = d.toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit' });
  return `${date}, ${time}`;
}

function formatDate(iso) {
  const d = new Date(iso);
  return d.toLocaleDateString('ru-RU', { day: '2-digit', month: '2-digit', year: 'numeric' });
}

function dateInputToApi(value) {
  return value;
}

function stockBadgeHtml(qty) {
  const low = qty <= LOW_STOCK_THRESHOLD;
  const cls = low ? 'badge badge-low' : 'badge badge-ok';
  const text = low ? `Осталось ${qty} шт` : `В наличии ${qty} шт`;
  return `<span class="${cls}">${text}</span>`;
}

function shortId(id) {
  return id ? id.slice(0, 8) : '';
}

function formatAddress(addr) {
  return `${addr.country}, ${addr.city}, ${addr.street}`;
}

function yandexMapsUrl(addr) {
  return `https://yandex.ru/maps/?text=${encodeURIComponent(formatAddress(addr))}`;
}

const MAP_PIN_ICON = '<svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M18 13v6a2 2 0 01-2 2H5a2 2 0 01-2-2V8a2 2 0 012-2h6"></path><path d="M15 3h6v6"></path><path d="M10 14L21 3"></path></svg>';

function addressLinkHtml(addr, { className = 'addr-link' } = {}) {
  return `<a class="${className}" href="${yandexMapsUrl(addr)}" target="_blank" rel="noopener noreferrer" title="Открыть адрес в Яндекс Картах">${escapeHtml(formatAddress(addr))}${MAP_PIN_ICON}</a>`;
}

function debounce(fn, wait) {
  let t;
  return (...args) => {
    clearTimeout(t);
    t = setTimeout(() => fn(...args), wait);
  };
}

function el(html) {
  const tpl = document.createElement('template');
  tpl.innerHTML = html.trim();
  return tpl.content.firstElementChild;
}

function friendlyErrorMessage(err) {
  if (err instanceof ApiError) return err.message;
  return 'Произошла непредвиденная ошибка';
}
