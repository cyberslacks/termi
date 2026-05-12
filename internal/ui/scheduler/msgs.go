package scheduler

import "github.com/cyberslacks/termi/internal/store"

// SchedulerLoadedMsg is sent when the job list is fetched from the DB.
type SchedulerLoadedMsg struct {
	Jobs          []store.ScheduledJob
	PlaybookNames map[int64]string // playbookID → name
	Err           error
}

// OpenJobFormMsg opens the create/edit form. nil = new, non-nil = edit.
type OpenJobFormMsg struct {
	Job *store.ScheduledJob
}

// SaveJobMsg is emitted by the form when the user submits.
type SaveJobMsg struct {
	Job store.ScheduledJob
}

// DeleteJobMsg is sent when the user confirms deletion.
type DeleteJobMsg struct {
	ID int64
}

// ToggleJobMsg enables or disables a job.
type ToggleJobMsg struct {
	ID      int64
	Enabled bool
}

// JobFormCancelledMsg is sent when the user exits the form without saving.
type JobFormCancelledMsg struct{}
