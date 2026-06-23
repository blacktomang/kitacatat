-- name: CreateLoginToken :exec
INSERT INTO login_tokens (token, telegram_id, telegram_username, display_name, expires_at)
VALUES ($1, $2, $3, $4, $5);
