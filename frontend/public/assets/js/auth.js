(function () {
  const auth = AuthStore.peek();
  if (!auth) {
    AuthStore.clear();
    redirectToLogin();
    return;
  }

  const USER_ICON = '<svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><circle cx="12" cy="8" r="4"></circle><path d="M4 21c0-4 4-6 8-6s8 2 8 6"></path></svg>';

  async function logout(all = false) {
    try {
      await (all ? Api.logoutAll() : Api.logout());
    } catch {}
    AuthStore.clear();
    window.location.assign(LOGIN_PAGE);
  }

  function openChangePasswordModal() {
    openOverlay(`
      <form class="modal-form" novalidate>
        <div>
          <div class="modal-title" style="font-size:20px;">Смена пароля</div>
          <div class="modal-subtitle" style="margin-top:6px">${escapeHtml(auth.email || '')}</div>
        </div>
        <label class="field">Текущий пароль
          <input type="password" name="old" autocomplete="current-password" required>
        </label>
        <label class="field">Новый пароль
          <input type="password" name="new" autocomplete="new-password" required>
        </label>
        <p class="msg msg-error" data-role="error" hidden></p>
        <div class="modal-actions">
          <button type="button" class="btn btn-secondary" data-role="cancel">Отмена</button>
          <button type="submit" class="btn btn-primary">Сменить пароль</button>
        </div>
      </form>
    `, {
      onMount(overlay, close) {
        const form = overlay.querySelector('form');
        const errorEl = overlay.querySelector('[data-role="error"]');
        const submit = form.querySelector('button[type="submit"]');

        overlay.querySelector('[data-role="cancel"]').addEventListener('click', close);
        form.old.focus();

        form.addEventListener('submit', async (e) => {
          e.preventDefault();
          errorEl.hidden = true;
          if (!form.old.value || !form.new.value) {
            errorEl.textContent = 'Заполните оба поля';
            errorEl.hidden = false;
            return;
          }

          submit.disabled = true;
          try {
            const res = await Api.changePassword(form.old.value, form.new.value);
            AuthStore.save(res.token, auth.email);
            close();
          } catch (err) {
            errorEl.textContent = err instanceof ApiError && err.code === 'INVALID_CREDENTIALS'
              ? 'Текущий пароль указан неверно'
              : friendlyErrorMessage(err);
            errorEl.hidden = false;
          } finally {
            submit.disabled = false;
          }
        });
      },
    });
  }

  function mountUserMenu() {
    const host = document.querySelector('.admin-header-right') || document.querySelector('.header-top');
    if (!host) return;

    let container = host;
    if (host.classList.contains('header-top')) {
      container = el('<div class="header-actions"></div>');
      const adminLink = host.querySelector('.chip-link');
      host.appendChild(container);
      if (adminLink) container.appendChild(adminLink);
    }

    const menu = el(`
      <div class="user-menu">
        <button type="button" class="chip-icon" aria-haspopup="menu" aria-expanded="false" title="${escapeHtml(auth.email || 'Аккаунт')}">${USER_ICON}</button>
        <div class="user-menu-panel" role="menu" hidden>
          <div class="user-menu-email">${escapeHtml(auth.email || '')}</div>
          <button type="button" role="menuitem" data-action="change-password">Сменить пароль</button>
          <button type="button" role="menuitem" data-action="logout">Выйти</button>
          <button type="button" role="menuitem" data-action="logout-all">Выйти на всех устройствах</button>
        </div>
      </div>
    `);
    container.appendChild(menu);

    const toggle = menu.querySelector('.chip-icon');
    const panel = menu.querySelector('.user-menu-panel');
    const setOpen = (open) => {
      panel.hidden = !open;
      toggle.setAttribute('aria-expanded', String(open));
    };

    toggle.addEventListener('click', () => setOpen(panel.hidden));
    document.addEventListener('click', (e) => {
      if (!menu.contains(e.target)) setOpen(false);
    });
    panel.querySelector('[data-action="change-password"]').addEventListener('click', () => {
      setOpen(false);
      openChangePasswordModal();
    });
    panel.querySelector('[data-action="logout"]').addEventListener('click', () => logout(false));
    panel.querySelector('[data-action="logout-all"]').addEventListener('click', () => logout(true));
  }

  mountUserMenu();
})();
