-- name: CreateVideo :one
INSERT INTO videos (id, created_at, updated_at, user_id)
VALUES (
	gen_random_uuid(),
	NOW(),
	NOW(),
	$1
)
RETURNING *;

-- name: DeleteVideos :exec
DELETE
FROM videos;

-- name: GetVideo :one
SELECT *
FROM videos
WHERE id = $1;

-- name: GetSingleUserVideos :many
SELECT *
FROM videos
WHERE user_id = $1;
