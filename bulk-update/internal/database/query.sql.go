// Code generated by sqlc. DO NOT EDIT.
// versions:
//   sqlc v1.22.0
// source: query.sql

package database

import (
	"context"
)

const listColors = `-- name: ListColors :many
SELECT id, name, active FROM colors ORDER BY id
`

func (q *Queries) ListColors(ctx context.Context) ([]Color, error) {
	rows, err := q.db.QueryContext(ctx, listColors)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []Color
	for rows.Next() {
		var i Color
		if err := rows.Scan(&i.ID, &i.Name, &i.Active); err != nil {
			return nil, err
		}
		items = append(items, i)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return items, nil
}

const setStatus = `-- name: SetStatus :exec
UPDATE colors SET active = ? WHERE id = ?
`

type SetStatusParams struct {
	Active bool
	ID     int64
}

func (q *Queries) SetStatus(ctx context.Context, arg SetStatusParams) error {
	_, err := q.db.ExecContext(ctx, setStatus, arg.Active, arg.ID)
	return err
}