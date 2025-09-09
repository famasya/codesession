## codesession - Agent Guidelines

This is a Discord bot that integrates with [Opencode](https://opencode.ai), allowing users to start AI coding sessions through Discord. The bot creates git worktrees for isolated development environments and manages Opencode sessions for collaborative coding.

## Architecture

- **Main Components**:
  - `main.go`: Entry point that runs both Discord bot and OpenCode server concurrently
  - `discord.go`: Discord bot setup, slash command registration, and message handling
  - `interaction-handlers.go`: Discord slash command handlers (`/ping`, `/codesession`, `/commit`)
  - `opencode-server.go`: OpenCode server management and lifecycle
  - `opencode-client.go`: OpenCode client integration, session management, and event streaming
  - `opencode-event-types.go`: Type definitions for OpenCode event handling
  - `config.go`: JSONC configuration loading and management

- **Core Features**:
  - Discord slash commands for starting Opencode sessions
  - Git worktree creation for isolated development environments
  - Real-time Opencode event streaming to Discord threads
  - Automatic commit generation with AI summaries
  - Session persistence and recovery

## Build and Development Commands

- **Build**: `go build -o ./tmp/main .`
- **Run with hot reload**: `air` (recommended for development)
- **Run with task runner**: `task` (equivalent to `air`)
- **Run directly**: `go run .`
- **Test**: `go test ./...` (no tests currently exist)
- **Format**: `go fmt ./...`

## Configuration

The application requires a `config.jsonc` file (excluded from git). Use `config.example.jsonc` as a template:

```jsonc
{
  "bot_token": "your_discord_bot_token",
  "opencode_port": 5000,
  "log_level": "debug",
  "summarizer_instruction": "", // Optional custom AI commit summarizer
  "models": [
    {
      "provider_id": "openrouter",
      "model_id": "z-ai/glm-4.5"
    }
  ],
  "repositories": [
    {
      "path": "/path/to/repository",
      "name": "repository_name"
    }
  ]
}
```

## Session Management

- Sessions are stored in `.sessions/` directory as JSON files
- Worktrees are created in `.worktrees/` directory
  - Each Discord thread corresponds to one Opencode session
- Sessions persist across bot restarts and can be lazy-loaded

## Key Dependencies

- `github.com/bwmarrin/discordgo`: Discord API client
- `github.com/sst/opencode-sdk-go`: OpenCode integration
- Standard library `encoding/json` with comment stripping: JSONC configuration parsing
- `github.com/goombaio/namegenerator`: Random thread naming

## Code Patterns

- Use structured logging with `slog` package
- Error handling with explicit `if err != nil` checks
- Concurrent operations use `sync.WaitGroup` and `context.Context`
- Thread-safe operations use `sync.RWMutex` for session management
- Git operations are executed via `os/exec` Command

## Directory Structure

- `.worktrees/`: Git worktrees for isolated development (excluded from git)
- `.sessions/`: Session data persistence (excluded from git)
- `tmp/`: Build output directory (excluded from git)
