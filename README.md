# Example Webapps using [Go](https://go.dev) and [HTMX](https://htmx.org)

I am also using [Pico CSS](https://picocss.com) and [SQLite3+SQLC](https://docs.sqlc.dev/en/latest/tutorials/getting-started-sqlite.html).

## Examples

This table is a summary of stuff used in the various examples.
| Example       | HTMX Attributes *                               | State Management | Router                              |
|---------------|-------------------------------------------------|------------------|-------------------------------------|
| bulk-update   | hx-include, hx-put                              | sqlc             | github.com/julienschmidt/httprouter |
| click-to-edit | hx-boost, hx-swap                               | sqlc             | github.com/julienschmidt/httprouter |
| click-to-load | hx-get, hx-swap                                 | json  in-memory  | net/html                            |
| delete-row    | hx-confirm, hx-delete, hx-swap                  | json, in-memory  | net/html                            |
| edit-row      | hx-get, hx-include, hx-post, hx-swap            | json, in-memory  | github.com/julienschmidt/httprouter |
| spreadsheet   | hx-encoding, hx-get, hx-patch, hx-post, hx-swap | json, in-memory  | github.com/julienschmidt/httprouter |
* everything uses:  hx-target

## Generation

This project uses some code generation. Before you execute `go generate ./...`,
you need to install:

- `go run github.com/maxbrunsfeld/counterfeiter/v6 -generate`
- `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest` see https://docs.sqlc.dev