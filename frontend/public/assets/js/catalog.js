(function () {
  const grid = document.getElementById('catalog-grid');
  const countEl = document.getElementById('catalog-count');
  const nav = document.getElementById('category-nav');
  const searchInput = document.getElementById('search-input');
  const sortSelect = document.getElementById('sort-select');
  const liveFeed = LiveFeed.create(document.getElementById('live-feed'));

  let allProducts = [];
  let activeCategory = '';

  function pluralize(n, one, few, many) {
    const mod10 = n % 10;
    const mod100 = n % 100;
    if (mod10 === 1 && mod100 !== 11) return one;
    if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) return few;
    return many;
  }

  function renderCategories() {
    const categories = [...new Set(allProducts.map((p) => p.category))].sort((a, b) => a.localeCompare(b, 'ru'));
    nav.innerHTML = '';
    const all = el(`<button type="button" data-category="">Все</button>`);
    nav.appendChild(all);
    categories.forEach((c) => {
      nav.appendChild(el(`<button type="button" data-category="${escapeHtml(c)}">${escapeHtml(c)}</button>`));
    });
    nav.querySelectorAll('button').forEach((btn) => {
      btn.classList.toggle('active', btn.dataset.category === activeCategory);
      btn.addEventListener('click', () => {
        activeCategory = btn.dataset.category;
        nav.querySelectorAll('button').forEach((b) => b.classList.toggle('active', b === btn));
        render();
      });
    });
  }

  function cardHtml(p) {
    const thumb = p.image_id
      ? `<img data-api-src="/products/${p.id}/image" alt="${escapeHtml(p.name)}">`
      : `<div class="ph">Фото товара</div>`;
    return `
      <article class="product-card" data-product-id="${p.id}">
        <div class="thumb-wrap">${thumb}</div>
        <div class="body">
          <div class="category">${escapeHtml(p.category)}</div>
          <h2>${escapeHtml(p.name)}</h2>
          <div class="price-row">
            <div class="price">${formatPrice(p.price)}</div>
            <span class="stock">${stockBadgeHtml(p.available_stock)}</span>
          </div>
          <a class="more-link" href="/product.html?id=${p.id}">Подробнее</a>
        </div>
      </article>
    `;
  }

  function render() {
    let items = allProducts;
    if (activeCategory) items = items.filter((p) => p.category === activeCategory);
    const query = searchInput.value.trim().toLowerCase();
    if (query) items = items.filter((p) => p.name.toLowerCase().includes(query));

    const sort = sortSelect.value;
    items = [...items].sort((a, b) => {
      if (sort === 'price-asc') return a.price - b.price;
      if (sort === 'price-desc') return b.price - a.price;
      return a.name.localeCompare(b.name, 'ru');
    });

    countEl.textContent = `${items.length} ${pluralize(items.length, 'товар', 'товара', 'товаров')}`;

    if (items.length === 0) {
      grid.innerHTML = `<div class="empty-state" style="grid-column:1/-1">Ничего не найдено</div>`;
      return;
    }
    grid.innerHTML = items.map(cardHtml).join('');
    loadApiImages(grid);
  }

  async function load() {
    try {
      allProducts = await Api.listProducts();
      renderCategories();
      render();
    } catch (err) {
      countEl.textContent = 'Ошибка загрузки';
      grid.innerHTML = `<div class="empty-state" style="grid-column:1/-1">${escapeHtml(friendlyErrorMessage(err))}</div>`;
    }
  }

  function applyLiveUpdate(update) {
    const product = allProducts.find((p) => p.id === update.id);
    liveFeed.add(update, product ? { ...product } : null);
    if (!product) return;

    const priceDirection = Math.sign(update.price - product.price);
    const stockDirection = Math.sign(update.available_stock - product.available_stock);
    Object.assign(product, {
      price: update.price,
      available_stock: update.available_stock,
      last_update_date: update.last_update_date,
    });

    const card = grid.querySelector(`[data-product-id="${product.id}"]`);
    if (!card) return;

    const priceEl = card.querySelector('.price');
    priceEl.textContent = formatPrice(product.price);
    flashChange(priceEl, priceDirection);

    const stockEl = card.querySelector('.stock');
    stockEl.innerHTML = stockBadgeHtml(product.available_stock);
    flashChange(stockEl, stockDirection);
  }

  searchInput.addEventListener('input', debounce(render, 150));
  sortSelect.addEventListener('change', render);

  load();
  ProductLive.subscribe(applyLiveUpdate, { onStatus: liveFeed.setStatus });
})();
