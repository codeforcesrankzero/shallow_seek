# ShallowSeek

Поисковая система для документов с поддержкой русского языка и синонимов.

## Основные возможности

- Поиск по текстовым документам (TXT, PDF, DOC, DOCX)
- Поддержка русского языка
- Поиск по синонимам
- Подсветка найденных фрагментов
- Кэширование результатов поиска

## Технологии

- Go
- Elasticsearch
- Docker
- Gin (веб-фреймворк)

## Установка и запуск

1. Клонируйте репозиторий:
```bash
git clone https://github.com/yourusername/shallowseek.git
cd shallowseek
```

2. Запустите Elasticsearch:
```bash
docker-compose up -d elasticsearch
```

3. Соберите и запустите приложение:
```bash
docker-compose up -d app
```

## Конфигурация

Основные настройки в файле `config/config.go`:
- URL Elasticsearch
- Настройки кэширования
- Параметры поиска

## Использование

### API Endpoints

- `GET /api/search?q=запрос` - поиск документов
- `GET /api/status` - статус системы
- `POST /api/upload` - загрузка документов
- `GET /api/documents/{id}/download` - скачивание документа
- `GET /api/documents/{id}/view` - просмотр документа

### Поиск

Поиск поддерживает:
- Точное совпадение фраз
- Поиск по синонимам
- Подсветку найденных фрагментов
- Фильтрацию по типу документа

## Разработка

### Структура проекта

- `handlers/` - обработчики HTTP запросов
- `elasticsearch/` - работа с Elasticsearch
- `models/` - модели данных
- `config/` - конфигурация
- `cache/` - кэширование
- `metrics/` - метрики
- `dict/` - словари синонимов