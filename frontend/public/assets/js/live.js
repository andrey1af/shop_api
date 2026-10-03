const LIVE_RETRY_MS = 5000;

const ProductLive = {
  subscribe(onUpdate, { onStatus = () => {} } = {}) {
    let source = null;
    let retryTimer = null;

    async function connect(staleToken = null) {
      if (!AuthStore.peek()) {
        onStatus('offline');
        return;
      }
      onStatus('connecting');
      if (!AuthStore.get() || staleToken) await refreshSession(staleToken);

      const auth = AuthStore.get();
      if (!auth) {
        onStatus('offline');
        if (AuthStore.peek()) retryTimer = setTimeout(() => connect(staleToken), LIVE_RETRY_MS);
        return;
      }
      const usedToken = auth.accessToken;

      source = new EventSource(`${API_BASE}/sse/products?access_token=${encodeURIComponent(auth.accessToken)}`);

      source.addEventListener('open', () => onStatus('live'));

      source.addEventListener('product.updated', (event) => {
        let product;
        try { product = JSON.parse(event.data); } catch { return; }
        onUpdate(product);
      });

      source.addEventListener('error', () => {
        if (source.readyState !== EventSource.CLOSED) {
          onStatus('connecting');
          return;
        }
        onStatus('offline');
        source = null;
        retryTimer = setTimeout(() => connect(usedToken), LIVE_RETRY_MS);
      });
    }

    window.addEventListener('pagehide', () => {
      clearTimeout(retryTimer);
      if (source) source.close();
    });

    connect();
  },
};

function flashChange(element, direction) {
  if (!element || !direction) return;
  const cls = direction > 0 ? 'live-up' : 'live-down';
  element.classList.remove('live-up', 'live-down');
  void element.offsetWidth;
  element.classList.add(cls);
}
