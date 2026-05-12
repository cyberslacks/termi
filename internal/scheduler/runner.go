package scheduler

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/cyberslacks/termi/internal/audit"
	"github.com/cyberslacks/termi/internal/playbook"
	"github.com/cyberslacks/termi/internal/store"
	"github.com/robfig/cron/v3"
)

// ApprovalRequest is sent to the TUI when an interactive job fires.
type ApprovalRequest struct {
	Job         store.ScheduledJob
	PlaybookName string
	PlanSummary string
	ExpiresAt   time.Time
	ResponseCh  chan<- bool
}

type Runner struct {
	cron         *cron.Cron
	repo         *store.SchedulerRepo
	playbookRepo *store.PlaybookRepo
	sessionRepo  *store.SessionRepo
	executor     *playbook.Executor
	auditLog     *audit.Logger
	approvalCh   chan ApprovalRequest
	entryIDs     map[int64]cron.EntryID
	mu           sync.Mutex
}

func New(
	repo *store.SchedulerRepo,
	pbRepo *store.PlaybookRepo,
	sessionRepo *store.SessionRepo,
	executor *playbook.Executor,
	auditLog *audit.Logger,
) *Runner {
	return &Runner{
		cron:         cron.New(cron.WithSeconds()),
		repo:         repo,
		playbookRepo: pbRepo,
		sessionRepo:  sessionRepo,
		executor:     executor,
		auditLog:     auditLog,
		approvalCh:   make(chan ApprovalRequest, 32),
		entryIDs:     make(map[int64]cron.EntryID),
	}
}

// Start loads all enabled jobs and begins the cron loop.
func (r *Runner) Start(ctx context.Context) error {
	jobs, err := r.repo.List(ctx)
	if err != nil {
		return fmt.Errorf("load scheduled jobs: %w", err)
	}
	for _, j := range jobs {
		if j.Enabled {
			if err := r.AddJob(j); err != nil {
				log.Printf("scheduler: failed to register job %d %q: %v", j.ID, j.Name, err)
			}
		}
	}
	r.cron.Start()
	return nil
}

func (r *Runner) Stop() {
	ctx := r.cron.Stop()
	<-ctx.Done()
}

// ApprovalCh returns the channel on which approval requests arrive.
func (r *Runner) ApprovalCh() <-chan ApprovalRequest {
	return r.approvalCh
}

// AddJob registers a scheduled job.
func (r *Runner) AddJob(j store.ScheduledJob) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	entryID, err := r.cron.AddFunc(j.CronExpr, r.makeFunc(j))
	if err != nil {
		return fmt.Errorf("parse cron expr %q: %w", j.CronExpr, err)
	}
	r.entryIDs[j.ID] = entryID
	return nil
}

// RemoveJob removes a cron entry.
func (r *Runner) RemoveJob(jobID int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if entryID, ok := r.entryIDs[jobID]; ok {
		r.cron.Remove(entryID)
		delete(r.entryIDs, jobID)
	}
}

// TrustPlaybook marks a playbook as trusted in the DB.
func (r *Runner) TrustPlaybook(ctx context.Context, playbookID int64) error {
	return r.playbookRepo.Trust(ctx, playbookID)
}

func (r *Runner) makeFunc(j store.ScheduledJob) func() {
	return func() {
		ctx := context.Background()

		latest, err := r.repo.Get(ctx, j.ID)
		if err != nil || !latest.Enabled {
			return
		}

		pb, err := r.playbookRepo.Get(ctx, latest.PlaybookID)
		if err != nil {
			log.Printf("scheduler job %d: failed to load playbook: %v", j.ID, err)
			return
		}

		_ = r.repo.TouchLastRun(ctx, j.ID)

		targets, err := r.resolveTargets(ctx, latest)
		if err != nil || len(targets) == 0 {
			log.Printf("scheduler job %d: no targets: %v", j.ID, err)
			return
		}

		switch latest.Mode {
		case store.ApprovalAutonomous:
			r.executeAutonomous(ctx, *latest, pb, targets)
		case store.ApprovalInteractive:
			r.requestApproval(ctx, *latest, pb, targets)
		}
	}
}

func (r *Runner) executeAutonomous(ctx context.Context, j store.ScheduledJob, pb *store.DBPlaybook, targets []playbook.AnsibleTarget) {
	detail := fmt.Sprintf("scheduled-job:%d", j.ID)
	pbID := pb.ID

	lineCh, err := r.executor.Run(ctx, pb.FilePath, targets, j.Variables)
	if err != nil {
		log.Printf("scheduler job %d: run: %v", j.ID, err)
		return
	}

	var outputBuf strings.Builder
	for line := range lineCh {
		if line.Text != "" {
			outputBuf.WriteString(line.Text + "\n")
		}
		if line.Done {
			code := line.Code
			r.auditLog.Log(&store.AuditEntry{
				Actor:         store.ActorScheduler,
				ActorDetail:   detail,
				Command:       fmt.Sprintf("ansible-playbook %s", pb.FilePath),
				ExitCode:      &code,
				OutputSnippet: truncate(outputBuf.String(), 512),
				PlaybookID:    &pbID,
			})
			return
		}
	}
}

func (r *Runner) requestApproval(ctx context.Context, j store.ScheduledJob, pb *store.DBPlaybook, targets []playbook.AnsibleTarget) {
	responseCh := make(chan bool, 1)
	req := ApprovalRequest{
		Job:          j,
		PlaybookName: pb.Name,
		PlanSummary:  fmt.Sprintf("Scheduled job %q wants to run Ansible playbook %q against %d host(s)", j.Name, pb.Name, len(targets)),
		ExpiresAt:    time.Now().Add(10 * time.Minute),
		ResponseCh:   responseCh,
	}
	select {
	case r.approvalCh <- req:
	default:
		log.Printf("scheduler job %d: approval channel full, skipping run", j.ID)
		return
	}

	select {
	case approved := <-responseCh:
		if approved {
			r.executeAutonomous(ctx, j, pb, targets)
		}
	case <-time.After(10 * time.Minute):
		log.Printf("scheduler job %d: approval timed out", j.ID)
	}
}

// resolveTargets builds AnsibleTarget list from the job's session IDs.
func (r *Runner) resolveTargets(ctx context.Context, j *store.ScheduledJob) ([]playbook.AnsibleTarget, error) {
	var targets []playbook.AnsibleTarget
	for _, sid := range j.SessionIDs {
		sess, err := r.sessionRepo.Get(ctx, sid)
		if err != nil {
			log.Printf("scheduler: session %d not found: %v", sid, err)
			continue
		}
		t := playbook.AnsibleTarget{
			Name: sess.Name,
			Host: sess.Host,
			Port: sess.Port,
			User: sess.User,
		}
		if sess.AuthMethod == store.AuthKeyFile || sess.AuthMethod == store.AuthKeyRing {
			t.KeyPath = sess.CredentialID
		}
		targets = append(targets, t)
	}
	return targets, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
