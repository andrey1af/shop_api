(function () {
  const params = new URLSearchParams(location.search);
  const productId = params.get('id');

  const main = document.getElementById('product-main');
  const errorBox = document.getElementById('product-error');
  const titleEl = document.getElementById('product-title');
  const imageWrap = document.getElementById('product-image-wrap');
  const specsEl = document.getElementById('product-specs');
  const priceEl = document.getElementById('product-price');
  const stockBadgeEl = document.getElementById('product-stock-badge');
  const qtyInput = document.getElementById('qty-input');
  const qtyMinus = document.getElementById('qty-minus');
  const qtyPlus = document.getElementById('qty-plus');
  const buyBtn = document.getElementById('buy-btn');
  const buyMessage = document.getElementById('buy-message');
  const supplierInfo = document.getElementById('supplier-info');

  let product = null;

  function showError(message) {
    main.hidden = true;
    errorBox.hidden = false;
    errorBox.textContent = message;
    titleEl.textContent = 'Товар не найден';
  }

  function renderProduct() {
    titleEl.textContent = product.name;
    imageWrap.innerHTML = product.image_id
      ? `<img data-api-src="/products/${product.id}/image" alt="${escapeHtml(product.name)}" class="product-photo">`
      : `<div class="ph product-photo">Фото товара</div>`;
    loadApiImages(imageWrap);

    specsEl.innerHTML = `
      <dt>Категория</dt><dd>${escapeHtml(product.category)}</dd>
      <dt>Артикул</dt><dd class="mono">${shortId(product.id)}</dd>
      <dt>Остаток обновлён</dt><dd id="product-updated-at">${formatDateTimeLong(product.last_update_date)}</dd>
    `;

    renderPriceAndStock();
    qtyInput.value = product.available_stock <= 0 ? '0' : '1';
  }

  function renderPriceAndStock() {
    priceEl.textContent = formatPrice(product.price);
    stockBadgeEl.innerHTML = stockBadgeHtml(product.available_stock);
    const updatedAtEl = document.getElementById('product-updated-at');
    if (updatedAtEl) updatedAtEl.textContent = formatDateTimeLong(product.last_update_date);

    const outOfStock = product.available_stock <= 0;
    qtyInput.max = String(Math.max(product.available_stock, 1));
    if (outOfStock) {
      qtyInput.value = '0';
    } else {
      const qty = parseInt(qtyInput.value, 10) || 1;
      qtyInput.value = String(Math.min(Math.max(qty, 1), product.available_stock));
    }
    qtyInput.disabled = outOfStock;
    qtyMinus.disabled = outOfStock;
    qtyPlus.disabled = outOfStock;
    buyBtn.disabled = outOfStock;
    buyBtn.textContent = outOfStock ? 'Нет в наличии' : 'Купить';
  }

  async function renderSupplier() {
    try {
      const supplier = await Api.getSupplier(product.supplier_id);
      const addr = supplier.address;
      supplierInfo.innerHTML = `
        <div style="font-size:17px;font-weight:600;">${escapeHtml(supplier.name)}</div>
        <div style="font-size:15px;color:var(--muted);line-height:1.45;">${addressLinkHtml(addr)}</div>
        <a href="tel:${escapeHtml(supplier.phone_number)}" class="mono" style="font-size:15px;">${escapeHtml(supplier.phone_number)}</a>
      `;
    } catch (err) {
      supplierInfo.innerHTML = `<div class="field-hint">Не удалось загрузить поставщика</div>`;
    }
  }

  function clampQty() {
    const max = Number(qtyInput.max) || 1;
    let v = parseInt(qtyInput.value, 10);
    if (Number.isNaN(v) || v < 1) v = 1;
    if (v > max) v = max;
    qtyInput.value = String(v);
  }

  qtyMinus.addEventListener('click', () => {
    qtyInput.value = String(Math.max(1, (parseInt(qtyInput.value, 10) || 1) - 1));
  });
  qtyPlus.addEventListener('click', () => {
    clampQty();
    const max = Number(qtyInput.max) || 1;
    qtyInput.value = String(Math.min(max, (parseInt(qtyInput.value, 10) || 1) + 1));
  });
  qtyInput.addEventListener('change', clampQty);

  buyBtn.addEventListener('click', async () => {
    clampQty();
    const qty = parseInt(qtyInput.value, 10);
    buyBtn.disabled = true;
    buyMessage.hidden = true;
    try {
      product = await Api.decreaseStock(product.id, qty);
      renderProduct();
      buyMessage.className = 'msg msg-ok';
      buyMessage.textContent = `Куплено ${qty} шт. Остаток: ${product.available_stock} шт.`;
      buyMessage.hidden = false;
    } catch (err) {
      buyMessage.className = 'msg msg-error';
      buyMessage.textContent = friendlyErrorMessage(err);
      buyMessage.hidden = false;
      buyBtn.disabled = product.available_stock <= 0;
    }
  });

  async function load() {
    if (!productId) {
      showError('Не указан идентификатор товара');
      return;
    }
    try {
      product = await Api.getProduct(productId);
      main.hidden = false;
      renderProduct();
      renderSupplier();
    } catch (err) {
      showError(friendlyErrorMessage(err));
    }
  }

  function applyLiveUpdate(update) {
    if (!product || update.id !== product.id) return;

    const priceDirection = Math.sign(update.price - product.price);
    const stockDirection = Math.sign(update.available_stock - product.available_stock);
    Object.assign(product, {
      price: update.price,
      available_stock: update.available_stock,
      last_update_date: update.last_update_date,
    });

    renderPriceAndStock();
    flashChange(priceEl, priceDirection);
    flashChange(stockBadgeEl, stockDirection);
  }

  load();
  ProductLive.subscribe(applyLiveUpdate);
})();
