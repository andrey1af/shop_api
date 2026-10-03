(function () {
  const form = document.getElementById('new-product-form');
  const nameInput = document.getElementById('f-name');
  const categoryInput = document.getElementById('f-category');
  const categoryOptions = document.getElementById('category-options');
  const priceInput = document.getElementById('f-price');
  const stockInput = document.getElementById('f-stock');
  const supplierSelect = document.getElementById('f-supplier');
  const imageInput = document.getElementById('f-image');
  const dropzone = document.getElementById('dropzone');
  const filePreview = document.getElementById('file-preview');
  const filePreviewImg = document.getElementById('file-preview-img');
  const filePreviewName = document.getElementById('file-preview-name');
  const filePreviewSize = document.getElementById('file-preview-size');
  const filePreviewRemove = document.getElementById('file-preview-remove');
  const formError = document.getElementById('form-error');
  const submitBtn = document.getElementById('submit-btn');

  const errorEls = {
    name: document.getElementById('err-name'),
    price: document.getElementById('err-price'),
    stock: document.getElementById('err-stock'),
    supplier: document.getElementById('err-supplier'),
  };

  let selectedFile = null;

  function clearFieldErrors() {
    Object.values(errorEls).forEach((e) => { e.hidden = true; e.textContent = ''; });
    [nameInput, priceInput, stockInput, supplierSelect].forEach((i) => i.classList.remove('error'));
    formError.hidden = true;
  }

  function setFieldError(input, errEl, message) {
    input.classList.add('error');
    errEl.textContent = message;
    errEl.hidden = false;
  }

  async function loadSuppliers() {
    try {
      const suppliers = await Api.listSuppliers();
      supplierSelect.innerHTML = suppliers
        .map((s) => `<option value="${s.id}">${escapeHtml(s.name)} — ${escapeHtml(s.address.city)}</option>`)
        .join('');
      if (suppliers.length === 0) {
        supplierSelect.innerHTML = '<option value="">Нет доступных поставщиков</option>';
      }
    } catch (err) {
      supplierSelect.innerHTML = '<option value="">Не удалось загрузить поставщиков</option>';
    }
  }

  async function loadCategories() {
    try {
      const products = await Api.listProducts();
      const categories = [...new Set(products.map((p) => p.category))].sort((a, b) => a.localeCompare(b, 'ru'));
      categoryOptions.innerHTML = categories.map((c) => `<option value="${escapeHtml(c)}"></option>`).join('');
    } catch (err) {
    }
  }

  function formatFileSize(bytes) {
    if (bytes < 1024) return `${bytes} Б`;
    if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} КБ`;
    return `${(bytes / (1024 * 1024)).toFixed(1)} МБ`;
  }

  function setSelectedFile(file) {
    selectedFile = file;
    if (!file) {
      filePreview.hidden = true;
      imageInput.value = '';
      return;
    }
    filePreviewName.textContent = file.name;
    filePreviewSize.textContent = `${formatFileSize(file.size)} · готов к загрузке`;
    filePreviewImg.src = URL.createObjectURL(file);
    filePreview.hidden = false;
  }

  imageInput.addEventListener('change', () => {
    setSelectedFile(imageInput.files[0] || null);
  });

  filePreviewRemove.addEventListener('click', () => setSelectedFile(null));

  ['dragover', 'dragenter'].forEach((evt) => {
    dropzone.addEventListener(evt, (e) => {
      e.preventDefault();
      dropzone.style.background = '#EEF0F5';
    });
  });
  ['dragleave', 'drop'].forEach((evt) => {
    dropzone.addEventListener(evt, (e) => {
      e.preventDefault();
      dropzone.style.background = '';
    });
  });
  dropzone.addEventListener('drop', (e) => {
    const file = e.dataTransfer.files[0];
    if (file) setSelectedFile(file);
  });

  function parsePrice(value) {
    const normalized = value.replace(/\s/g, '').replace(',', '.');
    const num = Number(normalized);
    return Number.isFinite(num) ? num : NaN;
  }

  form.addEventListener('submit', async (e) => {
    e.preventDefault();
    clearFieldErrors();

    const name = nameInput.value.trim();
    const category = categoryInput.value.trim();
    const price = parsePrice(priceInput.value);
    const stock = parseInt(stockInput.value, 10);
    const supplierId = supplierSelect.value;

    let hasError = false;
    if (!name) { setFieldError(nameInput, errorEls.name, 'Введите название товара'); hasError = true; }
    if (!Number.isFinite(price) || price <= 0) { setFieldError(priceInput, errorEls.price, 'Цена должна быть больше нуля'); hasError = true; }
    if (!Number.isInteger(stock) || stock < 0) { setFieldError(stockInput, errorEls.stock, 'Остаток не может быть отрицательным'); hasError = true; }
    if (!supplierId) { setFieldError(supplierSelect, errorEls.supplier, 'Выберите поставщика'); hasError = true; }
    if (!category) { categoryInput.classList.add('error'); hasError = true; }
    if (hasError) return;

    submitBtn.disabled = true;
    submitBtn.textContent = 'Создание…';
    try {
      const product = await Api.createProduct({
        name,
        category,
        price,
        available_stock: stock,
        supplier_id: supplierId,
      });

      if (selectedFile) {
        try {
          await Api.uploadProductImage(product.id, selectedFile);
        } catch (imgErr) {
          formError.textContent = `Товар создан, но изображение не загружено: ${friendlyErrorMessage(imgErr)}`;
          formError.hidden = false;
          setTimeout(() => { location.href = '/admin/products.html'; }, 1800);
          return;
        }
      }

      location.href = '/admin/products.html';
    } catch (err) {
      formError.textContent = friendlyErrorMessage(err);
      formError.hidden = false;
      submitBtn.disabled = false;
      submitBtn.textContent = 'Создать товар';
    }
  });

  loadSuppliers();
  loadCategories();
})();
