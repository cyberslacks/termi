package scheduler

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/cyberslacks/termi/internal/store"
)

// jobItem wraps store.ScheduledJob to satisfy list.Item.
type jobItem struct {
	job          store.ScheduledJob
	playbookName string
}

func (j jobItem) Title() string {
	enabled := lipgloss.NewStyle().Foreground(lipgloss.Color("#10B981")).Render("● ")
	if !j.job.Enabled {
		enabled = lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render("○ ")
	}
	return enabled + j.job.Name
}

func (j jobItem) Description() string {
	mode := string(j.job.Mode)
	pbName := j.playbookName
	if pbName == "" {
		pbName = fmt.Sprintf("playbook #%d", j.job.PlaybookID)
	}
	var last string
	if j.job.LastRunAt != nil {
		last = "  last: " + humanDuration(time.Since(*j.job.LastRunAt))
	}
	var next string
	if j.job.NextRunAt != nil && j.job.Enabled {
		next = "  next: " + humanDuration(time.Until(*j.job.NextRunAt))
	}
	return fmt.Sprintf("%s  [%s]  %s%s%s", j.job.CronExpr, mode, pbName, last, next)
}

func (j jobItem) FilterValue() string {
	return j.job.Name + " " + j.playbookName
}

type listKeyMap struct {
	newJob key.Binding
	edit   key.Binding
	delete key.Binding
	toggle key.Binding
	back   key.Binding
}

func newListKeyMap() listKeyMap {
	return listKeyMap{
		newJob: key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "new")),
		edit:   key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "edit")),
		delete: key.NewBinding(key.WithKeys("d"), key.WithHelp("d", "delete")),
		toggle: key.NewBinding(key.WithKeys("t"), key.WithHelp("t", "enable/disable")),
		back:   key.NewBinding(key.WithKeys("esc"), key.WithHelp("esc", "back")),
	}
}

// ListModel is the scheduled-job browser.
type ListModel struct {
	list          list.Model
	keys          listKeyMap
	playbookNames map[int64]string
	width, height int
	loaded        bool
	confirmDel    *store.ScheduledJob
	err           string
}

func NewListModel(width, height int) ListModel {
	keys := newListKeyMap()

	delegate := list.NewDefaultDelegate()
	delegate.Styles.SelectedTitle = delegate.Styles.SelectedTitle.
		Foreground(lipgloss.Color("#F9FAFB")).
		Background(lipgloss.Color("#7C3AED")).
		Bold(true)
	delegate.Styles.SelectedDesc = delegate.Styles.SelectedDesc.
		Foreground(lipgloss.Color("#C4B5FD")).
		Background(lipgloss.Color("#7C3AED"))

	l := list.New(nil, delegate, width, height-4)
	l.Title = "Scheduled Jobs"
	l.Styles.Title = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#7C3AED"))
	l.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{keys.newJob, keys.edit, keys.delete, keys.toggle}
	}
	l.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{keys.newJob, keys.edit, keys.delete, keys.toggle, keys.back}
	}

	return ListModel{list: l, keys: keys, width: width, height: height}
}

// LoadCmd fetches jobs and playbook names from the DB.
func LoadCmd(jobRepo *store.SchedulerRepo, pbRepo *store.PlaybookRepo) tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		jobs, err := jobRepo.List(ctx)
		if err != nil {
			return SchedulerLoadedMsg{Err: err}
		}
		pbs, _ := pbRepo.List(ctx)
		names := make(map[int64]string, len(pbs))
		for _, p := range pbs {
			names[p.ID] = p.Name
		}
		return SchedulerLoadedMsg{Jobs: jobs, PlaybookNames: names}
	}
}

func (m ListModel) Init() tea.Cmd { return nil }

func (m ListModel) Update(msg tea.Msg) (ListModel, tea.Cmd) {
	switch msg := msg.(type) {
	case SchedulerLoadedMsg:
		if msg.Err != nil {
			m.err = msg.Err.Error()
			return m, nil
		}
		m.playbookNames = msg.PlaybookNames
		items := make([]list.Item, len(msg.Jobs))
		for i, j := range msg.Jobs {
			items[i] = jobItem{job: j, playbookName: msg.PlaybookNames[j.PlaybookID]}
		}
		cmd := m.list.SetItems(items)
		m.loaded = true
		m.err = ""
		return m, cmd

	case tea.KeyMsg:
		if m.confirmDel != nil {
			switch msg.String() {
			case "y", "Y":
				id := m.confirmDel.ID
				m.confirmDel = nil
				return m, func() tea.Msg { return DeleteJobMsg{ID: id} }
			default:
				m.confirmDel = nil
				return m, nil
			}
		}

		switch {
		case key.Matches(msg, m.keys.back):
			return m, func() tea.Msg { return JobFormCancelledMsg{} }

		case key.Matches(msg, m.keys.newJob):
			return m, func() tea.Msg { return OpenJobFormMsg{Job: nil} }

		case key.Matches(msg, m.keys.edit):
			if j, ok := m.selectedJob(); ok {
				cp := j
				return m, func() tea.Msg { return OpenJobFormMsg{Job: &cp} }
			}

		case key.Matches(msg, m.keys.delete):
			if j, ok := m.selectedJob(); ok {
				m.confirmDel = &j
				return m, nil
			}

		case key.Matches(msg, m.keys.toggle):
			if j, ok := m.selectedJob(); ok {
				return m, func() tea.Msg { return ToggleJobMsg{ID: j.ID, Enabled: !j.Enabled} }
			}
		}
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m ListModel) View() string {
	if m.err != "" {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#EF4444")).Render("  Error: " + m.err)
	}
	if !m.loaded {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("#6B7280")).Render("  Loading jobs…")
	}

	view := m.list.View()

	if m.confirmDel != nil {
		overlay := lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#F59E0B")).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#F59E0B")).
			Padding(1, 3).
			Render(fmt.Sprintf("Delete job %q? [y] yes  [any] cancel", m.confirmDel.Name))
		view = lipgloss.JoinVertical(lipgloss.Left, view, "", overlay)
	}

	return view
}

func (m *ListModel) SetSize(w, h int) {
	m.width = w
	m.height = h
	m.list.SetSize(w, h-4)
}

func (m ListModel) selectedJob() (store.ScheduledJob, bool) {
	item, ok := m.list.SelectedItem().(jobItem)
	if !ok {
		return store.ScheduledJob{}, false
	}
	return item.job, true
}

func humanDuration(d time.Duration) string {
	d = d.Abs()
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

// ensure strings is used
var _ = strings.TrimSpace
