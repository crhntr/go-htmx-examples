-- name: ListColors :many
SELECT * FROM colors ORDER BY id;

-- name: SetStatus :exec
UPDATE colors SET active = ? WHERE id = ?;