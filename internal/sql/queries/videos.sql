-- name: CreateVideo :one
INSERT INTO videos (id, created_at, updated_at, lift_type, user_id, lift_result)
VALUES (
	gen_random_uuid(),
	NOW(),
	NOW(),
	$1,
	$2,
	$3
)
RETURNING *;

-- name: DeleteVideos :exec
DELETE
FROM videos;

-- name: DeleteVideoById :exec
DELETE
FROM videos
where id = $1;

-- name: GetVideo :one
SELECT *
FROM videos
WHERE id = $1;

-- name: GetSingleUserVideos :many
SELECT *
FROM videos
WHERE user_id = $1
ORDER BY created_at desc;
