package bot

import (
	"log/slog"

	"github.com/Johnnycyan/cyan-birthdays/internal/config"
	"github.com/Johnnycyan/cyan-birthdays/internal/database"
	"github.com/bwmarrin/discordgo"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Bot represents the Discord bot
type Bot struct {
	session *discordgo.Session
	config  *config.Config
	repo    *database.Repository
	pool    *pgxpool.Pool
	stopCh  chan struct{}
}

// New creates a new Bot instance
func New(cfg *config.Config, pool *pgxpool.Pool) (*Bot, error) {
	session, err := discordgo.New("Bot " + cfg.DiscordToken)
	if err != nil {
		return nil, err
	}

	// Set intents
	session.Identify.Intents = discordgo.IntentsGuilds | discordgo.IntentsGuildMembers

	return &Bot{
		session: session,
		config:  cfg,
		repo:    database.NewRepository(pool),
		pool:    pool,
		stopCh:  make(chan struct{}),
	}, nil
}

// Start connects to Discord and registers commands
func (b *Bot) Start() error {
	// Register handlers
	b.session.AddHandler(b.handleReady)
	b.session.AddHandler(b.handleInteraction)

	// Open connection
	if err := b.session.Open(); err != nil {
		return err
	}

	return nil
}

// Stop gracefully shuts down the bot
func (b *Bot) Stop() error {
	close(b.stopCh)
	
	// Unregister commands
	if err := b.unregisterCommands(); err != nil {
		slog.Warn("Failed to unregister commands", "error", err)
	}

	return b.session.Close()
}

// handleReady is called when the bot connects to Discord
func (b *Bot) handleReady(s *discordgo.Session, r *discordgo.Ready) {
	slog.Info("Bot is ready", "user", r.User.Username, "guilds", len(r.Guilds))

	// Register slash commands
	if err := b.registerCommands(); err != nil {
		slog.Error("Failed to register commands", "error", err)
	}

	// Start birthday loop
	go b.startBirthdayLoop()
}

// handleInteraction routes interactions to the appropriate handler
func (b *Bot) handleInteraction(s *discordgo.Session, i *discordgo.InteractionCreate) {
	switch i.Type {
	case discordgo.InteractionApplicationCommand:
		b.handleCommand(s, i)
	case discordgo.InteractionApplicationCommandAutocomplete:
		b.handleAutocomplete(s, i)
	case discordgo.InteractionModalSubmit:
		b.handleModalSubmit(s, i)
	case discordgo.InteractionMessageComponent:
		b.handleComponent(s, i)
	}
}
