# client-server-fasthttp-test

Тестовый проект с библиотекой HTTP-клиента на `fasthttp` для потоковой отправки файла через `POST multipart/form-data`.

## Что реализовано

- Клиентское приложение (конфиг + запуск): `internal/client`
- Серверное приложение (конфиг + обработка upload/healthz): `internal/server`
- Потоковая отправка файла из локальной ФС через `SetBodyStreamWriter`
- Настраиваемый размер блока (`ChunkSize`), по умолчанию `256` байт
- Параллельная пофайловая загрузка на клиенте (`max_concurrent_uploads`)

Ответ `/upload` возвращается в JSON и содержит размер, время обработки, скорость и checksum.
При включенном `pprof` доступны эндпоинты `http://<host>:6060/debug/pprof/...`.

## Запуск (Docker-only)

Полный e2e сценарий (генерация blob, запуск сервера, запуск клиента):

```bash
make docker-e2e
```

С настройкой размера blob:

```bash
make docker-e2e BLOB_SIZE_MB=128
```

Остановка и очистка контейнеров/volume:

```bash
make docker-down
```

## Конфигурация сервисов

Конфигурация читается через `viper` из переменных окружения и `.env`-файлов:

- сервер: `.env.server`
- клиент: `.env.client`

CLI-флаги для конфигурации не используются.

В Docker эти файлы копируются в образ (`Dockerfile`), поэтому `docker-compose.yml` не содержит `UPLOAD_*` конфиг прямо в сервисах.

Важно:

- используется единый формат ключей и в `.env`, и в окружении процесса:
- `UPLOAD_CLIENT_*` для клиента
- `UPLOAD_SERVER_*` для сервера

Ключевые переменные:

- `UPLOAD_CLIENT_URL` - URL upload-эндпоинта
- `UPLOAD_CLIENT_FILES` - список файлов через запятую
- `UPLOAD_CLIENT_CHUNK_SIZE` - размер чанка в байтах
- `UPLOAD_CLIENT_MAX_CONCURRENT_UPLOADS` - число параллельных загрузок
- `UPLOAD_SERVER_ADDR` - адрес сервера
- `UPLOAD_SERVER_MAX_CONCURRENT_UPLOADS` - лимит одновременных upload на сервере
- `UPLOAD_SERVER_PPROF_ENABLED` и `UPLOAD_SERVER_PPROF_ADDR` - pprof

Примеры конфигурации:

- `.env.client`
- `.env.server`

## Профилирование в Docker

Быстрый изолированный сценарий (рекомендуется):

```bash
make docker-e2e-profile BLOB_SIZE_MB=128
```

`docker-e2e-profile` автоматически:

- измеряет длительность передачи
- снимает CPU-профиль на примерно ту же длительность
- затем снимает heap-профиль

Ручной сценарий:

1. Поднять сервер:

```bash
make docker-server
```

2. Снять профили:

```bash
make pprof-cpu PPROF_SECONDS=30
make pprof-heap
```

3. Открыть профиль:

```bash
make pprof-ui-cpu
make pprof-ui-heap
```
