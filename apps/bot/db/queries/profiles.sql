-- name: GetProfileByTelegramID :one
SELECT * FROM profiles
WHERE telegram_id = $1;
