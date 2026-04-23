package analytics

// UserMetrics holds per-user engagement metrics for a single day.
type UserMetrics struct {
	User              UserRef           `json:"user"`
	ChatMetrics       ChatMetrics       `json:"chat_metrics"`
	ClaudeCodeMetrics ClaudeCodeMetrics `json:"claude_code_metrics"`
	WebSearchCount    int               `json:"web_search_count"`
}

// UserRef identifies a user in an analytics response.
type UserRef struct {
	ID           string `json:"id"`
	EmailAddress string `json:"email_address"`
}

// ChatMetrics captures web/app conversation engagement.
type ChatMetrics struct {
	ConversationCount     int `json:"distinct_conversation_count"`
	MessageCount          int `json:"message_count"`
	ProjectsCreatedCount  int `json:"distinct_projects_created_count"`
	ProjectsUsedCount     int `json:"distinct_projects_used_count"`
	FilesUploadedCount    int `json:"distinct_files_uploaded_count"`
	ArtifactsCreatedCount int `json:"distinct_artifacts_created_count"`
	ThinkingMessageCount  int `json:"thinking_message_count"`
	SkillsUsedCount       int `json:"distinct_skills_used_count"`
	ConnectorsUsedCount   int `json:"connectors_used_count"`
}

// ClaudeCodeMetrics captures Claude Code (IDE/CLI) engagement.
type ClaudeCodeMetrics struct {
	CoreMetrics CoreMetrics           `json:"core_metrics"`
	ToolActions map[string]ToolAction `json:"tool_actions"`
}

// CoreMetrics holds the primary Claude Code engagement numbers.
type CoreMetrics struct {
	CommitCount          int         `json:"commit_count"`
	PullRequestCount     int         `json:"pull_request_count"`
	LinesOfCode          LinesOfCode `json:"lines_of_code"`
	DistinctSessionCount int         `json:"distinct_session_count"`
}

// LinesOfCode tracks code additions and removals.
type LinesOfCode struct {
	AddedCount   int `json:"added_count"`
	RemovedCount int `json:"removed_count"`
}

// ToolAction records acceptance and rejection counts for a tool.
type ToolAction struct {
	AcceptedCount int `json:"accepted_count"`
	RejectedCount int `json:"rejected_count"`
}

// UsersResponse is the paginated envelope for the users analytics endpoint.
type UsersResponse struct {
	Data     []UserMetrics `json:"data"`
	NextPage *string       `json:"next_page"`
}

// DailySummary holds org-level engagement metrics for a single day.
// The API returns `starting_at` (inclusive, RFC3339) and `ending_at`
// (exclusive, always the next day, RFC3339) per item.
type DailySummary struct {
	StartingAt             string `json:"starting_at"`
	EndingAt               string `json:"ending_at"`
	DailyActiveUserCount   int    `json:"daily_active_user_count"`
	WeeklyActiveUserCount  int    `json:"weekly_active_user_count"`
	MonthlyActiveUserCount int    `json:"monthly_active_user_count"`
	AssignedSeatCount      int    `json:"assigned_seat_count"`
	PendingInviteCount     int    `json:"pending_invite_count"`
}

// Date extracts the YYYY-MM-DD date from the StartingAt timestamp.
func (d DailySummary) Date() string {
	if len(d.StartingAt) >= 10 {
		return d.StartingAt[:10]
	}
	return d.StartingAt
}

// SummariesResponse wraps the summaries endpoint response.
type SummariesResponse struct {
	Summaries []DailySummary `json:"summaries"`
}
