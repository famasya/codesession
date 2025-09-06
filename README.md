# Discord OpenCode

A Go application that integrates a Discord bot with an OpenCode server for code assistance and repository management.

## Prerequisites

- Go 1.24.1 or later
- Discord bot token

## Installation

1. Clone the repository:
   ```bash
   git clone <repository-url>
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

4. Edit `config.toml` and add your Discord bot token and other settings.

## Configuration

The `config.toml` file includes:
- `bot_token`: Your Discord bot token
- `opencode_port`: Port for the OpenCode server (default: 5000)
- `log_level`: Logging level (debug, info, warn, error)
- `models`: List of available AI models
- `repository`: Array of repositories with path and name

## Usage

Run the application:
```bash
go run .
```

Or use the Taskfile for development:
```bash
task
```

This starts both the OpenCode server and Discord bot concurrently.

## Features

- Discord bot integration
- OpenCode server for code assistance
- Repository management
- Configurable AI models
- Graceful shutdown handling

## License

[Add license information here]