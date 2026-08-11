-- name: CreateBook :one
INSERT INTO groups (owner_id, name)
VALUES ($1, $2)
RETURNING *;

-- name: GetBookByOwnerAndName :one
SELECT * FROM groups
WHERE owner_id = $1 AND name = $2;

-- name: GetBook :one
SELECT * FROM groups
WHERE id = $1;

-- name: ListBooksByUser :many
SELECT g.*, gm.role
FROM groups g
JOIN group_members gm ON g.id = gm.group_id
WHERE gm.user_id = $1
ORDER BY g.name;

-- name: AddGroupMember :exec
INSERT INTO group_members (group_id, user_id, role)
VALUES ($1, $2, $3);

-- name: RemoveGroupMember :exec
DELETE FROM group_members
WHERE group_id = $1 AND user_id = $2 AND role != 'owner';

-- name: GetGroupMember :one
SELECT * FROM group_members
WHERE group_id = $1 AND user_id = $2;

-- name: SetActiveBook :exec
UPDATE profiles
SET active_book_id = $2
WHERE id = $1;
