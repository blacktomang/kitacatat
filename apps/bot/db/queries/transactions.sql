-- name: CreateTransaction :one
INSERT INTO transactions (user_id, amount, type, category, description, occurred_at, group_id)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListTransactions :many
SELECT * FROM transactions
ORDER BY occurred_at DESC
LIMIT $1 OFFSET $2;

-- name: ListTransactionsByUser :many
SELECT * FROM transactions
WHERE user_id = $1
ORDER BY occurred_at DESC
LIMIT $2 OFFSET $3;
