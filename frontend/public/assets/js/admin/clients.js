(function () {
  const tbody = document.getElementById('clients-tbody');
  const searchForm = document.getElementById('search-form');
  const searchName = document.getElementById('search-name');
  const searchSurname = document.getElementById('search-surname');
  const searchReset = document.getElementById('search-reset');
  const addBtn = document.getElementById('add-client-btn');
  const pageSizeSelect = document.getElementById('page-size');
  const pagePrev = document.getElementById('page-prev');
  const pageNext = document.getElementById('page-next');
  const pageIndicator = document.getElementById('page-indicator');
  const paginationRow = document.getElementById('pagination-row');

  let clients = [];
  let offset = 0;
  let pageSize = 20;
  let hasNextPage = false;
  let searchMode = false;

  function rowHtml(c) {
    return `
      <tr data-id="${c.id}">
        <td>
          <div class="row-name">
            <span class="name">${escapeHtml(c.client_name)} ${escapeHtml(c.client_surname)}</span>
            <span class="id mono">${shortId(c.id)}</span>
          </div>
        </td>
        <td>${GENDER_LABELS[c.gender] || c.gender}</td>
        <td style="white-space:nowrap;">${formatDate(c.birthday)}</td>
        <td>${addressLinkHtml(c.address)}</td>
        <td style="color:var(--muted);white-space:nowrap;">${formatDate(c.registration_date)}</td>
        <td>
          <div class="row-actions">
            <button type="button" class="btn-icon" data-action="edit-address" aria-label="Изменить адрес"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#1B1F2B" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M12 21s-7-6-7-11a7 7 0 0114 0c0 5-7 11-7 11z"></path><circle cx="12" cy="10" r="2.5"></circle></svg></button>
            <button type="button" class="btn-icon danger" data-action="delete" aria-label="Удалить клиента"><svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="#D92D20" stroke-width="1.8" stroke-linecap="round"><path d="M4 7h16M10 11v6M14 11v6M6 7l1 13h10l1-13M9 7V4h6v3"></path></svg></button>
          </div>
        </td>
      </tr>
    `;
  }

  function render() {
    tbody.innerHTML = clients.length
      ? clients.map(rowHtml).join('')
      : `<tr><td colspan="6" style="padding:32px 24px;color:var(--muted);">Ничего не найдено</td></tr>`;

    paginationRow.style.display = searchMode ? 'none' : 'flex';
    if (!searchMode) {
      const page = Math.floor(offset / pageSize) + 1;
      pageIndicator.textContent = `Страница ${page}`;
      pagePrev.disabled = offset === 0;
      pageNext.disabled = !hasNextPage;
    }
  }

  async function loadList() {
    tbody.innerHTML = `<tr><td colspan="6" style="padding:32px 24px;color:var(--muted);">Загрузка…</td></tr>`;
    try {
      const items = await Api.listClients(pageSize + 1, offset);
      hasNextPage = items.length > pageSize;
      clients = items.slice(0, pageSize);
      render();
    } catch (err) {
      tbody.innerHTML = `<tr><td colspan="6" style="padding:32px 24px;color:var(--err-fg);">${escapeHtml(friendlyErrorMessage(err))}</td></tr>`;
    }
  }

  async function loadSearch() {
    tbody.innerHTML = `<tr><td colspan="6" style="padding:32px 24px;color:var(--muted);">Загрузка…</td></tr>`;
    try {
      clients = await Api.searchClients(searchName.value.trim(), searchSurname.value.trim());
      render();
    } catch (err) {
      tbody.innerHTML = `<tr><td colspan="6" style="padding:32px 24px;color:var(--err-fg);">${escapeHtml(friendlyErrorMessage(err))}</td></tr>`;
    }
  }

  function load() {
    if (searchMode) loadSearch();
    else loadList();
  }

  searchForm.addEventListener('submit', (e) => {
    e.preventDefault();
    if (!searchName.value.trim() || !searchSurname.value.trim()) return;
    searchMode = true;
    searchReset.hidden = false;
    load();
  });

  searchReset.addEventListener('click', () => {
    searchMode = false;
    searchReset.hidden = true;
    searchName.value = '';
    searchSurname.value = '';
    offset = 0;
    load();
  });

  pageSizeSelect.addEventListener('change', () => {
    pageSize = parseInt(pageSizeSelect.value, 10);
    offset = 0;
    load();
  });

  pagePrev.addEventListener('click', () => {
    offset = Math.max(0, offset - pageSize);
    load();
  });

  pageNext.addEventListener('click', () => {
    if (!hasNextPage) return;
    offset += pageSize;
    load();
  });

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
        <div class="modal-title" style="font-size:20px;">Новый клиент</div>
        <div class="field-row">
          <label class="field">Имя
            <input type="text" id="f-name" maxlength="100" required>
          </label>
          <label class="field">Фамилия
            <input type="text" id="f-surname" maxlength="100" required>
          </label>
        </div>
        <div class="field-row">
          <label class="field">Дата рождения
            <input type="date" id="f-birthday" required>
          </label>
          <label class="field">Пол
            <select id="f-gender" required>
              <option value="male">Мужской</option>
              <option value="female">Женский</option>
              <option value="other">Другой</option>
            </select>
          </label>
        </div>
        ${addressFieldsHtml()}
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
            await Api.createClient({
              client_name: overlay.querySelector('#f-name').value.trim(),
              client_surname: overlay.querySelector('#f-surname').value.trim(),
              birthday: overlay.querySelector('#f-birthday').value,
              gender: overlay.querySelector('#f-gender').value,
              address: {
                country: overlay.querySelector('#f-country').value.trim(),
                city: overlay.querySelector('#f-city').value.trim(),
                street: overlay.querySelector('#f-street').value.trim(),
              },
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

  function openEditAddressModal(client) {
    openOverlay(`
      <aside class="slide-panel">
        <div class="panel-head">
          <div>
            <h2>Изменить адрес</h2>
            <span class="sub">${escapeHtml(client.client_name)} ${escapeHtml(client.client_surname)}</span>
          </div>
          <button type="button" class="btn-icon" data-role="cancel" aria-label="Закрыть"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="#1B1F2B" stroke-width="2" stroke-linecap="round"><path d="M6 6l12 12M18 6L6 18"></path></svg></button>
          </div>
        <div class="current-box">
          <span class="cap">Сейчас</span>
          <span class="val">${addressLinkHtml(client.address)}</span>
        </div>
        <form id="edit-form" style="display:flex;flex-direction:column;gap:22px;flex-grow:1;">
          ${addressFieldsHtml(client.address)}
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
            await Api.updateClientAddress(client.id, {
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

  tbody.addEventListener('click', async (e) => {
    const btn = e.target.closest('button[data-action]');
    if (!btn) return;
    const tr = btn.closest('tr');
    const client = clients.find((c) => c.id === tr.dataset.id);
    if (!client) return;

    if (btn.dataset.action === 'edit-address') {
      openEditAddressModal(client);
    } else if (btn.dataset.action === 'delete') {
      const ok = await confirmAction({
        title: `Удалить клиента «${client.client_name} ${client.client_surname}»?`,
        text: 'Это действие нельзя отменить.',
      });
      if (!ok) return;
      try {
        await Api.deleteClient(client.id);
        load();
      } catch (err) {
        alert(friendlyErrorMessage(err));
      }
    }
  });

  load();
})();
