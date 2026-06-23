-- name: CreateRecurringRule :one
INSERT INTO recurring_rules (
    user_id, amount, type, category, description,
    frequency, day_of_month, month_of_year, day_of_week
)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
RETURNING *;

-- name: ListRecurringRulesByUser :many
SELECT * FROM recurring_rules
WHERE user_id = $1
ORDER BY created_at;

-- name: DeleteRecurringRule :execrows
DELETE FROM recurring_rules
WHERE id = $1 AND user_id = $2;
