-- name: AddChildAgent :one
INSERT INTO agent_agents (parent_agent_id, child_agent_id)
VALUES (?, ?)
RETURNING *;

-- name: RemoveChildAgent :exec
DELETE FROM agent_agents WHERE parent_agent_id = ? AND child_agent_id = ?;

-- name: ListChildAgents :many
SELECT aa.id, aa.parent_agent_id, aa.child_agent_id, aa.created_at,
       child.id as child_id, child.name as child_name, child.description as child_description,
       child.environment_id
FROM agent_agents aa
JOIN agents child ON aa.child_agent_id = child.id
WHERE aa.parent_agent_id = ?
ORDER BY child.name;

-- name: ListParentAgents :many
SELECT aa.id, aa.parent_agent_id, aa.child_agent_id, aa.created_at,
       parent.id as parent_id, parent.name as parent_name, parent.description as parent_description,
       parent.environment_id
FROM agent_agents aa
JOIN agents parent ON aa.parent_agent_id = parent.id
WHERE aa.child_agent_id = ?
ORDER BY parent.name;

-- name: GetChildAgentRelationship :one
SELECT * FROM agent_agents
WHERE parent_agent_id = ? AND child_agent_id = ?;

-- name: DeleteAllChildAgentsForAgent :exec
DELETE FROM agent_agents WHERE parent_agent_id = ?;
