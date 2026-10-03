const LIVE_FEED_LIMIT = 20;

const LIVE_STATUS_LABELS = {
  connecting: 'Подключение…',
  live: 'В эфире',
  offline: 'Нет соединения',
};

const LiveFeed = {
  create(root) {
    const list = root.querySelector('.live-feed-list');
    const empty = root.querySelector('.live-feed-empty');
    const status = root.querySelector('.live-status');
    const statusText = root.querySelector('.live-status-text');

    function setStatus(value) {
      status.dataset.status = value;
      statusText.textContent = LIVE_STATUS_LABELS[value] || value;
    }

    function add(update, previous) {
      const time = new Date().toLocaleTimeString('ru-RU', { hour: '2-digit', minute: '2-digit', second: '2-digit' });

      const item = el(`
        <li class="live-feed-item">
          <span class="live-feed-time mono">${time}</span>
          <a class="live-feed-name" href="/product.html?id=${encodeURIComponent(update.id)}">${escapeHtml(update.name)}</a>
          <span class="live-feed-change">
            <span class="cap">Цена</span>
            ${changeHtml(previous && previous.price, update.price, formatPrice)}
          </span>
          <span class="live-feed-change">
            <span class="cap">Остаток</span>
            ${changeHtml(previous && previous.available_stock, update.available_stock, (n) => `${n} шт`)}
          </span>
        </li>
      `);

      list.prepend(item);
      while (list.children.length > LIVE_FEED_LIMIT) list.lastElementChild.remove();
      empty.hidden = true;
    }

    return { add, setStatus };
  },
};

function changeHtml(before, after, format) {
  if (before === null || before === undefined || before === after) {
    return `<span class="live-feed-value">${escapeHtml(format(after))}</span>`;
  }
  const up = after > before;
  return `
    <span class="live-feed-old">${escapeHtml(format(before))}</span>
    <span class="live-feed-value ${up ? 'live-feed-up' : 'live-feed-down'}">${up ? '▲' : '▼'} ${escapeHtml(format(after))}</span>
  `;
}
