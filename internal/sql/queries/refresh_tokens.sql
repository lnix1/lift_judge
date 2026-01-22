-- name: CreateRefreshToken :one
INSERT INTO refresh_tokens (token, created_at, updated_at, expires_at, revoked_at, user_id)
VALUES (
	$1,
	NOW(),
	NOW(),
	NOW() + INTERVAL '60 DAY',
	null,
	$2
)
RETURNING *;

-- name: GetRefreshToken :one
SELECT *, expires_at > NOW() as expired_bool, revoked_at is null as revoke_check
FROM refresh_tokens
WHERE token = $1;

-- name: RevokeRefreshToken :exec
UPDATE refresh_tokens
SET revoked_at = NOW(), updated_at = NOW()
WHERE token = $1;
