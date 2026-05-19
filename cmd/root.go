package cmd

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/cyberslacks/termi/internal/app"
	"github.com/cyberslacks/termi/internal/audit"
	"github.com/cyberslacks/termi/internal/config"
	"github.com/cyberslacks/termi/internal/creds"
	"github.com/cyberslacks/termi/internal/playbook"
	"github.com/cyberslacks/termi/internal/scheduler"
	"github.com/cyberslacks/termi/internal/ssh"
	"github.com/cyberslacks/termi/internal/store"
)

var (
	dataDir      string
	openAIURL    string
	openAIModel  string
	openAIKey    string
)

var rootCmd = &cobra.Command{
	Use:   "termi",
	Short: "SSH terminal manager with AI agent automation",
	Long: `termi — TUI SSH terminal manager.

Features: session management, credential storage, broadcast commands,
Ansible playbook automation, AI-assisted management (Claude + Ollama + OpenAI-compatible), and
scheduled autonomous or interactive job execution.

Environment variables:
  ANTHROPIC_API_KEY  — enables Claude AI panel
  OLLAMA_HOST        — Ollama server URL (default: http://localhost:11434)
  OPENAI_BASE_URL    — OpenAI-compatible endpoint (OpenWebUI, LiteLLM, vLLM, Groq…)
  OPENAI_API_KEY     — bearer token for the OpenAI-compatible endpoint
  TERMI_DATA_DIR     — override data directory`,
	RunE: func(cmd *cobra.Command, args []string) error {
		return run(cmd)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&dataDir, "data-dir", "", "data directory (default ~/.local/share/termi)")
	rootCmd.Flags().StringVar(&openAIURL, "openai-url", "", "OpenAI-compatible base URL (e.g. http://localhost:3000)")
	rootCmd.Flags().StringVar(&openAIModel, "openai-model", "", "model name for the OpenAI-compatible endpoint")
	rootCmd.Flags().StringVar(&openAIKey, "openai-key", "", "API key / bearer token for the OpenAI-compatible endpoint")
}

func run(cmd *cobra.Command) error {
	ctx := context.Background()

	// Load config
	cfg, err := config.Load(dataDir)
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	// CLI flags override config/env
	if cmd.Flags().Changed("openai-url") {
		cfg.AI.OpenAIBaseURL = openAIURL
	}
	if cmd.Flags().Changed("openai-model") {
		cfg.AI.OpenAIModel = openAIModel
	}
	if cmd.Flags().Changed("openai-key") {
		cfg.AI.OpenAIAPIKey = openAIKey
	}

	// Open database
	db, err := store.Open(cfg.DataDir)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}

	// Wire repositories
	sessionRepo := store.NewSessionRepo(db)
	playbookRepo := store.NewPlaybookRepo(db)
	schedulerRepo := store.NewSchedulerRepo(db)
	auditRepo := store.NewAuditRepo(db)

	// Wire services
	credStore := creds.New()
	sshMgr := ssh.NewManager()
	auditLog := audit.New(auditRepo)
	defer auditLog.Close()

	ansibleExec := playbook.NewExecutor()

	schedRunner := scheduler.New(schedulerRepo, playbookRepo, sessionRepo, ansibleExec, auditLog)
	if err := schedRunner.Start(ctx); err != nil {
		return fmt.Errorf("start scheduler: %w", err)
	}
	defer schedRunner.Stop()

	// Build and run the TUI
	model := app.New(cfg, app.Services{
		SSHMgr:        sshMgr,
		CredStore:     credStore,
		SessionRepo:   sessionRepo,
		PlaybookRepo:  playbookRepo,
		SchedulerRepo: schedulerRepo,
		AuditRepo:     auditRepo,
		AnsibleExec:   ansibleExec,
		SchedRunner:   schedRunner,
		AuditLog:      auditLog,
	})

	p := tea.NewProgram(model,
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	model.SetProgram(p)

	_, err = p.Run()
	return err
}
