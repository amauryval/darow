# darow — architecture

darow is one module, `darow`, hosted by the MyAppLibs platform (`go-pkg`, `web-platform`). The
behaviour is specified in `spec/darow/back.md` and `spec/darow/front.md`.

## Layers

```mermaid
flowchart LR
    browser[Browser: web/src/darow on the platform's shell] -->|/api/v1| proxy[cmd/darow: reverse proxy and front]
    gis[GIS client] -->|/api/v1/darow/ogc| proxy
    proxy --> api[AppStarter: middlewares, auth, events, backups]
    api --> rest[internal/darow/rest]
    rest --> engine[internal/darow/engine]
    rest --> sources[internal/darow/sources]
    rest --> filters[internal/darow/filters]
    rest --> settings[internal/darow/settings]
    settings --> sqlite
    engine --> duckdb[(darow.duckdb)]
    filters --> sqlite[(darow.db)]
    sources --> files[(sources/)]
```

## Storage

| File                     | Holds                                             | Archived                             |
| ------------------------ | ------------------------------------------------- | ------------------------------------ |
| `<workdir>/darow.db`     | the saved filters, the settings (platform SQLite) | yes                                  |
| `<workdir>/darow.duckdb` | the tables, `__id` numbered, geometry 3857        | yes, as an attachment of the archive |
| `<workdir>/sources/`     | the uploaded source files                         | no                                   |

## A table's life

```mermaid
sequenceDiagram
    participant UI
    participant REST as rest
    participant Engine as engine
    UI->>REST: POST /sources (file)
    UI->>REST: POST /sources/{name}/load
    REST->>Engine: CreateFromFile (ST_Read, ST_Transform to 3857, indexes)
    UI->>REST: GET /tables/{table}/rows?where=
    UI->>REST: GET /tables/{table}/tiles/{z}/{x}/{y}?where=
    REST->>Engine: Rows, Tile (ST_AsMVT)
```

## Front

`web/src/darow`: `api` (the client and the generated types) → `stores` (Pinia) → `composables` (the
logic) → `views` (the pages: tables, saved filters, sources, SQL). The shell, the layout, the
backups and erasures pages, the documentation page and the sign-in are the platform's.
