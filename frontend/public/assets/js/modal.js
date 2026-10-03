
function openOverlay(innerHtml, { onMount } = {}) {
  const overlay = el(`<div class="overlay"></div>`);
  overlay.innerHTML = innerHtml;
  document.body.appendChild(overlay);

  const close = () => overlay.remove();
  overlay.addEventListener('mousedown', (e) => {
    if (e.target === overlay) close();
  });
  document.addEventListener('keydown', function onKey(e) {
    if (e.key === 'Escape') {
      close();
      document.removeEventListener('keydown', onKey);
    }
  });

  if (onMount) onMount(overlay, close);
  return { overlay, close };
}

function confirmAction({ title, text, confirmLabel = 'Удалить', cancelLabel = 'Отмена', danger = true }) {
  return new Promise((resolve) => {
    const { close } = openOverlay(`
      <div class="modal">
        <div>
          <div class="modal-title">${escapeHtml(title)}</div>
          ${text ? `<div class="modal-subtitle" style="margin-top:6px">${escapeHtml(text)}</div>` : ''}
        </div>
        <div class="modal-actions">
          <button type="button" class="btn btn-secondary" data-role="cancel">${escapeHtml(cancelLabel)}</button>
          <button type="button" class="btn ${danger ? 'btn-primary' : 'btn-primary'}" data-role="confirm">${escapeHtml(confirmLabel)}</button>
        </div>
      </div>
    `, {
      onMount(overlay, closeOverlay) {
        overlay.querySelector('[data-role="cancel"]').addEventListener('click', () => {
          closeOverlay();
          resolve(false);
        });
        overlay.querySelector('[data-role="confirm"]').addEventListener('click', () => {
          closeOverlay();
          resolve(true);
        });
      },
    });
  });
}
