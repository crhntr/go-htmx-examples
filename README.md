# Example Webapps using [Go](https://go.dev) and [HTMX](https://htmx.org)

I am also using [Pico CSS](https://picocss.com) and [SQLite3+SQLC](https://docs.sqlc.dev/en/latest/tutorials/getting-started-sqlite.html).

## Examples

This table is a summary of stuff used in the various examples.
| Example       | Router                              | HTMX Attributes *                               | State Management |
|---------------|-------------------------------------|-------------------------------------------------|------------------|
| bulk-update   | github.com/julienschmidt/httprouter | hx-include, hx-put                              | sqlc             |
| click-to-edit | github.com/julienschmidt/httprouter | hx-boost, hx-swap                               | sqlc             |
| click-to-load | net/html                            | hx-get, hx-swap                                 | json  in-memory  |
| delete-row    | net/html                            | hx-confirm, hx-delete, hx-swap                  | json, in-memory  |
| edit-row      | github.com/julienschmidt/httprouter | hx-get, hx-include, hx-post, hx-swap            | json, in-memory  |
| spreadsheet   | github.com/julienschmidt/httprouter | hx-encoding, hx-get, hx-patch, hx-post, hx-swap | json, in-memory  |
* everything uses:  hx-target

## Generation

This project uses some code generation. Before you execute `go generate ./...`,
you need to install:

- `go run github.com/maxbrunsfeld/counterfeiter/v6 -generate`
- `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest` see https://docs.sqlc.dev