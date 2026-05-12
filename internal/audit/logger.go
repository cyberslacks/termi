package audit

import (
	"context"
	"log"
	"time"

	"github.com/jtfrow/termi/internal/store"
)

// Logger writes audit entries non-blocking via a channel-backed goroutine.
type Logger struct {
	repo *store.AuditRepo
	ch   chan op
	done chan struct{}
}

type opKind int

const (
	opInsert opKind = iota
	opComplete
)

type op struct {
	kind          opKind
	entry         *store.AuditEntry
	id            int64
	exitCode      int
	outputSnippet string
	result        chan error
}

func New(repo *store.AuditRepo) *Logger {
	l := &Logger{
		repo: repo,
		ch:   make(chan op, 512),
		done: make(chan struct{}),
	}
	go l.loop()
	return l
}

func (l *Logger) loop() {
	defer close(l.done)
	ctx := context.Background()
	for o := range l.ch {
		switch o.kind {
		case opInsert:
			err := l.repo.Insert(ctx, o.entry)
			if o.result != nil {
				o.result <- err
			} else if err != nil {
				log.Printf("audit insert error: %v", err)
			}
		case opComplete:
			err := l.repo.Complete(ctx, o.id, o.exitCode, o.outputSnippet)
			if o.result != nil {
				o.result <- err
			} else if err != nil {
				log.Printf("audit complete error: %v", err)
			}
		}
	}
}

// Log queues an entry non-blocking. If the channel is full it logs and drops.
func (l *Logger) Log(e *store.AuditEntry) {
	if e.ExecutedAt.IsZero() {
		e.ExecutedAt = time.Now()
	}
	select {
	case l.ch <- op{kind: opInsert, entry: e}:
	default:
		log.Printf("audit log channel full, dropping entry for session %d", e.SessionID)
	}
}

// LogSync inserts an entry and waits for the ID to be populated.
func (l *Logger) LogSync(ctx context.Context, e *store.AuditEntry) error {
	if e.ExecutedAt.IsZero() {
		e.ExecutedAt = time.Now()
	}
	result := make(chan error, 1)
	l.ch <- op{kind: opInsert, entry: e, result: result}
	select {
	case err := <-result:
		return err
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Complete updates exit code and output snippet for an existing entry.
func (l *Logger) Complete(id int64, exitCode int, outputSnippet string) {
	if len(outputSnippet) > 512 {
		outputSnippet = outputSnippet[:512]
	}
	select {
	case l.ch <- op{kind: opComplete, id: id, exitCode: exitCode, outputSnippet: outputSnippet}:
	default:
		log.Printf("audit log channel full, dropping complete for entry %d", id)
	}
}

func (l *Logger) Query(ctx context.Context, f store.AuditFilter) ([]store.AuditEntry, error) {
	return l.repo.Query(ctx, f)
}

// Close flushes all pending entries and shuts down the background goroutine.
func (l *Logger) Close() {
	close(l.ch)
	<-l.done
}
