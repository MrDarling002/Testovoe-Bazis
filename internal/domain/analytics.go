package domain

type TeamSummary struct {
	TeamID        int64  `db:"team_id" json:"team_id"`
	TeamName      string `db:"team_name" json:"team_name"`
	MembersCount  int64  `db:"members_count" json:"members_count"`
	DoneLast7Days int64  `db:"done_last_7_days" json:"done_last_7_days"`
}

type TopCreator struct {
	TeamID       int64  `db:"team_id" json:"team_id"`
	TeamName     string `db:"team_name" json:"team_name"`
	UserID       int64  `db:"user_id" json:"user_id"`
	Username     string `db:"username" json:"username"`
	TasksCreated int64  `db:"tasks_created" json:"tasks_created"`
	RankPosition int64  `db:"rank_position" json:"rank"`
}

type InvalidAssigneeTask struct {
	TaskID     int64  `db:"task_id" json:"task_id"`
	TaskTitle  string `db:"task_title" json:"task_title"`
	TeamID     int64  `db:"team_id" json:"team_id"`
	AssigneeID int64  `db:"assignee_id" json:"assignee_id"`
}
