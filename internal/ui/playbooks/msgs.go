package playbooks

import "github.com/cyberslacks/termi/internal/store"

// PlaybooksLoadedMsg is sent when the playbook list is fetched from the DB.
type PlaybooksLoadedMsg struct {
	Playbooks []store.DBPlaybook
	Err       error
}

// OpenPlaybookFormMsg opens the create/edit form.
type OpenPlaybookFormMsg struct {
	Playbook *store.DBPlaybook // nil = new
}

// SavePlaybookMsg is emitted by the form on submit.
type SavePlaybookMsg struct {
	Playbook store.DBPlaybook
}

// DeletePlaybookMsg requests deletion.
type DeletePlaybookMsg struct {
	ID int64
}

// RunPlaybookMsg asks app.go to run the playbook against selected sessions.
type RunPlaybookMsg struct {
	Playbook store.DBPlaybook
	Sessions []store.Session
}

// PlaybookOutputLineMsg carries one line of ansible-playbook stdout/stderr.
type PlaybookOutputLineMsg struct {
	Text string
	Done bool
	Code int
	Err  error
}

// PlaybookFormCancelledMsg is sent when the user presses esc on the form.
type PlaybookFormCancelledMsg struct{}

// PlaybookCancelledMsg is sent when the user leaves the playbook screen.
type PlaybookCancelledMsg struct{}
