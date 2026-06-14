-- name: GetProfileByTelegramID :one
SELECT * FROM profiles
WHERE telegram_id = $1;

-- name: LinkTelegram :one
-- Consume a valid, unexpired, unused code and attach the Telegram id to that
-- code's profile, all in one statement.
WITH consumed AS (
    UPDATE telegram_link_codes
    SET consumed_at = now()
    WHERE code = $1
      AND consumed_at IS NULL
      AND expires_at > now()
    RETURNING profile_id
)
UPDATE profiles
SET telegram_id = $2
FROM consumed
WHERE profiles.id = consumed.profile_id
RETURNING profiles.id, profiles.telegram_id, profiles.display_name, profiles.created_at;
