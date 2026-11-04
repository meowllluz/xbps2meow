# xbps2meow

Converts a **Void Linux package database** (`/var/db/xbps`) into the format used by
[meow](https://github.com/meowllluz/meowpm), the package manager of Meow Linux.
It lets you switch an already installed Void system from xbps to meow without
reinstalling anything.

Русская версия: [README-ru.md](README-ru.md)

## What it does

Reads `/var/db/xbps/pkgdb-0.38.plist`, and for every installed package writes:

- `/var/db/meow/<pkg>.json` — name, version, architecture, description,
  `run_depends`, `provides`, `conflicts`, file list with sha256 and the
  `automatic-install` flag;
- `/var/db/meow/installed.list` — the `name|version|date|auto` index.

File lists are taken from the per-package `.pkgname-files.plist` files next to the
database, where the path lives in the `file` key (normalised to `/usr/...` with a
leading slash). Packages that are already present in meow's `installed.list` are
skipped, so the tool is safe to run twice.

The `automatic-install` flag is carried over into `Auto`; anything that was
installed manually stays `Auto=false`, so `meow remove -R` will not cascade-delete
system packages such as glibc or coreutils.

```
# xbps2meow
pkgdb: 214 packages
converted: 214, skipped: 0, failed: 0
```

## Usage

Run as root, on the Void system you want to migrate:

```sh
go build -o xbps2meow .
sudo ./xbps2meow

# now install meow itself, see github.com/meowllluz/meowpm
sudo meow sync
sudo meow list | head

# and only then remove xbps
sudo meow remove -y xbps
```

`meow list`, `meow info` and `meow upgrade -n` should agree with what xbps reported
before the migration — check that before removing xbps.

Notes:

- no arguments, no flags: it always reads `/var/db/xbps` and writes `/var/db/meow`;
- it never touches the file system outside those two directories and never runs
  package scripts — only metadata is converted;
- the old xbps database stays where it is, so a rollback is possible until you
  delete `/var/db/xbps` yourself.

## Building

```sh
git clone https://github.com/meowllluz/xbps2meow.git
cd xbps2meow
go build -o xbps2meow .
```

Requires Go 1.27+; the only dependency is `howett.net/plist`.

## License

GNU GPL v2 — see [LICENSE](LICENSE).
