-- name: ListContacts :many
SELECT * FROM contacts ORDER BY id;

-- name: UpdateContact :exec
UPDATE contacts SET first_name = ?, last_name = ?, email = ? WHERE id = ?;

-- name: ContactWithID :one
SELECT * FROM contacts WHERE id = ?;