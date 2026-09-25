# Docu-UI

Web UI to edit Docker Compose env files and redeploy the affected services.

- Single image (Go backend + embedded web UI), runs behind any reverse proxy / gateway
- Username/password login with optional TOTP
- Apply adapters: Docker Compose, Doco-CD API, webhook/command
- Env files stay on the host — never committed anywhere

Status: early development.

## Development

```sh
make web     # build the UI into web/dist
make test    # go vet + go test
make run     # build and start on :8080
make image   # docker build
```

| Env var | Default | Meaning |
|---|---|---|
| `DOCU_ADDR` | `:8080` | Listen address |
| `DOCU_BASE_PATH` | *(empty)* | Sub-path when served behind a gateway, e.g. `/docu-ui` |
| `DOCU_DATA_DIR` | `/data` | Where the SQLite database (`docu-ui.db`) lives; mount a volume here |
| `DOCU_TLS_CERT` / `DOCU_TLS_KEY` | *(empty)* | PEM cert and key to serve HTTPS directly (standalone). Leave empty behind a TLS-terminating gateway |
| `DOCU_INSECURE_COOKIE` | `false` | `true` drops the Secure flag from the session cookie so sign-in works over plain HTTP. Trusted networks only |

## First run

On first start there is no account. Docu-UI prints a one-time **setup token** to its log:

```sh
docker logs docu-ui 2>&1 | grep setup_token
```

Open the UI, enter the token, and create the admin account. Two-factor (TOTP) is optional on that page.
The setup page closes for good once the first account exists; a new token is printed on each restart until then.

## Env files

After signing in, open **Env files** and enter the folder that holds your `*.env` files.
The folder is saved in the database and applies at once — no env var, no restart.
It is a path **inside the container**, so mount the host folder first:

```sh
docker run -v /opt/textiq/env:/host/env -v docu-data:/data ... docu-ui
```

Mount a parent folder (e.g. `/opt/textiq:/host`) if you want to switch between sub-folders from the UI later.
Only plain `name.env` files directly in that folder are listed; symlinks pointing outside it are refused.
Values are masked; each one is loaded only when you click to show it, and that is logged (user, file, key — never the value).

### Editing

Change variables one by one (other values stay masked), or switch to **Edit as text** for bigger edits (shows every value).
Both show the changes before saving. If someone saved the same file in the meantime, the save is refused and you reload.

Before each save, the previous content is kept in `<folder>/.history/<file>/<time>_<user>.env` (newest 50 per file).
The file is rewritten in place, so its owner and mode stay as they are.

**History** lists the kept versions. Pick one to see what restoring it would change in the current file, then restore it.
A restore is a save like any other: the content it replaces goes to the history too, so it can be undone.

Docu-UI runs as user `65532` inside the image, so it needs write access to the folder (for `.history`) and to the files, e.g.:

```sh
sudo chown -R 65532:65532 /opt/textiq/env    # or: docker run --user <uid of the owner> ...
```

Without it, saving fails with `permission denied` and the file is left untouched.

`GET /healthz` returns `ok` for load balancer and container health checks.

## License

MIT. The web UI is based on [shadcn-admin](https://github.com/satnaing/shadcn-admin) (MIT, see `web/LICENSE`).
