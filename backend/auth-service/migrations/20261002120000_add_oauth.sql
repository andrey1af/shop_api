-- +goose Up
-- +goose StatementBegin
-- Пользователи, пришедшие через OAuth, могут не иметь пароля и телефона.
ALTER TABLE users ALTER COLUMN phone_number DROP NOT NULL;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE users ALTER COLUMN password_hash DROP NOT NULL;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE user_identities
(
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID         NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    provider         VARCHAR(32)  NOT NULL,
    provider_user_id VARCHAR(255) NOT NULL,
    email            VARCHAR(255) NOT NULL,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now(),
    UNIQUE (provider, provider_user_id),
    UNIQUE (user_id, provider)
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TABLE oauth_states
(
    state         VARCHAR(128) PRIMARY KEY,
    provider      VARCHAR(32)  NOT NULL,
    code_verifier VARCHAR(128) NOT NULL,
    expires_at    TIMESTAMPTZ  NOT NULL
);
-- +goose StatementEnd

-- +goose StatementBegin
CREATE INDEX idx_oauth_states_expires_at ON oauth_states (expires_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE oauth_states;
-- +goose StatementEnd

-- +goose StatementBegin
DROP TABLE user_identities;
-- +goose StatementEnd

-- Откат не пройдёт, пока в users есть записи без пароля или телефона.
-- +goose StatementBegin
ALTER TABLE users ALTER COLUMN password_hash SET NOT NULL;
-- +goose StatementEnd

-- +goose StatementBegin
ALTER TABLE users ALTER COLUMN phone_number SET NOT NULL;
-- +goose StatementEnd
