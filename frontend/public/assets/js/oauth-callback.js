(function () {
  const statusEl = document.querySelector('[data-role="status"]');
  const errorEl = document.querySelector('[data-role="error"]');
  const backEl = document.querySelector('[data-role="back"]');

  function fail(message) {
    statusEl.hidden = true;
    errorEl.textContent = message;
    errorEl.hidden = false;
    backEl.hidden = false;
  }

  function nextUrl() {
    let next = null;
    try {
      next = sessionStorage.getItem(OAUTH_NEXT_STORAGE_KEY);
      sessionStorage.removeItem(OAUTH_NEXT_STORAGE_KEY);
    } catch {}
    return next && next.startsWith('/') && !next.startsWith('//') && !next.startsWith(LOGIN_PAGE) ? next : '/';
  }

  const params = new URLSearchParams(window.location.search);
  const code = params.get('code');
  const state = params.get('state');

  if (params.get('error')) {
    fail(params.get('error') === 'access_denied' ? 'Вход отменён' : 'Провайдер не подтвердил вход');
    return;
  }
  if (!code || !state) {
    fail('Не хватает данных для входа');
    return;
  }

  window.history.replaceState(null, '', window.location.pathname);

  Api.oauthCallback(code, state)
    .then((res) => {
      AuthStore.save(res.token, res.email);
      window.location.replace(nextUrl());
    })
    .catch((err) => {
      if (err instanceof ApiError && err.code === 'USER_ALREADY_EXISTS') {
        fail('Пользователь с таким email уже существует. Войдите по паролю');
      } else if (err instanceof ApiError && err.code === 'OAUTH_FAILED') {
        fail('Не удалось войти: ссылка устарела или вход не подтверждён. Попробуйте ещё раз');
      } else {
        fail(friendlyErrorMessage(err));
      }
    });
})();
