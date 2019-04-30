package main

// ActionContext passed from action buttons
type ActionContext struct {
	ReporterUserID string `json:"reporter_user_id"`
	Action         string `json:"action"`
}

// Action type for decoding action buttons
type Action struct {
	UserID  string         `json:"user_id"`
	PostID  string         `json:"post_id"`
	Context *ActionContext `json:"context"`
}
