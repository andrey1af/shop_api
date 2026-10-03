-- +goose Up
-- +goose StatementBegin
ALTER TABLE images DROP COLUMN data;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE images DROP COLUMN content_type;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE images ADD COLUMN content_type TEXT NOT NULL DEFAULT 'application/octet-stream';
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE images ADD COLUMN data BYTEA NOT NULL DEFAULT '';
-- +goose StatementEnd
