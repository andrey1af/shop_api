const API_BASE = '/api/v1';
const LOGIN_PAGE = '/login.html';
const AUTH_STORAGE_KEY = 'shop.auth';
const OAUTH_NEXT_STORAGE_KEY = 'shop.oauth.next';

const AuthStore = {
  get() {
    const auth = AuthStore.peek();
    if (!auth) return null;
    if (auth.expiresAt && new Date(auth.expiresAt) <= new Date()) return null;
    return auth;
  },
  peek() {
    try {
      const auth = JSON.parse(localStorage.getItem(AUTH_STORAGE_KEY));
      return auth && auth.accessToken ? auth : null;
    } catch {
      return null;
    }
  },
  save(token, email) {
    try {
      localStorage.setItem(AUTH_STORAGE_KEY, JSON.stringify({
        accessToken: token.access_token,
        expiresAt: token.expires_at,
        email,
      }));
    } catch {}
  },
  clear() {
    try { localStorage.removeItem(AUTH_STORAGE_KEY); } catch {}
  },
};

function redirectToLogin() {
  const next = window.location.pathname + window.location.search;
  window.location.assign(`${LOGIN_PAGE}?next=${encodeURIComponent(next)}`);
}

function authHeaders(headers = {}) {
  const auth = AuthStore.get();
  return auth ? { ...headers, Authorization: `Bearer ${auth.accessToken}` } : headers;
}

class ApiError extends Error {
  constructor(status, code, message) {
    super(message);
    this.status = status;
    this.code = code;
  }
}

let refreshInFlight = null;

function refreshSession(staleToken = null) {
  if (!refreshInFlight) {
    const run = async () => {
      const current = AuthStore.get();
      if (current && current.accessToken !== staleToken) return true;

      try {
        const res = await fetch(`${API_BASE}/auth/refresh`, { method: 'POST' });
        if (!res.ok) {
          if (res.status === 401) AuthStore.clear();
          return false;
        }
        const token = await res.json();
        const saved = AuthStore.peek();
        AuthStore.save(token, saved ? saved.email : '');
        return true;
      } catch {
        return false;
      }
    };

    const locked = navigator.locks
      ? navigator.locks.request('shop-auth-refresh', run)
      : run();
    refreshInFlight = locked.finally(() => { refreshInFlight = null; });
  }
  return refreshInFlight;
}

async function rawFetch(path, options = {}, isRetry = false) {
  const usedToken = AuthStore.get();
  let res;
  try {
    res = await fetch(API_BASE + path, { ...options, headers: authHeaders(options.headers) });
  } catch (err) {
    throw new ApiError(0, 'NETWORK_ERROR', 'Не удалось связаться с сервером');
  }

  if (res.status === 401 && !path.startsWith('/auth/')) {
    if (!isRetry && await refreshSession(usedToken ? usedToken.accessToken : null)) {
      return rawFetch(path, options, true);
    }
    AuthStore.clear();
    redirectToLogin();
    throw new ApiError(401, 'UNAUTHORIZED', 'Требуется вход');
  }

  return res;
}

async function apiFetch(path, options = {}) {
  const res = await rawFetch(path, options);

  if (res.status === 204) return null;

  const text = await res.text();
  let body = null;
  if (text) {
    try { body = JSON.parse(text); } catch { body = null; }
  }

  if (!res.ok) {
    const code = body && body.code ? body.code : 'UNKNOWN_ERROR';
    const message = body && body.message ? body.message : `Ошибка запроса (${res.status})`;
    throw new ApiError(res.status, code, message);
  }

  return body;
}

async function loadApiImages(root = document) {
  const images = root.querySelectorAll('img[data-api-src]');
  await Promise.all([...images].map(async (img) => {
    const path = img.dataset.apiSrc;
    img.removeAttribute('data-api-src');
    try {
      const res = await rawFetch(path);
      if (!res.ok) return;
      img.src = URL.createObjectURL(await res.blob());
    } catch {}
  }));
}

const Api = {
  register(data) {
    return apiFetch('/auth/register', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
  },
  login(email, password) {
    return apiFetch('/auth/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email, password }),
    });
  },
  changePassword(oldPassword, newPassword) {
    return apiFetch('/auth/change-password', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ old_password: oldPassword, new_password: newPassword }),
    });
  },
  logout() {
    return apiFetch('/auth/logout', { method: 'POST' });
  },
  logoutAll() {
    return apiFetch('/auth/logout-all', { method: 'POST' });
  },
  resetPassword(email) {
    return apiFetch('/auth/reset-password', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ email }),
    });
  },
  oauthStart(provider) {
    return apiFetch(`/auth/oauth/${encodeURIComponent(provider)}/start`, { method: 'POST' });
  },
  oauthCallback(code, state) {
    return apiFetch('/auth/oauth/callback', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ code, state }),
    });
  },

  listProducts() {
    return apiFetch('/products');
  },
  getProduct(id) {
    return apiFetch(`/products/${id}`);
  },
  createProduct(data) {
    return apiFetch('/products', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
  },
  deleteProduct(id) {
    return apiFetch(`/products/${id}`, { method: 'DELETE' });
  },
  decreaseStock(id, decreaseBy) {
    return apiFetch(`/products/${id}/stock`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ decrease_by: decreaseBy }),
    });
  },
  uploadProductImage(id, file) {
    return apiFetch(`/products/${id}/image`, {
      method: 'POST',
      headers: { 'Content-Type': file.type || 'application/octet-stream' },
      body: file,
    });
  },

  listSuppliers() {
    return apiFetch('/suppliers');
  },
  getSupplier(id) {
    return apiFetch(`/suppliers/${id}`);
  },
  createSupplier(data) {
    return apiFetch('/suppliers', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
  },
  deleteSupplier(id) {
    return apiFetch(`/suppliers/${id}`, { method: 'DELETE' });
  },
  updateSupplierAddress(id, address) {
    return apiFetch(`/suppliers/${id}/address`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(address),
    });
  },

  listClients(limit, offset) {
    const params = new URLSearchParams();
    if (limit != null) params.set('limit', limit);
    if (offset != null) params.set('offset', offset);
    const qs = params.toString();
    return apiFetch(`/clients${qs ? `?${qs}` : ''}`);
  },
  searchClients(name, surname) {
    const params = new URLSearchParams({ client_name: name, client_surname: surname });
    return apiFetch(`/clients/search?${params.toString()}`);
  },
  createClient(data) {
    return apiFetch('/clients', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
    });
  },
  deleteClient(id) {
    return apiFetch(`/clients/${id}`, { method: 'DELETE' });
  },
  updateClientAddress(id, address) {
    return apiFetch(`/clients/${id}/address`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(address),
    });
  },
};
