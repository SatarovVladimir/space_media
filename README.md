# Орбита — космическая медиаплатформа

Учебный проект: веб-платформа, где пользователи публикуют статьи о космосе,
читают публикации друг друга и взаимодействуют через комментарии, реакции и
подписки на авторов. Backend написан на Go, данные хранятся в PostgreSQL,
обработка заказов демонстрирует RabbitMQ, а состояние HTTP API наблюдается
через Prometheus и Grafana.

## Что умеет сайт

- Публичная лента статей и поиск по заголовку, краткому описанию и тексту.
- Регистрация и вход с JWT.
- Профиль автора: имя, описание, аватар, число опубликованных статей и
  подписчиков.
- Публикация статей, просмотр статьи и комментариев.
- Лайк или дизлайк статьи. У пользователя может быть одна переключаемая реакция
  на статью.
- Подписка на авторов и отдельная лента **Мои авторы**.
- Первые три статьи публикуются бесплатно. Затем можно заказать учебный план
  публикаций на месяц или год. Заказ проходит через RabbitMQ; настоящей оплаты
  нет.
- Аватары JPG, PNG или WebP размером до 2 МиБ хранятся в Docker volume.


## Структура проекта

```text
my-awesome-project/
├── cmd/
│   ├── api/main.go                 # Запуск HTTP API
│   └── order-worker/
│       ├── main.go                 # Запуск обработчика заказов
│       └── worker.go
├── frontend/
│   ├── Dockerfile
│   ├── nginx.conf                  # Сайт и прокси API
│   └── public/
│       ├── app.js
│       ├── index.html
│       └── styles.css
├── internal/
│   ├── broker/                     # RabbitMQ
│   ├── config/                     # Настройки окружения
│   ├── controllers/                # HTTP-обработчики
│   ├── database/                   # Соединение с PostgreSQL
│   ├── dto/                        # Форматы запросов и ответов
│   ├── metrics/                    # Метрики Prometheus
│   ├── middleware/                 # Проверка JWT
│   ├── migrations/                 # SQL-миграции
│   ├── models/                     # Модели
│   ├── repositories/               # Запросы к PostgreSQL
│   ├── routes/                     # Маршруты HTTP
│   └── services/                   # Бизнес-логика
├── monitoring/
│   ├── grafana/                    # Дашборд и provisioning Grafana
│   └── prometheus/                 # Конфигурация сбора метрик
├── .dockerignore
├── .gitignore
├── docker-compose.yml
├── Dockerfile                      # Образ API и worker
├── go.mod
├── go.sum
└── README.md
```

## Запуск проекта

### Требования

- Docker Desktop с поддержкой `docker compose`
- Терминал, например терминал VS Code

### 1. Настрой окружение

В корне проекта создай файл `.env`, если его ещё нет. **Не перезаписывай уже
существующий `.env`** — в нём находятся локальные настройки. В файле должны
быть заданы переменные:

```dotenv
DB_PASSWORD=задай_пароль_для_postgres
JWT_SECRET=вставь_сгенерированный_секрет
RABBITMQ_PASSWORD=задай_пароль_rabbitmq (quest)
GRAFANA_PASSWORD=задай_пароль_grafana
```

Секрет JWT можно сгенерировать командой:

```bash
openssl rand -hex 32
```

Для `RABBITMQ_PASSWORD` используй пароль без символов, которые требуют
URL-кодирования, например `@`, `/`, `:` и `#`. Файл `.env` не публикуй.

### 2. Собери и запусти сервисы

Убедись, что Docker Desktop запущен, затем из корня проекта выполни:

```bash
docker compose up --build -d
docker compose ps
```

В колонке состояния должны запуститься `postgres`, `rabbitmq`, `api`,
`order-worker`, `frontend`, `prometheus` и `grafana`. Логи приложений:

```bash
docker compose logs -f api order-worker
```

Остановить сервисы, сохранив данные:

```bash
docker compose down
```

`docker compose down -v` удаляет также volumes, включая данные PostgreSQL и
загруженные аватары. Используй эту команду только если действительно хочешь
удалить данные.

### 3. Открой приложения

| Приложение / сервис | Адрес | Для чего |
|---|---|---|
| Сайт | [http://localhost:8081](http://localhost:8081) | Лента и профили |
| Go API | [http://localhost:8080](http://localhost:8080) | HTTP API |
| API health check | [http://localhost:8080/test](http://localhost:8080/test) | Проверка API, ответ `Hello!` |
| Метрики API | [http://localhost:8080/metrics](http://localhost:8080/metrics) | Текстовые метрики Prometheus |
| RabbitMQ Management | [http://localhost:15673](http://localhost:15673) | Очереди и сообщения |
| Prometheus | [http://localhost:9090](http://localhost:9090) | Проверка и запрос метрик |
| Grafana | [http://localhost:3000](http://localhost:3000) | Дашборд API |
| PostgreSQL для pgAdmin | `localhost:5433` | Подключение к базе с компьютера |

Для Grafana логин — `admin`, пароль — значение `GRAFANA_PASSWORD` из `.env`.
В pgAdmin укажи host `localhost`, port `5433`, database `space_media`,
username `space_media_user` и пароль `DB_PASSWORD`. RabbitMQ логин по умолчанию
`guest`; пароль — `RABBITMQ_PASSWORD`.

В Docker Compose приложения общаются друг с другом по внутренним портам:
PostgreSQL `5432`, RabbitMQ AMQP `5672`, RabbitMQ Management `15672`. Снаружи
они опубликованы соответственно как `5433`, `5673` и `15673`.

### 4. Как проверить RabbitMQ и мониторинг

RabbitMQ показывает две очереди:

- `orders.created` — API отправляет новый заказ, worker получает его и после
  обработки подтверждает.
- `orders.status` — worker сообщает `processing` и `completed`, API записывает
  эти статусы в PostgreSQL. Поэтому очередь часто пуста: API быстро забирает
  сообщения.

Worker имитирует обработку 15 секунд. Чтобы прочитать JSON нового заказа в
RabbitMQ Management, останови worker до отправки заказа:

```bash
docker compose stop order-worker
```

Создай подписку на сайте, открой **Queues and Streams → orders.created**,
нажми **Get messages** и выбери **Nack message requeue true**, чтобы сообщение
вернулось в очередь. После просмотра запусти worker:

```bash
docker compose start order-worker
```

Чтобы проверить мониторинг, открой сайт или сделай несколько запросов к API.
В Prometheus выполни запрос `up`: цель API должна иметь значение `1`. В Grafana
открой папку **Space Media** и дашборд **Space Media API**.

## Проверки через терминал

Команды ниже предполагают, что Compose уже запущен и доступен API на
`http://localhost:8080`. Для запросов с авторизацией сначала скопируй JWT из
ответа входа и задай его в текущем терминале:

```bash
export TOKEN='вставь_JWT_из_ответа_входа'
```

### Проверка API и записи в базу

```bash
curl -i http://localhost:8080/test
```

Ожидается `200 OK` и `Hello!`.

```bash
curl -i -X POST http://localhost:8080/dbtest \
  -H 'Content-Type: text/plain' \
  --data 'Проверка записи PostgreSQL'
```

Ожидается `201 Created`. Проверить запись в pgAdmin:

```sql
SELECT id, message, created_at
FROM db_test_messages
ORDER BY id DESC
LIMIT 10;
```

### Регистрация, вход и пользователь

Регистрация выполняется один раз для каждого email:

```bash
curl -i -X POST http://localhost:8080/auth/register \
  -H 'Content-Type: application/json' \
  --data '{"email":"pilot@example.com","password":"space-pass-123"}'
```

Вход и получение JWT:

```bash
curl -i -X POST http://localhost:8080/auth/login \
  -H 'Content-Type: application/json' \
  --data '{"email":"pilot@example.com","password":"space-pass-123"}'
```

Скопируй значение `token` из JSON-ответа в переменную `TOKEN`.

### Профиль и статьи

Получить свой профиль и обновить его:

```bash
curl -i http://localhost:8080/profile/me \
  -H "Authorization: Bearer $TOKEN"

curl -i -X PUT http://localhost:8080/profile/me \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  --data '{"display_name":"Космический исследователь","bio":"Пишу о планетах и космосе."}'
```

Получить публичную ленту, найти статьи по слову и создать статью:

```bash
curl -i 'http://localhost:8080/articles'
curl -i 'http://localhost:8080/articles?q=Марс'

curl -i -X POST http://localhost:8080/articles \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  --data '{"title":"Миссия к Марсу","summary":"Краткий обзор экспедиции","body":"В этой статье мы рассмотрим подготовку будущей миссии к Марсу."}'
```

Первые три статьи бесплатны. После лимита ожидается `402 Payment Required`;
затем оформи план в следующем разделе. Для просмотра конкретной статьи
подставь её ID:

```bash
curl -i http://localhost:8080/articles/1
curl -i http://localhost:8080/profiles/1
```

Загрузить аватар можно командой, заменив путь на существующий файл JPG, PNG
или WebP:

```bash
curl -i -X POST http://localhost:8080/profile/me/avatar \
  -H "Authorization: Bearer $TOKEN" \
  -F 'avatar=@/полный/путь/к/аватару.jpg'
```

### Комментарии, реакции и авторы

```bash
curl -i http://localhost:8080/articles/1/comments

curl -i -X POST http://localhost:8080/articles/1/comments \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  --data '{"body":"Интересная статья!"}'

curl -i -X POST http://localhost:8080/articles/1/reaction \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  --data '{"reaction":"like"}'

curl -i http://localhost:8080/follows \
  -H "Authorization: Bearer $TOKEN"

curl -i -X POST http://localhost:8080/follows/2 \
  -H "Authorization: Bearer $TOKEN"

curl -i -X DELETE http://localhost:8080/follows/2 \
  -H "Authorization: Bearer $TOKEN"

curl -i 'http://localhost:8080/articles?followed=true' \
  -H "Authorization: Bearer $TOKEN"
```

Замени `1` и `2` на существующие ID статьи и автора. Для снятия реакции повтори
запрос с тем же типом реакции.

### Заказы и подписка на публикации

Создать обычный заказ и посмотреть только свои заказы:

```bash
curl -i -X POST http://localhost:8080/orders \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  --data '{"description":"Тестовый заказ"}'

curl -i http://localhost:8080/orders \
  -H "Authorization: Bearer $TOKEN"
```

Посмотреть статус плана, заказать месяц или год:

```bash
curl -i http://localhost:8080/publication-plan \
  -H "Authorization: Bearer $TOKEN"

curl -i -X POST http://localhost:8080/publication-plan \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  --data '{"duration_months":1}'
```

Для годовой подписки укажи `{"duration_months":12}`. Ответ заказа первоначально
имеет статус `pending`; worker публикует `processing`, ждёт 15 секунд, затем
отправляет `completed`. После этого повтори `GET /publication-plan` и попробуй
опубликовать статью ещё раз.

Посмотреть записи в PostgreSQL можно через Query Tool в pgAdmin:

```sql
SELECT id, email, created_at
FROM users
ORDER BY id DESC;

SELECT id, user_id, description, order_type, plan_duration_months,
       status, created_at, updated_at, subscription_expires_at
FROM orders
ORDER BY id DESC
LIMIT 20;
```

Для проверки изоляции зарегистрируй второй email, войди под ним, скопируй его
JWT в `TOKEN` и вызови `GET /orders`: он должен вернуть только заказы второго
пользователя. Повторный вход под первым пользователем покажет его собственный
список.

## API: методы, адреса и данные для Swagger

Swagger UI и OpenAPI-файл в проект пока не добавлены. Таблица ниже описывает
фактические маршруты; её можно использовать как основу для OpenAPI/Swagger.
Base URL API: `http://localhost:8080`. JSON-запросы требуют
`Content-Type: application/json`. Для защищённых методов нужен заголовок
`Authorization: Bearer <JWT>`.

| Метод | Путь | Авторизация | Параметры / тело |
|---|---|---|---|
| GET | `/test` | Нет | Нет |
| POST | `/dbtest` | Нет | Текстовое тело, не пустое, до 1 MiB |
| POST | `/auth/register` | Нет | `{"email":"pilot@example.com","password":"space-pass-123"}` |
| POST | `/auth/login` | Нет | `{"email":"pilot@example.com","password":"space-pass-123"}` |
| GET | `/orders` | JWT | Нет; заказы только текущего пользователя |
| POST | `/orders` | JWT | `{"description":"Тестовый заказ"}` |
| GET | `/articles` | Необязательно | Query: `q`, `author_id`; JWT необязателен |
| GET | `/articles?followed=true` | JWT | Только статьи авторов, на которых подписан пользователь |
| POST | `/articles` | JWT | `{"title":"Миссия к Марсу","summary":"Кратко","body":"Текст статьи не короче 20 байт."}` |
| GET | `/articles/{id}` | Необязательно | ID статьи в пути |
| GET | `/articles/{id}/comments` | Нет | ID статьи в пути |
| POST | `/articles/{id}/comments` | JWT | `{"body":"Текст комментария"}` |
| POST | `/articles/{id}/reaction` | JWT | `{"reaction":"like"}` или `{"reaction":"dislike"}` |
| GET | `/profiles/{id}` | Нет | ID автора в пути |
| GET | `/profile/me` | JWT | Нет |
| PUT | `/profile/me` | JWT | `{"display_name":"Имя","bio":"Описание"}` |
| POST | `/profile/me/avatar` | JWT | multipart/form-data поле `avatar`, JPG/PNG/WebP до 2 МиБ |
| GET | `/follows` | JWT | Список авторов, на которых подписан пользователь |
| POST | `/follows/{author_id}` | JWT | ID автора в пути; тело отсутствует |
| DELETE | `/follows/{author_id}` | JWT | ID автора в пути; тело отсутствует |
| GET | `/publication-plan` | JWT | Нет |
| POST | `/publication-plan` | JWT | `{"duration_months":1}` или `{"duration_months":12}` |
| GET | `/metrics` | Нет | Метрики Prometheus; endpoint доступен на API |

Успешные ответы обычно имеют статус `200`, `201`, `202` или `204`; ошибки
валидации — `400`, отсутствие JWT — `401`, а исчерпанный бесплатный лимит
статей — `402`. Swagger UI появится после добавления OpenAPI-спецификации и
подключения Swagger к проекту.

## Лабораторные работы

- **ЛР1:** HTTP-сервер, `GET /test`, прохождение через контроллер, сервис и
  репозиторий, graceful shutdown.
- **ЛР2:** PostgreSQL, автоматические SQL-миграции и `POST /dbtest`.
- **ЛР3:** регистрация, вход, bcrypt-хеширование пароля и JWT.
- **ЛР4:** создание заказа и получение списка заказов пользователя.
- **ЛР5:** JWT middleware передаёт ID пользователя из токена в контекст запроса;
  пользователь видит только свои заказы.
- **RabbitMQ:** отдельный worker имитирует обработку заказов и публикует
  изменения статуса.
- **Мониторинг:** Prometheus собирает HTTP-метрики, Grafana отображает дашборд.

## Проверки кода

Из корня Go-модуля:

```bash
go test ./...
go vet ./...
go build ./...
```

Автоматические тестовые файлы пока не добавлены, поэтому `go test` сейчас
проверяет сборку пакетов и сообщает `[no test files]`.
