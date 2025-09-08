# Docker Setup for CodeSession Discord Bot

This Discord bot uses Docker for easy deployment and consistent environments with persistent storage for `.sessions` and `.worktrees`.

## Quick Start

1. **Configure the bot**:
   ```bash
   cp config.example.toml config.toml
   # Edit config.toml with your Discord bot token and settings
   ```

2. **Create data directories**:
   ```bash
   mkdir -p data/sessions data/worktrees
   ```

3. **Run with Docker Compose**:
   ```bash
   # Local development
   docker-compose up -d

   # Production deployment
   docker-compose -f docker-compose.prod.yml up -d
   ```

## Configuration

The bot uses `config.toml` for all configuration (copied from `config.example.toml`):

- `bot_token`: Your Discord bot token
- `opencode_port`: Port for OpenCode integration (default: 5000)
- `log_level`: Logging level (debug/info/warn/error)
- `summarizer_instruction`: Custom commit summarizer prompt
- `[[model]]` and `[[repository]]` sections as needed

## Persistent Data

The following directories are persistent across container restarts:
- `.sessions/` - Bot session data
- `.worktrees/` - Git worktree data

## Backup

Optional backup service is included:
```bash
# Run backup manually
docker-compose --profile backup run --rm backup

# Production backup (automatic retention)
docker-compose -f docker-compose.prod.yml run --rm backup
```

## Building

The Docker image is built automatically in GitHub Actions and pushed to GitHub Container Registry on releases.

Manual build:
```bash
docker build -t codesession .
```