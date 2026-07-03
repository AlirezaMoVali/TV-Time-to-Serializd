package service

type StepStatus string

const (
	StepPending          StepStatus = "pending"
	StepRunning          StepStatus = "running"
	StepDone             StepStatus = "done"
	StepWrongCredentials StepStatus = "wrong_credentials"
	StepError            StepStatus = "error"
)

type ShowRef struct {
	Name   string `json:"name"`
	TVDBID *int64 `json:"tvdb_id,omitempty"`
}

type LoginStep struct {
	Status  StepStatus `json:"status"`
	Message string     `json:"message,omitempty"`
}

type CountStep struct {
	Status    StepStatus `json:"status"`
	Phase     string     `json:"phase,omitempty"`
	Total     int        `json:"total,omitempty"`
	Done      int        `json:"done,omitempty"`
	Remaining int        `json:"remaining,omitempty"`
	Message   string     `json:"message,omitempty"`
}

type CheckExistingStep struct {
	Status    StepStatus `json:"status"`
	Total     int        `json:"total,omitempty"`
	Done      int        `json:"done,omitempty"`
	Remaining int        `json:"remaining,omitempty"`
	Skipped   []ShowRef  `json:"skipped,omitempty"`
}

type ImportStep struct {
	Status   StepStatus `json:"status"`
	Total    int        `json:"total,omitempty"`
	Done     int        `json:"done,omitempty"`
	Remaining int       `json:"remaining,omitempty"`
	NotFound []ShowRef  `json:"not_found,omitempty"`
}

type SummaryStep struct {
	Status             StepStatus `json:"status"`
	TotalTVTime        int        `json:"total_tvtime,omitempty"`
	AlreadyInSerializd int        `json:"already_in_serializd,omitempty"`
	NewlyAdded         int        `json:"newly_added,omitempty"`
	NotFound           int        `json:"not_found,omitempty"`
	ExportID           *string    `json:"export_id,omitempty"`
}

type MigrateProgress struct {
	JobID           string            `json:"job_id"`
	Status          string            `json:"status"`
	CurrentStep     int               `json:"current_step"`
	CurrentActivity string            `json:"current_activity,omitempty"`
	CurrentShow     string            `json:"current_show,omitempty"`
	ActiveShows     []string          `json:"active_shows,omitempty"`
	Queue           QueueInfo         `json:"queue,omitempty"`
	TVTimeLogin     LoginStep         `json:"tvtime_login"`
	SerializdLogin  LoginStep         `json:"serializd_login"`
	GatherShows     CountStep         `json:"gather_shows"`
	CheckExisting   CheckExistingStep `json:"check_existing"`
	ImportShows     ImportStep        `json:"import_shows"`
	Summary         SummaryStep       `json:"summary"`
	Logs            []string          `json:"logs"`
}

// QueueInfo describes migration queue position and wait estimates for queued jobs.
type QueueInfo struct {
	Position             int `json:"position,omitempty"`
	Ahead                int `json:"ahead,omitempty"`
	RunningSlotsUsed     int `json:"running_slots_used,omitempty"`
	RunningSlotsMax      int `json:"running_slots_max,omitempty"`
	EstimatedWaitSeconds int `json:"estimated_wait_seconds,omitempty"`
}
