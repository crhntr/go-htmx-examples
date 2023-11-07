# Example Webapps using [Go](https://go.dev) and [HTMX](https://htmx.org)

## Usage Summary

This table is a summary of stuff used in the various examples.
| Example       | HTMX Attributes (everything uses hx-target)                 | State Management | Router                              |
|---------------|-------------------------------------------------------------|------------------|-------------------------------------|
| bulk-update   | hx-include, hx-put, hx-target                               | sqlc             | github.com/julienschmidt/httprouter |
| click-to-edit | hx-boost, hx-swap, hx-target                                | sqlc             | github.com/julienschmidt/httprouter |
| click-to-load | hx-get, hx-swap, hx-target                                  | json  in-memory  | net/html                            |
| delete-row    | hx-confirm, hx-delete, hx-swap, hx-target                   | json, in-memory  | net/html                            |
| edit-row      | hx-get, hx-include, hx-post, hx-swap, hx-target             | json, in-memory  | github.com/julienschmidt/httprouter |
| lazy-loading  | hx-get, hx-trigger                                          | N/A              | net/html                            |
| spreadsheet   | hx-encoding, hx-get, hx-patch, hx-post, hx-swap, hx-trigger | json, in-memory  | github.com/julienschmidt/httprouter |

* I am also using [Pico CSS](https://picocss.com) in some examples.

## Generation

This project uses some code generation. Before you execute `go generate ./...`,
you need to install:

- `go run github.com/maxbrunsfeld/counterfeiter/v6 -generate`
- `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest` see https://docs.sqlc.dev