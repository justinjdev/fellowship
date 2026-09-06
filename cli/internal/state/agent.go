package state

import (
	"fmt"
	"time"

	"zombiezen.com/go/sqlite"
	"zombiezen.com/go/sqlite/sqlitex"
)

// RecordAgentQuest maps a subagent id (the agent_id Claude Code puts in a
// subagent's hook payloads) to the quest it works. Written by the hooks
// whenever a subagent's tool call resolves to a quest through a path it
// names; read back for the calls that name none.
func RecordAgentQuest(conn *sqlite.Conn, agentID, questName string) error {
	if agentID == "" || questName == "" {
		return nil
	}
	err := sqlitex.Execute(conn, `INSERT INTO agent_quests (agent_id, quest_name, updated_at)
		VALUES (:agent, :quest, :now)
		ON CONFLICT(agent_id) DO UPDATE SET quest_name = :quest, updated_at = :now`,
		&sqlitex.ExecOptions{
			Named: map[string]any{
				":agent": agentID,
				":quest": questName,
				":now":   time.Now().UTC().Format(time.RFC3339),
			},
		})
	if err != nil {
		return fmt.Errorf("state: record agent quest: %w", err)
	}
	return nil
}

// FindQuestByAgent returns the quest recorded for a subagent id, or "".
func FindQuestByAgent(conn *sqlite.Conn, agentID string) (string, error) {
	if agentID == "" {
		return "", nil
	}
	var name string
	err := sqlitex.Execute(conn, `SELECT quest_name FROM agent_quests WHERE agent_id = :agent`,
		&sqlitex.ExecOptions{
			Named: map[string]any{":agent": agentID},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				name = stmt.ColumnText(0)
				return nil
			},
		})
	if err != nil {
		return "", fmt.Errorf("state: find quest by agent: %w", err)
	}
	return name, nil
}

// QuestWorktree returns the worktree registered for a quest, or "".
func QuestWorktree(conn *sqlite.Conn, questName string) (string, error) {
	var wt string
	err := sqlitex.Execute(conn, `SELECT COALESCE(worktree, '') FROM fellowship_quests WHERE name = :name`,
		&sqlitex.ExecOptions{
			Named: map[string]any{":name": questName},
			ResultFunc: func(stmt *sqlite.Stmt) error {
				wt = stmt.ColumnText(0)
				return nil
			},
		})
	if err != nil {
		return "", fmt.Errorf("state: quest worktree: %w", err)
	}
	return wt, nil
}
