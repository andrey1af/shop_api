(function () {
  const tbody = document.getElementById('products-tbody');
  const searchInput = document.getElementById('filter-search');
  const categorySelect = document.getElementById('filter-category');
  const supplierSelect = document.getElementById('filter-supplier');

  let products = [];
  let suppliersById = new Map();

  function renderFilterOptions() {
    const categories = [...new Set(products.map((p) => p.category))].sort((a, b) => a.localeCompare(b, 'ru'));
    const currentCategory = categorySelect.value;
    categorySelect.innerHTML = '<option value="">Все категории</option>' +
      categories.map((c) => `<option value="${escapeHtml(c)}">${escapeHtml(c)}</option>`).join('');
    categorySelect.value = categories.includes(currentCategory) ? currentCategory : '';

    const supplierIds = [...new Set(products.map((p) => p.supplier_id))];
    const currentSupplier = supplierSelect.value;
    supplierSelect.innerHTML = '<option value="">Все поставщики</option>' +
      supplierIds
        .map((id) => suppliersById.get(id))
        .filter(Boolean)
        .sort((a, b) => a.name.localeCompare(b.name, 'ru'))
        .map((s) => `<option value="${s.id}">${escapeHtml(s.name)}</option>`)
        .join('');
    supplierSelect.value = supplierIds.includes(currentSupplier) ? currentSupplier : '';
  }

  function rowHtml(p) {
    const supplier = suppliersById.get(p.supplier_id);
    const thumb = p.image_id
      ? `<img class="thumb" data-api-src="/products/${p.id}/image" alt="">`
      : `<div class="thumb-empty" title="Нет изображения"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#7A8090" stroke-width="1.8" stroke-linecap="round"><rect x="3" y="5" width="18" height="14" rx="2"></rect><path d="M3 16l5-5 5 5M14 14l2-2 5 5"></path></svg></div>`;
    return `
      <tr data-id="${p.id}">
        <td>
          <div class="row-thumb">
            ${thumb}
            <div class="row-name">
              <span class="name">${escapeHtml(p.name)}</span>
              <span class="id mono">${shortId(p.id)}</span>
            </div>
          </div>
        </td>
        <td>${escapeHtml(p.category)}</td>
        <td class="num" style="font-weight:600;white-space:nowrap;">${formatPrice(p.price)}</td>
        <td class="num">${stockBadgeHtml(p.available_stock)}</td>
        <td>${supplier ? escapeHtml(supplier.name) : '—'}</td>
        <td style="color:var(--muted);white-space:nowrap;">${formatDateTime(p.last_update_date)}</td>
        <td>
          <div class="row-actions">
            <button type="button" class="btn-sm" data-action="writeoff">Списать</button>
            <button type="button" class="btn-icon danger" data-action="delete" aria-label="Удалить товар"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#D92D20" stroke-width="1.8" stroke-linecap="round"><path d="M4 7h16M10 11v6M14 11v6M6 7l1 13h10l1-13M9 7V4h6v3"></path></svg></button>
          </div>
        </td>
      </tr>
    `;
  }

  function render() {
    let items = products;
    const query = searchInput.value.trim().toLowerCase();
    if (query) items = items.filter((p) => p.name.toLowerCase().includes(query));
    if (categorySelect.value) items = items.filter((p) => p.category === categorySelect.value);
    if (supplierSelect.value) items = items.filter((p) => p.supplier_id === supplierSelect.value);

    if (items.length === 0) {
      tbody.innerHTML = `<tr><td colspan="7" style="padding:32px 24px;color:var(--muted);">Ничего не найдено</td></tr>`;
      return;
    }
    tbody.innerHTML = items.map(rowHtml).join('');
    loadApiImages(tbody);
  }

  function openWriteoffModal(product) {
    const { close } = openOverlay(`
      <div class="modal">
        <div>
          <div class="modal-title">Списать со склада</div>
          <div class="modal-subtitle" id="writeoff-subtitle">${escapeHtml(product.name)} · на складе ${product.available_stock} шт</div>
        </div>
        <label class="field">Количество, шт
          <input type="number" id="writeoff-qty" value="1" min="1" max="${product.available_stock}">
        </label>
        <div class="field-error" id="writeoff-error" hidden></div>
        <div class="modal-actions">
          <button type="button" class="btn btn-secondary" data-role="cancel">Отмена</button>
          <button type="button" class="btn btn-primary" data-role="confirm">Списать</button>
        </div>
      </div>
    `, {
      onMount(overlay, closeOverlay) {
        const qtyInput = overlay.querySelector('#writeoff-qty');
        const errorEl = overlay.querySelector('#writeoff-error');
        overlay.querySelector('[data-role="cancel"]').addEventListener('click', closeOverlay);
        overlay.querySelector('[data-role="confirm"]').addEventListener('click', async () => {
          const qty = parseInt(qtyInput.value, 10);
          if (!qty || qty < 1) {
            qtyInput.classList.add('error');
            errorEl.textContent = 'Введите количество больше нуля';
            errorEl.hidden = false;
            return;
          }
          try {
            await Api.decreaseStock(product.id, qty);
            closeOverlay();
            load();
          } catch (err) {
            qtyInput.classList.add('error');
            errorEl.textContent = friendlyErrorMessage(err);
            errorEl.hidden = false;
          }
        });
      },
    });
    return close;
  }

  tbody.addEventListener('click', async (e) => {
    const btn = e.target.closest('button[data-action]');
    if (!btn) return;
    const tr = btn.closest('tr');
    const id = tr.dataset.id;
    const product = products.find((p) => p.id === id);
    if (!product) return;

    if (btn.dataset.action === 'writeoff') {
      openWriteoffModal(product);
    } else if (btn.dataset.action === 'delete') {
      const ok = await confirmAction({
        title: `Удалить «${product.name}»?`,
        text: 'Это действие нельзя отменить.',
        confirmLabel: 'Удалить',
      });
      if (!ok) return;
      try {
        await Api.deleteProduct(id);
        load();
      } catch (err) {
        alert(friendlyErrorMessage(err));
      }
    }
  });

  searchInput.addEventListener('input', debounce(render, 150));
  categorySelect.addEventListener('change', render);
  supplierSelect.addEventListener('change', render);

  async function load() {
    try {
      const [productList, supplierList] = await Promise.all([Api.listProducts(), Api.listSuppliers()]);
      products = productList;
      suppliersById = new Map(supplierList.map((s) => [s.id, s]));
      renderFilterOptions();

      const presetSupplier = new URLSearchParams(location.search).get('supplier');
      if (presetSupplier && [...supplierSelect.options].some((o) => o.value === presetSupplier)) {
        supplierSelect.value = presetSupplier;
      }

      render();
    } catch (err) {
      tbody.innerHTML = `<tr><td colspan="7" style="padding:32px 24px;color:var(--err-fg);">${escapeHtml(friendlyErrorMessage(err))}</td></tr>`;
    }
  }

  load();
})();
