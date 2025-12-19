# Cyan Birthdays

A Discord birthday bot written in Go. Set birthdays, get announcements with custom messages and roles, with per-user timezone support.

## Features

- 🎂 **Birthday Management**: Users set their birthday with `/birthday set`
- 🌍 **Timezone Support**: Per-user timezones so announcements happen at midnight *in their timezone*
- 🎭 **Custom Roles**: Automatic birthday role assignment/removal
- 📢 **Custom Messages**: Configurable messages with placeholders (`{mention}`, `{name}`, `{new_age}`)
- 🔒 **Subscriber Gating**: Optional required role for birthday announcements
- 🔍 **Upcoming Birthdays**: View who has birthdays coming up

## Quick Start

### Docker Compose (Recommended)

1. Copy the example files:
   ```bash
   cp docker-compose.yml.example docker-compose.yml
   cp .env.example .env
   ```

2. Edit `.env` with your Discord bot token:
   ```env
   DISCORD_TOKEN=your_bot_token_here
   POSTGRES_USER=birthday
   POSTGRES_PASSWORD=your_secure_password
   POSTGRES_DB=birthday
   ```

3. Start the bot:
   ```bash
   docker compose up -d
   ```

### Binary

1. Download the latest release from [Releases](https://github.com/Johnnycyan/cyan-birthdays/releases)
2. Set environment variables:
   ```bash
   export DISCORD_TOKEN=your_bot_token
   export DATABASE_URL=postgres://user:password@localhost:5432/birthday?sslmode=disable
   ```
3. Run the binary:
   ```bash
   ./cyan-birthdays-linux-amd64
   ```

## Commands

### User Commands (`/birthday`)

| Command | Description |
|---------|-------------|
| `/birthday set` | Set your birthday (opens a modal) |
| `/birthday remove` | Remove your birthday |
| `/birthday upcoming [days]` | View upcoming birthdays |

### Admin Commands (`/bdset`)

| Command | Description |
|---------|-------------|
| `/bdset interactive` | Start the setup wizard |
| `/bdset channel` | Set the announcement channel |
| `/bdset role` | Set the birthday role |
| `/bdset time` | Set the announcement hour (0-23) |
| `/bdset msgwithyear` | Set message for birthdays with age |
| `/bdset msgwithoutyear` | Set message for birthdays without age |
| `/bdset rolemention` | Toggle role mentions in messages |
| `/bdset requiredrole` | Set a role required for announcements |
| `/bdset defaulttimezone` | Set default timezone for users |
| `/bdset force` | Force-set a user's birthday |
| `/bdset settings` | View current settings |
| `/bdset stop` | Clear all settings |

## Message Placeholders

- `{mention}` - @mention the user
- `{name}` - User's display name
- `{new_age}` - User's new age (only for messages with year)

**Example messages:**
```
{mention} has turned {new_age}, happy birthday! 🎂
Happy birthday {name}! 🎉
```

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `DISCORD_TOKEN` | Yes | Discord bot token |
| `DATABASE_URL` | Yes | PostgreSQL connection string |
| `OWNER_ID` | No | Bot owner Discord ID |
| `LOG_LEVEL` | No | Logging level (debug/info/warn/error) |

## Development

### Build from source

```bash
# Clone the repository
git clone https://github.com/Johnnycyan/cyan-birthdays.git
cd cyan-birthdays

# Install dependencies
go mod download

# Build
go build -o cyan-birthdays ./cmd/bot

# Run
./cyan-birthdays
```

### Project Structure

```
├── cmd/bot/            # Application entry point
├── internal/
│   ├── bot/            # Discord bot logic
│   ├── config/         # Configuration loading
│   ├── database/       # PostgreSQL repository
│   └── timezone/       # Timezone handling
├── Dockerfile
├── docker-compose.yml.example
└── .github/workflows/  # CI/CD
```

## License

MIT
