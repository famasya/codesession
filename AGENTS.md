## CodeSession - Agent Guidelines

This is a Discord bot that integrates with OpenCode, allowing users to start AI coding sessions through Discord. The bot creates git worktrees for isolated development environments and manages OpenCode sessions for collaborative coding.

## Architecture

- **Main Components**:
  - `main.go`: Entry point that runs both Discord bot and OpenCode server concurrently
  - `discord.go`: Discord bot setup, slash command registration, and message handling
  - `interaction-handlers.go`: Discord slash command handlers (`/ping`, `/opencode`, `/plan`, `/commit`)
  - `opencode-server.go`: OpenCode server management and lifecycle
  - `opencode-client.go`: OpenCode client integration, session management, and event streaming
  - `opencode-event-types.go`: Type definitions for OpenCode event handling
  - `config.go`: TOML configuration loading and management

- **Core Features**:
  - Discord slash commands for starting OpenCode sessions (`/opencode` for build mode, `/plan` for planning mode)
  - Git worktree creation for isolated development environments
  - Real-time OpenCode event streaming to Discord threads
  - Automatic commit generation with AI summaries
  - Session persistence and recovery
  - Agent-based interactions (build agent for full development, plan agent for analysis)

## Build and Development Commands

- **Build**: `go build -o ./tmp/main .`
- **Run with hot reload**: `air` (recommended for development)
- **Run with task runner**: `task` (equivalent to `air`)
- **Run directly**: `go run .`
- **Test**: `go test ./...` (no tests currently exist)
- **Format**: `go fmt ./...`

## Configuration

The application requires a `config.toml` file (excluded from git). Use `config.example.toml` as a template:

```toml
bot_token = "your_discord_bot_token"
opencode_port = 5000
log_level = "debug"
summarizer_instruction = ""  # Optional custom AI commit summarizer

[[model]]
provider_id = "openrouter"
model_id = "z-ai/glm-4.5"

[[repository]]
path = "/path/to/repository"
name = "repository_name"
```

## Session Management

- Sessions are stored in `.sessions/` directory as JSON files
- Worktrees are created in `.worktrees/` directory
- Each Discord thread corresponds to one OpenCode session
- Sessions persist across bot restarts and can be lazy-loaded

## Key Dependencies

- `github.com/bwmarrin/discordgo`: Discord API client
- `github.com/sst/opencode-sdk-go`: OpenCode integration
- `github.com/BurntSushi/toml`: Configuration parsing
- `github.com/goombaio/namegenerator`: Random thread naming

## Code Patterns

- Use structured logging with `slog` package
- Error handling with explicit `if err != nil` checks
- Concurrent operations use `sync.WaitGroup` and `context.Context`
- Thread-safe operations use `sync.RWMutex` for session management
- Git operations are executed via `os/exec` Command

## Agent System

The bot supports two OpenCode agents with different capabilities:

### Build Agent (Default)
- **Command**: `/opencode`
- **Capabilities**: Full development access with all tools enabled
- **Use case**: General coding, implementation, and development tasks
- **Tools**: write, edit, bash, read, grep, glob, etc.

### Plan Agent
- **Command**: `/plan`
- **Capabilities**: Read-only analysis and planning mode
- **Use case**: Code review, planning, analysis without making changes
- **Tools**: Limited to read-only operations (read, grep, glob)

### Agent Selection
- Use `/opencode` for full development sessions (uses "build" agent)
- Use `/plan` for planning and analysis sessions (uses "plan" agent)
- Mention the bot (@bot) in existing threads to continue with the agent that was used to start the session

## Directory Structure

- `.worktrees/`: Git worktrees for isolated development (excluded from git)
- `.sessions/`: Session data persistence (excluded from git)
- `tmp/`: Build output directory (excluded from git)
