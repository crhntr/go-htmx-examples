# Example Webapps using [Go](https://go.dev) and [HTMX](https://htmx.org)

## Usage Summary

This table is a summary of stuff used in the various examples.

All requests are now routed with new Go 1.22 `http.ServeMux` instead of [`"github.com/julienschmidt/httprouter"`](github.com/julienschmidt/httprouter).

| Example            | HTMX Attributes                                               | State Management |
|--------------------|---------------------------------------------------------------|------------------|
| bulk-update        | hx-include, hx-put, hx-target                                 | sqlc             |
| click-to-edit      | hx-boost, hx-swap, hx-target                                  | sqlc             |
| click-to-load      | hx-get, hx-swap, hx-target                                    | json  in-memory  |
| delete-row         | hx-confirm, hx-delete, hx-swap, hx-target                     | json, in-memory  |
| edit-row           | hx-get, hx-include, hx-post, hx-swap, hx-target               | json, in-memory  |
| lazy-loading       | hx-get, hx-trigger                                            | N/A              |
| inline-validation  | hx-post, hx-swap, hx-target, hx-trigger, sse-connect sse-swap | in-memory        |
| spreadsheet        | hx-encoding, hx-get, hx-patch, hx-post, hx-swap, hx-trigger   | json, in-memory  |

* I am also using [Pico CSS](https://picocss.com) in some examples.

## Generation

This project uses some code generation. Before you execute `go generate ./...`,
you need to install:

- `go run github.com/maxbrunsfeld/counterfeiter/v6 -generate`
- `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest` see https://docs.sqlc.dev