# Project — Accepted limitations

> **Role** — what the project deliberately does not do at its outer layer. These are not defects
> but scope decisions, listed so that a user or a contributor knows where they stand. A limitation
> that is lifted leaves this file and becomes an entry of [todo.md](todo.md).
> Neighbours: [blockers.md](blockers.md) · [improvements.md](improvements.md) ·
> [spec.md](spec.md).
>
> Go server: [../back/limitations.md](../back/limitations.md) · Vue 3 UI:
> [../front/limitations.md](../front/limitations.md).

Each entry carries a stable identifier (`LIM-Pn`); a retired identifier is never reused.

| # | Limitation | Consequence | Reference |
|---|---|---|---|
| LIM-P1 | No release process: no version tag, no changelog, no published binary or image | The only way to obtain the product is to build it from a clone; there is nothing to pin to | `spec.md` §5 |
| LIM-P2 | Only the Linux/amd64 path is exercised — CI builds a single Docker image and no cross-platform matrix | Other platforms may work but are unverified | `spec.md` §6 |
