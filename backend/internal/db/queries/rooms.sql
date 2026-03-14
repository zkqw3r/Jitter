-- name: CreateRoom :one
INSERT INTO rooms DEFAULT VALUES RETURNING *;

-- name: GetRoom :one
SELECT * FROM rooms WHERE id = $1;

-- name: DeleteRoom :exec
DELETE FROM rooms where id = $1;