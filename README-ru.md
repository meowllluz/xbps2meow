# xbps2meow

Переносит **базу пакетов Void Linux** (`/var/db/xbps`) в формат
[meow](https://github.com/meowllluz/meowpm) — пакетного менеджера Meow Linux.
Позволяет перевести уже установленную систему с xbps на meow, ничего не
переставляя.

English: [README.md](README.md)

## Что делает

Читает `/var/db/xbps/pkgdb-0.38.plist` и для каждого установленного пакета пишет:

- `/var/db/meow/<pkg>.json` — имя, версия, архитектура, описание, `run_depends`,
  `provides`, `conflicts`, список файлов с sha256 и флаг `automatic-install`;
- `/var/db/meow/installed.list` — индекс `имя|версия|дата|auto`.

Списки файлов берутся из соседних с базой `.pkgname-files.plist`, где путь лежит
в ключе `file` (нормализуется к `/usr/...` с ведущим слэшем). Пакеты, которые уже
есть в `installed.list` у meow, пропускаются — инструмент можно запускать повторно.

Флаг `automatic-install` переносится в `Auto`; всё, что ставилось вручную,
остаётся с `Auto=false`, поэтому `meow remove -R` не вычистит системные пакеты
вроде glibc или coreutils.

```
# xbps2meow
pkgdb: 214 packages
converted: 214, skipped: 0, failed: 0
```

## Как пользоваться

Запускать от root на той системе Void, которую переносим:

```sh
go build -o xbps2meow .
sudo ./xbps2meow

# теперь ставим сам meow, см. github.com/meowllluz/meowpm
sudo meow sync
sudo meow list | head

# и только потом удаляем xbps
sudo meow remove -y xbps
```

`meow list`, `meow info` и `meow upgrade -n` должны показывать то же, что показывал
xbps до переноса — проверь это до удаления xbps.

Замечания:

- аргументов и флагов нет: всегда читает `/var/db/xbps`, пишет в `/var/db/meow`;
- за пределы этих двух каталогов не выходит и скрипты пакетов не запускает —
  переносится только метаинформация;
- старая база xbps остаётся на месте, так что откатиться можно, пока не удалишь
  `/var/db/xbps` сам.

## Сборка

```sh
git clone https://github.com/meowllluz/xbps2meow.git
cd xbps2meow
go build -o xbps2meow .
```

Нужен Go 1.27+, единственная зависимость — `howett.net/plist`.

## Лицензия

GNU GPL v2 — см. [LICENSE](LICENSE).
