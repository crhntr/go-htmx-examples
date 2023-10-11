# Example Webapps using [Go](https://go.dev) and [HTMX](https://htmx.org)

I am also using [Pico CSS](https://picocss.com) and [SQLite3+SQLC](https://docs.sqlc.dev/en/latest/tutorials/getting-started-sqlite.html).


## Generation

This project uses some code generation. Before you execute `go generate ./...`,
you need to install:

- `go run github.com/maxbrunsfeld/counterfeiter/v6 -generate`
- `go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest` see https://docs.sqlc.dev