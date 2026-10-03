# Схемы баз данных

У каждого сервиса своя база Postgres; связей между базами нет. Схемы
меняются только миграциями goose (`backend/<сервис>/migrations`), а
применяет их одноразовый сервис `migrate` / `auth-migrate` / `image-migrate`
при `make up`.

Общая архитектура — в [architecture.md](architecture.md). Диаграммы рисуются
в PlantUML (`docs/diagrams/*.puml`) и пересобираются командой `make diagrams`.

## api-service — `shop_api`

![Схема базы shop_api](images/db-api-service.png)

Исходник — [diagrams/db-api-service.puml](diagrams/db-api-service.puml).

| Таблица | Назначение и особенности |
|---|---|
| `addresses` | адреса клиентов и поставщиков; у каждого клиента и поставщика свой адрес (`address_id` уникален) и удаляется вместе с ним |
| `clients` | покупатели; индекс `(client_name, client_surname)` для поиска |
| `suppliers` | поставщики; поставщика нельзя удалить, пока у него есть товары |
| `products` | каталог; частичный индекс по `available_stock > 0` для списка товаров в наличии, индекс по `supplier_id`. Цену и остаток меняют покупки и события из Kafka. `image_id` — id фото в image-service, без внешнего ключа: фото лежит в другой базе |

## image-service — `shop_images_1` … `shop_images_4`

![Схема баз image-service](images/db-image-service.png)

Исходник — [diagrams/db-image-service.puml](diagrams/db-image-service.puml).

Четыре базы-шарда с одинаковой схемой; пока все они на одном сервере
Postgres `image-db` и создаются скриптом `backend/image-service/initdb`
при первом запуске контейнера. Шард выбирается по первому символу id фото
(`0-3`, `4-7`, `8-b`, `c-f`).

| Таблица | Назначение и особенности |
|---|---|
| `images` | фото товаров: содержимое (`bytea`) и `content_type`. Связи с товаром здесь нет — она в `shop_api.products.image_id` |

## auth-service — `shop_auth`

![Схема базы shop_auth](images/db-auth-service.png)

Исходник — [diagrams/db-auth-service.puml](diagrams/db-auth-service.puml).

| Таблица | Назначение и особенности |
|---|---|
| `users` | учётные записи; `email` уникален. У пользователей, пришедших через OAuth, может не быть пароля и телефона |
| `user_identities` | привязки к внешним OAuth-провайдерам; уникальны пары `(provider, provider_user_id)` и `(user_id, provider)` |
| `oauth_states` | одноразовые `state` и PKCE `code_verifier` незавершённых OAuth-входов; удаляются при использовании и по истечении (индекс по `expires_at`) |
| `sessions` | входы пользователя (устройства). Сессия активна, пока `revoked_at IS NULL` и `expires_at > now()`; это проверяется при каждой проверке access-токена. Индексы по `user_id` (выход со всех устройств) и `expires_at` (очистка) |
| `refresh_tokens` | одноразовые refresh-токены сессии, хранится только SHA-256. Обмен в одной транзакции помечает старый токен `used_at`, сохраняет новый и продлевает сессию |

### Жизненный цикл сессии

Очистка (раз в `SESSION_CLEANUP_INTERVAL`, по умолчанию 1 ч) удаляет
сессии с истёкшим `expires_at` вместе с их refresh-токенами, а также
refresh-токены, истёкшие или использованные больше 7 дней назад.
