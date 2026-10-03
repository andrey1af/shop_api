(function () {
  const grid = document.getElementById('supplier-grid');
  const countEl = document.getElementById('supplier-count');
  const addBtn = document.getElementById('add-supplier-btn');
  const conflictAlert = document.getElementById('conflict-alert');
  const conflictTitle = document.getElementById('conflict-title');
  const conflictLink = document.getElementById('conflict-link');
  const conflictClose = document.getElementById('conflict-close');

  let suppliers = [];

  function pluralizeSuppliers(n) {
    const mod10 = n % 10;
    const mod100 = n % 100;
    if (mod10 === 1 && mod100 !== 11) return 'поставщик';
    if (mod10 >= 2 && mod10 <= 4 && (mod100 < 10 || mod100 >= 20)) return 'поставщика';
    return 'поставщиков';
  }

  function cardHtml(s) {
    return `
      <article class="supplier-card-item" data-id="${s.id}">
        <div class="head">
          <div>
            <h2>${escapeHtml(s.name)}</h2>
            <span class="mono" style="font-size:12px;color:var(--muted);">${shortId(s.id)}</span>
          </div>
          <div class="actions">
            <button type="button" class="btn-icon" data-action="edit-address" aria-label="Изменить адрес"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#1B1F2B" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M12 21s-7-6-7-11a7 7 0 0114 0c0 5-7 11-7 11z"></path><circle cx="12" cy="10" r="2.5"></circle></svg></button>
            <button type="button" class="btn-icon danger" data-action="delete" aria-label="Удалить поставщика"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#D92D20" stroke-width="1.8" stroke-linecap="round"><path d="M4 7h16M10 11v6M14 11v6M6 7l1 13h10l1-13M9 7V4h6v3"></path></svg></button>
          </div>
        </div>
        <div class="addr-box">
          <span class="line1">${addressLinkHtml(s.address)}</span>
          <span class="mono" style="font-size:13px;">${escapeHtml(s.phone_number)}</span>
        </div>
      </article>
    `;
  }

  function render() {
    countEl.textContent = `${suppliers.length} ${pluralizeSuppliers(suppliers.length)}`;
    grid.innerHTML = suppliers.length
      ? suppliers.map(cardHtml).join('')
      : `<div class="empty-state" style="grid-column:1/-1">Поставщиков пока нет</div>`;
  }

  function addressFieldsHtml(addr = { country: '', city: '', street: '' }) {
    return `
      <label class="field">Страна
        <input type="text" id="f-country" maxlength="100" value="${escapeHtml(addr.country)}" required>
      </label>
      <label class="field">Город
        <input type="text" id="f-city" maxlength="100" value="${escapeHtml(addr.city)}" required>
      </label>
      <label class="field">Улица, дом
        <input type="text" id="f-street" maxlength="255" value="${escapeHtml(addr.street)}" required>
      </label>
    `;
  }

  function openCreateModal() {
    openOverlay(`
      <form class="modal-form" id="create-form">
        <div>
          <div class="modal-title" style="font-size:20px;">Новый поставщик</div>
        </div>
        <label class="field">Название
          <input type="text" id="f-name" maxlength="255" required>
        </label>
        ${addressFieldsHtml()}
        <label class="field">Телефон
          <input type="tel" id="f-phone" placeholder="+79991234567" required>
          <span class="field-hint">Формат E.164, например +79991234567</span>
        </label>
        <p class="msg msg-error" id="create-error" hidden></p>
        <div class="modal-actions">
          <button type="button" class="btn btn-secondary" data-role="cancel">Отмена</button>
          <button type="submit" class="btn btn-primary">Добавить</button>
        </div>
      </form>
    `, {
      onMount(overlay, close) {
        overlay.querySelector('[data-role="cancel"]').addEventListener('click', close);
        overlay.querySelector('#create-form').addEventListener('submit', async (e) => {
          e.preventDefault();
          const errorEl = overlay.querySelector('#create-error');
          errorEl.hidden = true;
          try {
            await Api.createSupplier({
              name: overlay.querySelector('#f-name').value.trim(),
              address: {
                country: overlay.querySelector('#f-country').value.trim(),
                city: overlay.querySelector('#f-city').value.trim(),
                street: overlay.querySelector('#f-street').value.trim(),
              },
              phone_number: overlay.querySelector('#f-phone').value.trim(),
            });
            close();
            load();
          } catch (err) {
            errorEl.textContent = friendlyErrorMessage(err);
            errorEl.hidden = false;
          }
        });
      },
    });
  }

  function openEditAddressModal(supplier) {
    openOverlay(`
      <aside class="slide-panel">
        <div class="panel-head">
          <div>
            <h2>Изменить адрес</h2>
            <span class="sub">${escapeHtml(supplier.name)}</span>
          </div>
          <button type="button" class="btn-icon" data-role="cancel" aria-label="Закрыть"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#1B1F2B" stroke-width="2" stroke-linecap="round"><path d="M6 6l12 12M18 6L6 18"></path></svg></button>
        </div>
        <div class="current-box">
          <span class="cap">Сейчас</span>
          <span class="val">${addressLinkHtml(supplier.address)}</span>
        </div>
        <form id="edit-form" style="display:flex;flex-direction:column;gap:22px;flex-grow:1;">
          ${addressFieldsHtml(supplier.address)}
          <p class="msg msg-error" id="edit-error" hidden></p>
          <div class="modal-actions" style="margin-top:auto;">
            <button type="button" class="btn btn-secondary" style="flex-grow:1;" data-role="cancel">Отмена</button>
            <button type="submit" class="btn btn-primary" style="flex-grow:1;">Сохранить адрес</button>
          </div>
        </form>
      </aside>
    `, {
      onMount(overlay, close) {
        overlay.querySelectorAll('[data-role="cancel"]').forEach((b) => b.addEventListener('click', close));
        overlay.querySelector('#edit-form').addEventListener('submit', async (e) => {
          e.preventDefault();
          const errorEl = overlay.querySelector('#edit-error');
          errorEl.hidden = true;
          try {
            await Api.updateSupplierAddress(supplier.id, {
              country: overlay.querySelector('#f-country').value.trim(),
              city: overlay.querySelector('#f-city').value.trim(),
              street: overlay.querySelector('#f-street').value.trim(),
            });
            close();
            load();
          } catch (err) {
            errorEl.textContent = friendlyErrorMessage(err);
            errorEl.hidden = false;
          }
        });
      },
    });
  }

  addBtn.addEventListener('click', openCreateModal);
  conflictClose.addEventListener('click', () => { conflictAlert.hidden = true; });

  grid.addEventListener('click', async (e) => {
    const btn = e.target.closest('button[data-action]');
    if (!btn) return;
    const card = btn.closest('[data-id]');
    const supplier = suppliers.find((s) => s.id === card.dataset.id);
    if (!supplier) return;

    if (btn.dataset.action === 'edit-address') {
      openEditAddressModal(supplier);
    } else if (btn.dataset.action === 'delete') {
      const ok = await confirmAction({
        title: `Удалить «${supplier.name}»?`,
        text: 'Это действие нельзя отменить.',
      });
      if (!ok) return;
      try {
        await Api.deleteSupplier(supplier.id);
        conflictAlert.hidden = true;
        load();
      } catch (err) {
        if (err instanceof ApiError && err.code === 'SUPPLIER_IN_USE') {
          conflictTitle.textContent = `Нельзя удалить «${supplier.name}»`;
          conflictLink.href = `/admin/products.html?supplier=${supplier.id}`;
          conflictAlert.hidden = false;
        } else {
          alert(friendlyErrorMessage(err));
        }
      }
    }
  });

  async function load() {
    try {
      suppliers = await Api.listSuppliers();
      render();
    } catch (err) {
      grid.innerHTML = `<div class="empty-state" style="grid-column:1/-1">${escapeHtml(friendlyErrorMessage(err))}</div>`;
    }
  }

  load();
})();
