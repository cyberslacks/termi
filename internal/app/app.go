package app

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/jtfrow/termi/internal/audit"
	"github.com/jtfrow/termi/internal/config"
	"github.com/jtfrow/termi/internal/creds"
	"github.com/jtfrow/termi/internal/msgs"
	"github.com/jtfrow/termi/internal/playbook"
	"github.com/jtfrow/termi/internal/scheduler"
	"github.com/jtfrow/termi/internal/ssh"
	"github.com/jtfrow/termi/internal/store"
	uiaudit "github.com/jtfrow/termi/internal/ui/audit"
	uiai "github.com/jtfrow/termi/internal/ui/ai"
	"github.com/jtfrow/termi/internal/ui/broadcast"
	uipb "github.com/jtfrow/termi/internal/ui/playbooks"
	uisched "github.com/jtfrow/termi/internal/ui/scheduler"
	"github.com/jtfrow/termi/internal/ui/sessions"
	"github.com/jtfrow/termi/internal/ui/terminal"
)

// RootModel is the top-level Bubbletea model.
type RootModel struct {
	cfg    *config.Config
	width  int
	height int
	prog   *tea.Program

	activeScreen msgs.ScreenID

	// Sub-screens
	sessionList   sessions.ListModel
	sessionForm   sessions.FormModel
	broadcastMdl  broadcast.Model
	schedList     uisched.ListModel
	schedForm     uisched.FormModel
	schedFormOpen bool
	auditMdl      uiaudit.Model
	aiPanel       uiai.Model
	pbList        uipb.ListModel
	pbForm        uipb.FormModel
	pbFormOpen    bool
	pbRunView     uipb.RunView
	pbRunning     bool

	// Terminal tabs — one per open SSH session
	tabs       []terminal.Model
	activeTab  int
	prefixMode bool

	// Approval overlay
	pendingApproval *scheduler.ApprovalRequest

	// Backend services
	sshMgr        ssh.Manager
	credStore     creds.Store
	sessionRepo   *store.SessionRepo
	playbookRepo  *store.PlaybookRepo
	schedulerRepo *store.SchedulerRepo
	auditRepo     *store.AuditRepo
	ansibleExec   *playbook.Executor
	schedRunner   *scheduler.Runner
	auditLog      *audit.Logger

	statusMsg string
	errMsg    string
}

// Services holds all wired-up backend services.
type Services struct {
	SSHMgr        ssh.Manager
	CredStore     creds.Store
	SessionRepo   *store.SessionRepo
	PlaybookRepo  *store.PlaybookRepo
	SchedulerRepo *store.SchedulerRepo
	AuditRepo     *store.AuditRepo
	AnsibleExec   *playbook.Executor
	SchedRunner   *scheduler.Runner
	AuditLog      *audit.Logger
}

func New(cfg *config.Config, svc Services) *RootModel {
	return &RootModel{
		cfg:           cfg,
		activeScreen:  msgs.ScreenDashboard,
		sessionList:   sessions.NewListModel(80, 24),
		sessionForm:   sessions.NewFormModel(80),
		broadcastMdl:  broadcast.New(80, 24),
		schedList:     uisched.NewListModel(80, 24),
		auditMdl:      uiaudit.New(svc.AuditRepo, 80, 24),
		aiPanel:       uiai.NewModel(&cfg.AI, 80, 24),
		pbList:        uipb.NewListModel(80, 24),
		sshMgr:        svc.SSHMgr,
		credStore:     svc.CredStore,
		sessionRepo:   svc.SessionRepo,
		playbookRepo:  svc.PlaybookRepo,
		schedulerRepo: svc.SchedulerRepo,
		auditRepo:     svc.AuditRepo,
		ansibleExec:   svc.AnsibleExec,
		schedRunner:   svc.SchedRunner,
		auditLog:      svc.AuditLog,
	}
}

func (m *RootModel) SetProgram(p *tea.Program) { m.prog = p }

func (m *RootModel) Init() tea.Cmd {
	return tea.Batch(
		listenForApprovals(m.schedRunner),
		sessions.LoadCmd(m.sessionRepo),
	)
}

func (m *RootModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	// ── Window resize ─────────────────────────────────────────
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.sessionList.SetSize(msg.Width, msg.Height-2)
		m.sessionForm.SetWidth(msg.Width)
		m.broadcastMdl.SetSize(msg.Width, msg.Height-2)
		m.schedList.SetSize(msg.Width, msg.Height-2)
		m.auditMdl.SetSize(msg.Width, msg.Height-2)
		m.aiPanel.SetSize(msg.Width, msg.Height-2)
		m.pbList.SetSize(msg.Width, msg.Height-2)
		m.pbRunView.SetSize(msg.Width, msg.Height-2)
		for i := range m.tabs {
			m.tabs[i].SetSize(msg.Width, msg.Height-2)
		}
		return m, nil

	// ── Session UI ────────────────────────────────────────────
	case sessions.SessionsLoadedMsg:
		updated, cmd := m.sessionList.Update(msg)
		m.sessionList = updated
		return m, cmd

	case sessions_loaded_and_navigate:
		updated, cmd := m.sessionList.Update(msg.loaded)
		m.sessionList = updated
		m.activeScreen = msgs.ScreenSessionList
		m.statusMsg = "Session saved."
		return m, cmd

	case sessions.ConnectRequestMsg:
		return m, m.connectToSession(msg.Session)

	case sessions.OpenFormMsg:
		form := sessions.NewFormModel(m.width)
		if msg.Session != nil {
			form.LoadSession(*msg.Session)
		}
		m.sessionForm = form
		m.activeScreen = msgs.ScreenSessionForm
		return m, form.Init()

	case sessions.SaveSessionMsg:
		return m, m.saveSession(msg)

	case sessions.DeleteSessionMsg:
		return m, m.deleteSession(msg.ID)

	case sessions.FormCancelledMsg:
		m.activeScreen = msgs.ScreenSessionList
		return m, nil

	// ── Broadcast ─────────────────────────────────────────────
	case broadcast.RunBroadcastMsg:
		return m, m.runBroadcast(msg)

	case broadcast.BroadcastDoneMsg:
		updated, cmd := m.broadcastMdl.Update(msg)
		m.broadcastMdl = updated
		return m, cmd

	case broadcast.BroadcastCancelledMsg:
		m.activeScreen = msgs.ScreenDashboard
		return m, nil

	// ── Playbooks ─────────────────────────────────────────────
	case uipb.PlaybooksLoadedMsg:
		updated, cmd := m.pbList.Update(msg)
		m.pbList = updated
		return m, cmd

	case uipb.OpenPlaybookFormMsg:
		form := uipb.NewFormModel(m.width)
		if msg.Playbook != nil {
			form.LoadPlaybook(*msg.Playbook)
		}
		m.pbForm = form
		m.pbFormOpen = true
		return m, form.Init()

	case uipb.SavePlaybookMsg:
		return m, m.savePlaybook(msg)

	case uipb.DeletePlaybookMsg:
		return m, m.deletePlaybook(msg.ID)

	case uipb.RunPlaybookMsg:
		return m, m.runPlaybook(msg)

	case uipb.PlaybookOutputLineMsg:
		updated, cmd := m.pbRunView.Update(msg)
		m.pbRunView = updated
		if msg.Done {
			m.pbRunning = false
		}
		return m, cmd

	case uipb.PlaybookFormCancelledMsg:
		m.pbFormOpen = false
		return m, nil

	case uipb.PlaybookCancelledMsg:
		if m.pbRunning {
			m.pbRunning = false
		}
		m.activeScreen = msgs.ScreenDashboard
		return m, nil

	case pbSavedMsg:
		m.pbFormOpen = false
		m.statusMsg = "Playbook saved."
		updated, cmd := m.pbList.Update(msg.loaded)
		m.pbList = updated
		return m, cmd

	// ── Scheduler ─────────────────────────────────────────────
	case uisched.SchedulerLoadedMsg:
		if m.schedFormOpen {
			return m, nil
		}
		updated, cmd := m.schedList.Update(msg)
		m.schedList = updated
		return m, cmd

	case uisched.OpenJobFormMsg:
		return m, m.openSchedulerForm(msg)

	case uisched.SaveJobMsg:
		return m, m.saveJob(msg)

	case uisched.DeleteJobMsg:
		return m, m.deleteJob(msg.ID)

	case uisched.ToggleJobMsg:
		return m, m.toggleJob(msg)

	case uisched.JobFormCancelledMsg:
		m.schedFormOpen = false
		return m, nil

	case schedFormReadyMsg:
		m.schedForm = msg.form
		m.schedFormOpen = true
		return m, msg.form.Init()

	case schedSavedMsg:
		m.schedFormOpen = false
		m.statusMsg = "Job saved."
		updated, cmd := m.schedList.Update(msg.loaded)
		m.schedList = updated
		return m, cmd

	// ── Audit ─────────────────────────────────────────────────
	case uiaudit.AuditLoadedMsg:
		updated, cmd := m.auditMdl.Update(msg)
		m.auditMdl = updated
		return m, cmd

	case uiaudit.AuditCancelledMsg:
		m.activeScreen = msgs.ScreenDashboard
		return m, nil

	// ── AI Panel ──────────────────────────────────────────────
	case uiai.AIPanelCancelledMsg:
		m.activeScreen = msgs.ScreenDashboard
		return m, nil

	case uiai.AnsiblePlaybookSavedMsg:
		m.statusMsg = "Playbook saved to " + msg.Path
		return m, nil

	// ── SSH / terminal ─────────────────────────────────────────
	case msgs.SessionConnectedMsg:
		return m.handleConnected(msg)

	case msgs.SessionDisconnectedMsg:
		return m.handleDisconnected(msg)

	case msgs.SessionErrorMsg:
		m.errMsg = msg.Err.Error()
		return m, nil

	case msgs.TermOutputMsg:
		if msg.TabIndex < len(m.tabs) {
			updated, cmd := m.tabs[msg.TabIndex].Update(msg)
			m.tabs[msg.TabIndex] = updated
			return m, cmd
		}
		return m, nil

	case msgs.TermResizeMsg:
		for i := range m.tabs {
			updated, _ := m.tabs[i].Update(msg)
			m.tabs[i] = updated
		}
		return m, nil

	// ── Scheduler approval ─────────────────────────────────────
	case ApprovalRequestMsg:
		m.pendingApproval = &msg.Req
		m.statusMsg = fmt.Sprintf("[APPROVAL] %s — [y] approve / [n] reject", msg.Req.PlanSummary)
		return m, listenForApprovals(m.schedRunner)

	// ── Navigation ─────────────────────────────────────────────
	case msgs.NavigateMsg:
		return m.navigateTo(msg.Screen)

	// ── Keys ───────────────────────────────────────────────────
	case tea.KeyMsg:
		return m.handleKey(msg)
	}

	// Forward to AI panel for streaming chunks
	if m.activeScreen == msgs.ScreenAIPanel {
		updated, cmd := m.aiPanel.Update(msg)
		m.aiPanel = updated
		return m, cmd
	}

	return m, nil
}

func (m *RootModel) navigateTo(screen msgs.ScreenID) (tea.Model, tea.Cmd) {
	m.activeScreen = screen
	switch screen {
	case msgs.ScreenSessionList:
		return m, sessions.LoadCmd(m.sessionRepo)
	case msgs.ScreenBroadcast:
		m.broadcastMdl.SetSessions(m.sshMgr.ActiveSessions())
		return m, nil
	case msgs.ScreenScheduler:
		m.schedFormOpen = false
		return m, uisched.LoadCmd(m.schedulerRepo, m.playbookRepo)
	case msgs.ScreenAuditLog:
		return m, m.auditMdl.Init()
	case msgs.ScreenAIPanel:
		termOut := m.activeTerminalOutput()
		m.aiPanel.SetContext(m.sshMgr.ActiveSessions(), termOut)
		return m, m.aiPanel.Init()
	case msgs.ScreenPlaybooks:
		m.pbFormOpen = false
		m.pbRunning = false
		return m, uipb.LoadCmd(m.playbookRepo)
	}
	return m, nil
}

// activeTerminalOutput returns recent output from the focused terminal tab.
func (m *RootModel) activeTerminalOutput() string {
	if m.activeScreen != msgs.ScreenTerminal || len(m.tabs) == 0 {
		return ""
	}
	return m.tabs[m.activeTab].RecentOutput(m.cfg.AI.ContextLines)
}

func (m *RootModel) handleKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.pendingApproval != nil {
		switch msg.String() {
		case "y", "Y":
			m.pendingApproval.ResponseCh <- true
			m.pendingApproval = nil
			m.statusMsg = "Approved — executing..."
		case "n", "N":
			m.pendingApproval.ResponseCh <- false
			m.pendingApproval = nil
			m.statusMsg = "Rejected."
		}
		return m, nil
	}

	if m.prefixMode {
		m.prefixMode = false
		return m.handlePrefixKey(msg)
	}

	switch m.activeScreen {
	case msgs.ScreenTerminal:
		if len(m.tabs) > 0 {
			if msg.String() == m.cfg.UI.EscapeKey {
				m.prefixMode = true
				return m, nil
			}
			updated, cmd := m.tabs[m.activeTab].Update(msg)
			m.tabs[m.activeTab] = updated
			return m, cmd
		}

	case msgs.ScreenSessionList:
		updated, cmd := m.sessionList.Update(msg)
		m.sessionList = updated
		return m, cmd

	case msgs.ScreenSessionForm:
		updated, cmd := m.sessionForm.Update(msg)
		m.sessionForm = updated
		return m, cmd

	case msgs.ScreenBroadcast:
		updated, cmd := m.broadcastMdl.Update(msg)
		m.broadcastMdl = updated
		return m, cmd

	case msgs.ScreenScheduler:
		if m.schedFormOpen {
			updated, cmd := m.schedForm.Update(msg)
			m.schedForm = updated
			return m, cmd
		}
		updated, cmd := m.schedList.Update(msg)
		m.schedList = updated
		return m, cmd

	case msgs.ScreenAuditLog:
		updated, cmd := m.auditMdl.Update(msg)
		m.auditMdl = updated
		return m, cmd

	case msgs.ScreenAIPanel:
		updated, cmd := m.aiPanel.Update(msg)
		m.aiPanel = updated
		return m, cmd

	case msgs.ScreenPlaybooks:
		if m.pbRunning {
			updated, cmd := m.pbRunView.Update(msg)
			m.pbRunView = updated
			return m, cmd
		}
		if m.pbFormOpen {
			updated, cmd := m.pbForm.Update(msg)
			m.pbForm = updated
			return m, cmd
		}
		updated, cmd := m.pbList.Update(msg)
		m.pbList = updated
		return m, cmd
	}

	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit
	case m.cfg.UI.EscapeKey:
		m.prefixMode = true
	}
	return m, nil
}

func (m *RootModel) handlePrefixKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m, tea.Quit
	case "s":
		return m.navigateTo(msgs.ScreenSessionList)
	case "n":
		return m.Update(sessions.OpenFormMsg{Session: nil})
	case "p":
		return m.navigateTo(msgs.ScreenPlaybooks)
	case "S":
		return m.navigateTo(msgs.ScreenScheduler)
	case "a":
		return m.navigateTo(msgs.ScreenAuditLog)
	case "A":
		return m.navigateTo(msgs.ScreenAIPanel)
	case "b":
		return m.navigateTo(msgs.ScreenBroadcast)
	case "tab":
		if len(m.tabs) > 0 {
			m.activeTab = (m.activeTab + 1) % len(m.tabs)
			m.activeScreen = msgs.ScreenTerminal
		}
	case "shift+tab":
		if len(m.tabs) > 0 {
			m.activeTab = (m.activeTab - 1 + len(m.tabs)) % len(m.tabs)
			m.activeScreen = msgs.ScreenTerminal
		}
	case "x":
		return m, m.closeActiveTab()
	case "1", "2", "3", "4", "5", "6", "7", "8", "9":
		idx := int(msg.String()[0] - '1')
		if idx < len(m.tabs) {
			m.activeTab = idx
			m.activeScreen = msgs.ScreenTerminal
		}
	}
	return m, nil
}

func (m *RootModel) closeActiveTab() tea.Cmd {
	if len(m.tabs) == 0 {
		return nil
	}
	tabIdx := m.tabs[m.activeTab].TabIndex
	_ = m.sshMgr.Disconnect(tabIdx)
	m.tabs = append(m.tabs[:m.activeTab], m.tabs[m.activeTab+1:]...)
	if m.activeTab >= len(m.tabs) && len(m.tabs) > 0 {
		m.activeTab = len(m.tabs) - 1
	}
	if len(m.tabs) == 0 {
		m.activeScreen = msgs.ScreenDashboard
	}
	return nil
}

// ── SSH session lifecycle ──────────────────────────────────────────────────

func (m *RootModel) connectToSession(s store.Session) tea.Cmd {
	prog := m.prog
	return func() tea.Msg {
		ctx := context.Background()
		cred, err := m.credStore.Resolve(ctx, s)
		if err != nil {
			return msgs.SessionErrorMsg{Err: fmt.Errorf("credentials: %w", err)}
		}
		cfg := ssh.ConnectConfig{
			ConnectTimeout: m.cfg.SSH.ConnectTimeout,
			InitCols:       m.width,
			InitRows:       m.height - 2,
		}
		tabIndex, err := m.sshMgr.Connect(ctx, s, cred, cfg)
		if err != nil {
			return msgs.SessionErrorMsg{Err: fmt.Errorf("connect: %w", err)}
		}
		_ = m.sessionRepo.TouchConnected(ctx, s.ID)

		stdout, err := m.sshMgr.SessionStdout(tabIndex)
		if err != nil {
			return msgs.SessionErrorMsg{Err: fmt.Errorf("stdout: %w", err)}
		}
		terminal.StartBridge(tabIndex, s.ID, stdout, prog)

		return msgs.SessionConnectedMsg{
			TabIndex:    tabIndex,
			SessionID:   s.ID,
			SessionName: s.Name,
			Host:        s.Host,
		}
	}
}

func (m *RootModel) saveSession(msg sessions.SaveSessionMsg) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		s := msg.Session
		var err error
		if s.ID == 0 {
			err = m.sessionRepo.Create(ctx, &s)
		} else {
			err = m.sessionRepo.Update(ctx, &s)
		}
		if err != nil {
			return msgs.SessionErrorMsg{Err: fmt.Errorf("save session: %w", err)}
		}
		if msg.Password != "" {
			if storeErr := m.credStore.StorePassword(s.ID, msg.Password); storeErr != nil {
				return msgs.SessionErrorMsg{Err: fmt.Errorf("save password: %w", storeErr)}
			}
		}
		sess, _ := m.sessionRepo.List(ctx)
		groups, _ := m.sessionRepo.ListGroups(ctx)
		return sessions_loaded_and_navigate{
			loaded: sessions_listLoadedMsg{Sessions: sess, Groups: groups},
		}
	}
}

type sessions_loaded_and_navigate struct{ loaded sessions_listLoadedMsg }
type sessions_listLoadedMsg = sessions.SessionsLoadedMsg

func (m *RootModel) deleteSession(id int64) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if err := m.sessionRepo.Delete(ctx, id); err != nil {
			return msgs.SessionErrorMsg{Err: fmt.Errorf("delete: %w", err)}
		}
		_ = m.credStore.DeletePassword(id)
		sess, _ := m.sessionRepo.List(ctx)
		groups, _ := m.sessionRepo.ListGroups(ctx)
		return sessions.SessionsLoadedMsg{Sessions: sess, Groups: groups}
	}
}

func (m *RootModel) handleConnected(msg msgs.SessionConnectedMsg) (tea.Model, tea.Cmd) {
	tab := terminal.New(msg.TabIndex, msg.SessionID, msg.SessionName, msg.Host, m.sshMgr, m.width, m.height-2)
	m.tabs = append(m.tabs, tab)
	m.activeTab = len(m.tabs) - 1
	m.activeScreen = msgs.ScreenTerminal
	m.statusMsg = fmt.Sprintf("Connected: %s (%s)", msg.SessionName, msg.Host)
	m.errMsg = ""
	return m, nil
}

func (m *RootModel) handleDisconnected(msg msgs.SessionDisconnectedMsg) (tea.Model, tea.Cmd) {
	for i, t := range m.tabs {
		if t.TabIndex == msg.TabIndex {
			m.tabs = append(m.tabs[:i], m.tabs[i+1:]...)
			if m.activeTab >= len(m.tabs) && len(m.tabs) > 0 {
				m.activeTab = len(m.tabs) - 1
			}
			break
		}
	}
	if msg.Err != nil {
		m.errMsg = fmt.Sprintf("Disconnected: %v", msg.Err)
	} else {
		m.statusMsg = fmt.Sprintf("Session %d closed", msg.SessionID)
	}
	if len(m.tabs) == 0 {
		m.activeScreen = msgs.ScreenDashboard
	}
	return m, nil
}

// ── Broadcast ────────────────────────────────────────────────────────────────

func (m *RootModel) runBroadcast(msg broadcast.RunBroadcastMsg) tea.Cmd {
	sshMgr := m.sshMgr
	return func() tea.Msg {
		results := sshMgr.BroadcastCommand(msg.TabIndexes, msg.Command)
		return broadcast.BroadcastDoneMsg{Command: msg.Command, Results: results}
	}
}

// ── Playbooks ────────────────────────────────────────────────────────────────

func (m *RootModel) savePlaybook(msg uipb.SavePlaybookMsg) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		p := msg.Playbook
		var err error
		if p.ID == 0 {
			err = m.playbookRepo.Create(ctx, &p)
		} else {
			err = m.playbookRepo.Update(ctx, &p)
		}
		if err != nil {
			return msgs.SessionErrorMsg{Err: fmt.Errorf("save playbook: %w", err)}
		}
		pbs, _ := m.playbookRepo.List(ctx)
		return pbSavedMsg{loaded: uipb.PlaybooksLoadedMsg{Playbooks: pbs}}
	}
}

type pbSavedMsg struct{ loaded uipb.PlaybooksLoadedMsg }

func (m *RootModel) deletePlaybook(id int64) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		if err := m.playbookRepo.Delete(ctx, id); err != nil {
			return msgs.SessionErrorMsg{Err: fmt.Errorf("delete playbook: %w", err)}
		}
		pbs, _ := m.playbookRepo.List(ctx)
		return uipb.PlaybooksLoadedMsg{Playbooks: pbs}
	}
}

func (m *RootModel) runPlaybook(msg uipb.RunPlaybookMsg) tea.Cmd {
	// Resolve targets from sessions attached to active tabs
	targets := m.buildAnsibleTargets(msg.Sessions)
	if len(targets) == 0 {
		// Fall back to all active sessions
		targets = m.buildAnsibleTargetsFromActive()
	}
	if len(targets) == 0 {
		m.errMsg = "No active sessions to run against"
		return nil
	}

	pb := msg.Playbook
	m.pbRunView = uipb.NewRunView(pb, m.width, m.height-2)
	m.pbRunning = true

	exec := m.ansibleExec
	prog := m.prog
	return func() tea.Msg {
		ch, err := exec.Run(context.Background(), pb.FilePath, targets, nil)
		if err != nil {
			return uipb.PlaybookOutputLineMsg{Err: err, Done: true}
		}
		// Stream lines back via program.Send
		go func() {
			for line := range ch {
				if line.Text != "" {
					prog.Send(uipb.PlaybookOutputLineMsg{Text: line.Text})
				}
				if line.Done {
					prog.Send(uipb.PlaybookOutputLineMsg{Done: true, Code: line.Code, Err: line.Err})
					return
				}
			}
		}()
		return nil
	}
}

func (m *RootModel) buildAnsibleTargets(sessions []store.Session) []playbook.AnsibleTarget {
	var out []playbook.AnsibleTarget
	for _, s := range sessions {
		t := playbook.AnsibleTarget{
			Name: s.Name,
			Host: s.Host,
			Port: s.Port,
			User: s.User,
		}
		if s.AuthMethod == store.AuthKeyFile || s.AuthMethod == store.AuthKeyRing {
			t.KeyPath = s.CredentialID
		}
		out = append(out, t)
	}
	return out
}

func (m *RootModel) buildAnsibleTargetsFromActive() []playbook.AnsibleTarget {
	active := m.sshMgr.ActiveSessions()
	ctx := context.Background()
	var out []playbook.AnsibleTarget
	for _, a := range active {
		sess, err := m.sessionRepo.Get(ctx, a.SessionID)
		if err != nil {
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
		out = append(out, t)
	}
	return out
}

// ── Scheduler ────────────────────────────────────────────────────────────────

func (m *RootModel) openSchedulerForm(msg uisched.OpenJobFormMsg) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		pbs, _ := m.playbookRepo.List(ctx)
		form := uisched.NewFormModel(m.width, pbs)
		if msg.Job != nil {
			form.LoadJob(*msg.Job)
		}
		return schedFormReadyMsg{form: form}
	}
}

type schedFormReadyMsg struct{ form uisched.FormModel }

func (m *RootModel) saveJob(msg uisched.SaveJobMsg) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		j := msg.Job
		var err error
		if j.ID == 0 {
			err = m.schedulerRepo.Create(ctx, &j)
			if err == nil && j.Enabled {
				_ = m.schedRunner.AddJob(j)
			}
		} else {
			err = m.schedulerRepo.Update(ctx, &j)
			if err == nil {
				m.schedRunner.RemoveJob(j.ID)
				if j.Enabled {
					_ = m.schedRunner.AddJob(j)
				}
			}
		}
		if err != nil {
			return msgs.SessionErrorMsg{Err: fmt.Errorf("save job: %w", err)}
		}
		jobs, _ := m.schedulerRepo.List(ctx)
		pbs, _ := m.playbookRepo.List(ctx)
		names := make(map[int64]string, len(pbs))
		for _, p := range pbs {
			names[p.ID] = p.Name
		}
		return schedSavedMsg{loaded: uisched.SchedulerLoadedMsg{Jobs: jobs, PlaybookNames: names}}
	}
}

type schedSavedMsg struct{ loaded uisched.SchedulerLoadedMsg }

func (m *RootModel) deleteJob(id int64) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		m.schedRunner.RemoveJob(id)
		if err := m.schedulerRepo.Delete(ctx, id); err != nil {
			return msgs.SessionErrorMsg{Err: fmt.Errorf("delete job: %w", err)}
		}
		jobs, _ := m.schedulerRepo.List(ctx)
		pbs, _ := m.playbookRepo.List(ctx)
		names := make(map[int64]string, len(pbs))
		for _, p := range pbs {
			names[p.ID] = p.Name
		}
		return uisched.SchedulerLoadedMsg{Jobs: jobs, PlaybookNames: names}
	}
}

func (m *RootModel) toggleJob(msg uisched.ToggleJobMsg) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		j, err := m.schedulerRepo.Get(ctx, msg.ID)
		if err != nil {
			return msgs.SessionErrorMsg{Err: fmt.Errorf("get job: %w", err)}
		}
		j.Enabled = msg.Enabled
		if err := m.schedulerRepo.Update(ctx, j); err != nil {
			return msgs.SessionErrorMsg{Err: fmt.Errorf("update job: %w", err)}
		}
		if msg.Enabled {
			_ = m.schedRunner.AddJob(*j)
		} else {
			m.schedRunner.RemoveJob(j.ID)
		}
		jobs, _ := m.schedulerRepo.List(ctx)
		pbs, _ := m.playbookRepo.List(ctx)
		names := make(map[int64]string, len(pbs))
		for _, p := range pbs {
			names[p.ID] = p.Name
		}
		return uisched.SchedulerLoadedMsg{Jobs: jobs, PlaybookNames: names}
	}
}

// ── View ──────────────────────────────────────────────────────────────────

func (m *RootModel) View() string {
	var body string
	switch m.activeScreen {
	case msgs.ScreenDashboard:
		body = m.viewDashboard()
	case msgs.ScreenTerminal:
		body = m.viewTerminal()
	case msgs.ScreenSessionList:
		body = m.sessionList.View()
	case msgs.ScreenSessionForm:
		body = m.sessionForm.View()
	case msgs.ScreenBroadcast:
		body = m.broadcastMdl.View()
	case msgs.ScreenScheduler:
		if m.schedFormOpen {
			body = m.schedForm.View()
		} else {
			body = m.schedList.View()
		}
	case msgs.ScreenAuditLog:
		body = m.auditMdl.View()
	case msgs.ScreenAIPanel:
		body = m.aiPanel.View()
	case msgs.ScreenPlaybooks:
		switch {
		case m.pbRunning:
			body = m.pbRunView.View()
		case m.pbFormOpen:
			body = m.pbForm.View()
		default:
			body = m.pbList.View()
		}
	default:
		body = fmt.Sprintf("  [%v — coming soon]", m.activeScreen)
	}

	if m.pendingApproval != nil {
		overlay := StyleWarning.Render(fmt.Sprintf(
			"\n  APPROVAL REQUIRED: %s\n  [y] approve  [n] reject\n",
			m.pendingApproval.PlanSummary))
		body = lipgloss.JoinVertical(lipgloss.Left, body, overlay)
	}

	return lipgloss.JoinVertical(lipgloss.Left,
		m.viewTabBar(),
		body,
		m.viewStatusBar(),
	)
}

func (m *RootModel) viewTabBar() string {
	if len(m.tabs) == 0 {
		screenName := screenLabel(m.activeScreen)
		return StyleMuted.Render(fmt.Sprintf("  termi  │  %s  │  ctrl+b s=sessions  ctrl+b q=quit", screenName))
	}
	var parts []string
	for i, t := range m.tabs {
		label := fmt.Sprintf(" %d:%s ", i+1, t.Name)
		if i == m.activeTab {
			parts = append(parts, StyleTabActive.Render(label))
		} else {
			parts = append(parts, StyleTabInactive.Render(label))
		}
	}
	return lipgloss.JoinHorizontal(lipgloss.Left, parts...)
}

func (m *RootModel) viewTerminal() string {
	if len(m.tabs) == 0 {
		return "  No active sessions.  Press ctrl+b s to open the session list."
	}
	return m.tabs[m.activeTab].View()
}

func (m *RootModel) viewDashboard() string {
	activeLine := StyleMuted.Render(fmt.Sprintf("  %d session(s) open", len(m.tabs)))
	if len(m.tabs) > 0 {
		activeLine = StyleSuccess.Render(fmt.Sprintf("  ● %d session(s) open  —  ctrl+b tab to cycle", len(m.tabs)))
	}
	return lipgloss.JoinVertical(lipgloss.Left,
		"",
		StyleTitle.Render("  termi"),
		"",
		"  ctrl+b s  — sessions",
		"  ctrl+b n  — new session",
		"  ctrl+b p  — playbooks (Ansible)",
		"  ctrl+b S  — scheduler",
		"  ctrl+b a  — audit log",
		"  ctrl+b A  — AI panel",
		"  ctrl+b b  — broadcast",
		"  ctrl+b q  — quit",
		"",
		activeLine,
	)
}

func (m *RootModel) viewStatusBar() string {
	var msg string
	if m.errMsg != "" {
		msg = StyleError.Render("  ✗ " + m.errMsg)
	} else if m.statusMsg != "" {
		msg = "  " + m.statusMsg
	} else {
		hint := "ctrl+b = menu"
		if m.activeScreen == msgs.ScreenTerminal {
			hint = "ctrl+b = menu  │  pgup/pgdn = scrollback"
		}
		msg = StyleMuted.Render("  " + hint)
	}
	return StyleStatusBar.Width(m.width).Render(msg)
}

func screenLabel(s msgs.ScreenID) string {
	switch s {
	case msgs.ScreenDashboard:
		return "dashboard"
	case msgs.ScreenSessionList:
		return "sessions"
	case msgs.ScreenSessionForm:
		return "new session"
	case msgs.ScreenTerminal:
		return "terminal"
	case msgs.ScreenBroadcast:
		return "broadcast"
	case msgs.ScreenPlaybooks:
		return "playbooks"
	case msgs.ScreenScheduler:
		return "scheduler"
	case msgs.ScreenAuditLog:
		return "audit log"
	case msgs.ScreenAIPanel:
		return "AI panel"
	}
	return "unknown"
}

func listenForApprovals(r *scheduler.Runner) tea.Cmd {
	return func() tea.Msg {
		req := <-r.ApprovalCh()
		return ApprovalRequestMsg{Req: req}
	}
}
