-- +goose Up
-- +goose StatementBegin
ALTER TABLE clients DROP CONSTRAINT clients_gender_check;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
ALTER TABLE clients ADD CONSTRAINT clients_gender_check CHECK (gender IN ('male', 'female', 'other'));
-- +goose StatementEnd
