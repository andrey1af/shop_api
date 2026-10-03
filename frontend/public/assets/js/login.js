(function () {
  const forms = {
    login: document.getElementById('login-form'),
    register: document.getElementById('register-form'),
    reset: document.getElementById('reset-form'),
  };
  const tabs = document.querySelectorAll('.auth-tabs button');
  const oauthBlock = document.getElementById('oauth-block');

  function nextUrl() {
    const next = new URLSearchParams(window.location.search).get('next');
    return next && next.startsWith('/') && !next.startsWith('//') && !next.startsWith(LOGIN_PAGE) ? next : '/';
  }

  if (AuthStore.get()) {
    window.location.replace(nextUrl());
    return;
  }

  function showTab(name) {
    Object.entries(forms).forEach(([key, form]) => { form.hidden = key !== name; });
    tabs.forEach((tab) => tab.classList.toggle('active', tab.dataset.tab === name));
    oauthBlock.hidden = name === 'reset';
    const first = forms[name].querySelector('input');
    if (first) first.focus();
  }

  document.querySelectorAll('[data-tab]').forEach((btn) => {
    btn.addEventListener('click', () => showTab(btn.dataset.tab));
  });

  function setError(form, message) {
    const errorEl = form.querySelector('[data-role="error"]');
    errorEl.textContent = message || '';
    errorEl.hidden = !message;
  }

  function handleSubmit(form, action) {
    const submit = form.querySelector('button[type="submit"]');
    form.addEventListener('submit', async (e) => {
      e.preventDefault();
      setError(form, '');

      const empty = [...form.querySelectorAll('input[required]')].find((input) => !input.value.trim());
      if (empty) {
        setError(form, 'Заполните все поля');
        empty.focus();
        return;
      }

      submit.disabled = true;
      try {
        await action(form);
      } catch (err) {
        setError(form, err instanceof ApiError && err.code === 'INVALID_CREDENTIALS'
          ? 'Неверный email или пароль'
          : friendlyErrorMessage(err));
      } finally {
        submit.disabled = false;
      }
    });
  }

  handleSubmit(forms.login, async (form) => {
    const email = form.email.value.trim();
    const res = await Api.login(email, form.password.value);
    AuthStore.save(res.token, email);
    window.location.assign(nextUrl());
  });

  handleSubmit(forms.register, async (form) => {
    const email = form.email.value.trim();
    const res = await Api.register({
      email,
      name: form.name.value.trim(),
      surname: form.surname.value.trim(),
      phone_number: form.phone_number.value.trim(),
      password: form.password.value,
    });
    AuthStore.save(res.token, email);
    window.location.assign(nextUrl());
  });

  handleSubmit(forms.reset, async (form) => {
    const okEl = form.querySelector('[data-role="ok"]');
    okEl.hidden = true;
    await Api.resetPassword(form.email.value.trim());
    okEl.textContent = 'Если учётная запись существует, ей назначен временный пароль.';
    okEl.hidden = false;
  });

  oauthBlock.querySelectorAll('[data-oauth-provider]').forEach((btn) => {
    btn.addEventListener('click', async () => {
      setError(oauthBlock, '');
      btn.disabled = true;
      try {
        const res = await Api.oauthStart(btn.dataset.oauthProvider);
        try { sessionStorage.setItem(OAUTH_NEXT_STORAGE_KEY, nextUrl()); } catch {}
        window.location.assign(res.auth_url);
      } catch (err) {
        setError(oauthBlock, friendlyErrorMessage(err));
        btn.disabled = false;
      }
    });
  });

  forms.login.email.focus();
})();
