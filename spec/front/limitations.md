# Darow — Accepted limitations (Vue 3 UI)

> **Role** — what the `web/` admin application deliberately does not do. These are not defects to
> fix but scope decisions. A limitation that is lifted leaves this file and becomes an entry of
> [todo.md](todo.md).
> Neighbours: [blockers.md](blockers.md) ·
> [improvements.md](improvements.md) · [spec.md](spec.md) (rules and
> architecture).
>
> Go server: [../back/limitations.md](../back/limitations.md) · Project: [../project/limitations.md](../project/limitations.md).

Each entry carries a stable identifier (`LIM-Fn`); a retired identifier is never reused.

| # | Limitation | Consequence | Rule |
|---|---|---|---|
| LIM-F1 | No automated tests on the front: verification is manual, backed by the type-check (`mise run ui-build`) and the lint (`mise run ui-lint`) | No behavioural regression is caught by CI; `client/http.test.ts` remains the only net, over the HTTP transport | §0.14 |
| LIM-F2 | No i18n: the product is monolingual, every visible string is in French | Adding a language is a chantier of its own, not a setting | §0.17 |
