-- +goose Up
-- +goose StatementBegin
CREATE TABLE addresses
(
    id      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    country VARCHAR(100) NOT NULL,
    city    VARCHAR(100) NOT NULL,
    street  VARCHAR(255) NOT NULL
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE clients
(
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    client_name       VARCHAR(100) NOT NULL,
    client_surname    VARCHAR(100) NOT NULL,
    birthday          DATE         NOT NULL,
    gender            TEXT         NOT NULL CHECK (gender IN ('male', 'female', 'other')),
    registration_date TIMESTAMPTZ  NOT NULL DEFAULT now(),
    address_id        UUID         NOT NULL UNIQUE REFERENCES addresses (id)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_clients_name_surname ON clients (client_name, client_surname);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE suppliers
(
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name         VARCHAR(255) NOT NULL,
    phone_number VARCHAR(20)  NOT NULL,
    address_id   UUID         NOT NULL UNIQUE REFERENCES addresses (id)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE images
(
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    data         BYTEA       NOT NULL,
    content_type TEXT        NOT NULL DEFAULT 'application/octet-stream',
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE products
(
    id                UUID           PRIMARY KEY DEFAULT gen_random_uuid(),
    name              VARCHAR(255)   NOT NULL,
    category          VARCHAR(100)   NOT NULL,
    price             NUMERIC(12, 2) NOT NULL CHECK (price > 0),
    available_stock   BIGINT         NOT NULL CHECK (available_stock >= 0),
    last_update_date  TIMESTAMPTZ    NOT NULL DEFAULT now(),
    supplier_id       UUID           NOT NULL REFERENCES suppliers (id),
    image_id          UUID           UNIQUE REFERENCES images (id) ON DELETE SET NULL
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_products_supplier_id ON products (supplier_id);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_products_available_stock ON products (available_stock) WHERE available_stock > 0;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE products;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE images;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE suppliers;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE clients;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE addresses;
-- +goose StatementEnd
