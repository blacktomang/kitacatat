-- name: GetProfileByTelegramID :one
SELECT * FROM profiles
WHERE telegram_id = $1;

-- name: GetProfileByTelegramUsername :one
SELECT * FROM profiles
WHERE telegram_username = $1;
