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

`GET /healthz` returns `ok` for load balancer and container health checks.

## License

MIT. The web UI is based on [shadcn-admin](https://github.com/satnaing/shadcn-admin) (MIT, see `web/LICENSE`).
