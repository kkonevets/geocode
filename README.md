# Geocoding
Конвертация координат в адреса и наоборот, кэширование запросов

## Прием данных - decoder
Разбирает очередь Кафки и декодирует координаты устройств в адреса с помощью алгоритма поиска

## Сохранение данных - decoder
1. Во время поиска адресов алгоритм поиска сохраняет их в таблицу _geocode_cache_
2. Адреса пишутся:
   1. в базу _x-company-devices_ таблицу _data_processed_ и обновляются поля:
	  * _update_dt_ - дата последней модификации таблицы, то есть текущее время сервера
	  * cледующие поля обновляются только если _address_dt_ из БД старее, чем _device_dt_ из задачи:
		  * _address_ - json строка (создать поле). Результат запроса нужен для нескольких языков
		  * _address_dt_ - это _device_dt_ последнего события, для которого произошло успешное обновление адреса (создать поле)
   2. в файловую базу данных xxdb - написать клиент go-xxdb на golang
3. Данные отправляются в модуль автоматизации

Пример поля _address_ таблицы _data_processed_:

```json
{
"en":"Петрохерсонецкий сельсовет, Grachyovsky District, Orenburg Oblast, Volga Federal District, Russia",
"ru":"Петрохерсонецкий сельсовет, Грачёвский район, Оренбургская область, Приволжский ФО, Россия",
"local":"Петрохерсонецкий сельсовет, Грачёвский район, Оренбургская область, Приволжский федеральный округ, Россия"
}
```

## Cache service (gRPC и http)
Данный сервис служит для кэширования результатов запросов, которые идут от decoder и от клиентов по gRPC.
Результаты запросов кэшируются в таблицу _geocode_cache_ и при повторном обращении берутся из нее же.
При запросе применяется [алгоритм поиска](#search-algorithm)

Типы запросов:
* По координате получить адрес
* По адресу получить координату (с помощью шэширования адресса)

В обоих случаях, если в запросе указан источник Source (Cache, Nominatim, Yandex, Google), то нужно искать только по этому источнику.

## Search algorithm
Ищем по списку пока не найдем, а если найдем, то пишем в _geocode_cache_ только для Яндекс и Google.

1. Таблица _geocode_cache_. Если найдено, но дата записи старая - ищу дальше по списку.
2. локальный Nominatim
3. Яндекс
4. Google
5. _default_: Если по всему списку не найдено, то пишем NULL в _geocode_cache_, считаем что он действует 24 часа, а потом по новому ищем по списку.

Сохранять в InfluxDB статистику по успешности запросов по всем источникам

Таблица geocode_cache:

| id | updated                       | source | address                        | address_hash        | lat    | lon    | lang  | encode |
|:--:|:-----------------------------:|:------:|:------------------------------:|:-------------------:|:------:|--------|-------|--------|
| 3  | 2021-02-22 19:46:41.370155+03 | yandex | Россия, Москва, Совeтская ул 4 | 3145177133033400300 | 547304 | 559946 | ru-RU | false  |

поле _encode_ нужно для отличия запросов на кодирование от декодирования координат, чтобы один не затирал другой

## Build
Установить [protoc](https://grpc.io/docs/protoc-installation/) и затем выполнить _make_, бинарники будут помещены в папку _build_

## Configure
Файл _config/config.yaml_ содержит конфигурационные параметры: подключения к базам данных, входные точки сервиса и т.д.

## Logging
_./build/decoder_ и _./build/cache_ принимают булевый параметр "-log", при котором логи пишутся в одноименные файлы в домашней дирректории, каждая строка файла есть json строка, что удобно для автоматического парсинга логов
