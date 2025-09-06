# Discord OpenCode Bot

A Discord bot that integrates with OpenCode to provide AI-assisted coding sessions directly in Discord threads.

## Features

- Discord slash commands for starting OpenCode sessions
- Automatic git worktree creation for isolated sessions
- Real-time messaging between Discord and OpenCode
- Support for multiple repositories and AI models
- Thread-based conversations for organized sessions

## Prerequisites

- Go 1.24.1 or later
- Discord bot token
- OpenCode CLI installed
- Git repositories configured

## Installation

1. Clone the repository:
   ```bash
   git clone https://github.com/yourusername/discord-opencode.git
   cd discord-opencode
   ```

2. Install dependencies:
   ```bash
   go mod download
   ```

3. Copy the example configuration:
   ```bash
   cp config.example.toml config.toml
   ```

## Configuration

Edit `config.toml` with your settings:

- `bot_token`: Your Discord bot token
- `opencode_port`: Port for OpenCode server (default: 5000)
- `log_level`: Logging level (debug, info, warn, error)
- `models`: Array of available AI models
- `repository`: Array of repositories with path and name

## Usage

1. Run the bot:
   ```bash
   go run .
   ```

   Or use the task runner:
   ```bash
   task
   ```

2. In Discord, use `/opencode` to start a session:
   - Select a repository
   - Choose an AI model
   - A new thread will be created with an OpenCode session

3. Mention the bot in the thread to send messages to OpenCode

## Commands

- `/ping`: Test bot responsiveness
- `/opencode`: Start a new OpenCode session

## Development

The project uses:
- [discordgo](https://github.com/bwmarrin/discordgo) for Discord integration
- [opencode-sdk-go](https://github.com/sst/opencode-sdk-go) for OpenCode client
- [air](https://github.com/cosmtrek/air) for hot reloading

## License

[Add your license here]