-- Базы шардов image-service. Пока все четыре живут на одном сервере Postgres;
-- при переезде шарда на отдельную машину меняется только его строка подключения.
-- Скрипт выполняется entrypoint-ом образа postgres один раз, при создании тома.
CREATE DATABASE shop_images_1;
CREATE DATABASE shop_images_2;
CREATE DATABASE shop_images_3;
CREATE DATABASE shop_images_4;
