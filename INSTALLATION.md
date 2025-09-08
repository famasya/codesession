# Installation Guide

## Installation

1. **Create dedicated user and directories** (replace `your-user` with desired username):
   ```bash
   sudo useradd -r -s /bin/false -d /opt/codesession your-user
   sudo mkdir -p /opt/codesession/{.sessions,.worktrees}
   ```

2. **Download the latest release** for your platform:
   ```bash
   cd /opt/codesession
   sudo wget https://github.com/famasya/codesession/releases/latest/download/codesession-linux-amd64
   sudo chmod +x codesession-linux-amd64
   sudo mv codesession-linux-amd64 codesession
   ```

3. **Create configuration**:
   ```bash
   sudo wget https://raw.githubusercontent.com/famasya/codesession/main/config.example.toml -O /opt/codesession/config.toml
   sudo nano /opt/codesession/config.toml  # Edit with your Discord bot token and settings
   ```

4. **Install OpenCode** for the user:
   ```bash
   # Switch to the user and install OpenCode
   sudo -u your-user bash -c 'curl -fsSL https://opencode.ai/install.sh | sh'
   ```

   Note: you should set your provider auth by running `opencode auth login`.

5. **Set ownership** (replace `your-user` with the username you created):
   ```bash
   sudo chown -R your-user:your-user /opt/codesession
   ```

## Running as a Daemon (systemd)

1. **Create systemd service file**:
   ```bash
   sudo nano /etc/systemd/system/codesession.service
   ```

2. **Add this content** (replace `your-user` with the username you created):
   ```ini
   [Unit]
   Description=codesession Discord bot
   After=network.target
   Wants=network.target

   [Service]
   Type=simple
   User=your-user
   Group=your-user
   WorkingDirectory=/opt/codesession
   ExecStart=/opt/codesession/codesession
   Restart=always
   RestartSec=5
   StandardOutput=journal
   StandardError=journal

   # Environment - Include user's OpenCode installation
   Environment=PATH=/home/your-user/.opencode/bin:/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin

   [Install]
   WantedBy=multi-user.target
   ```

3. **Enable and start the service**:
   ```bash
   sudo systemctl daemon-reload
   sudo systemctl enable codesession
   sudo systemctl start codesession
   ```

4. **Check service status**:
   ```bash
   sudo systemctl status codesession
   ```

5. **View logs**:
   ```bash
   sudo journalctl -u codesession -f
   ```

## Configuration

Edit `/opt/codesession/config.toml` with your settings:

- `bot_token`: Your Discord bot token
- `opencode_port`: Port for OpenCode integration (default: 5000)  
- `log_level`: Logging level (debug/info/warn/error)
- `summarizer_instruction`: Custom commit summarizer prompt
- `[[models]]` and `[[repositories]]` sections as needed

## Management Commands

```bash
# Start service
sudo systemctl start codesession

# Stop service
sudo systemctl stop codesession

# Restart service
sudo systemctl restart codesession

# Check status
sudo systemctl status codesession

# View logs
sudo journalctl -u codesession -f

# Disable service (prevent auto-start)
sudo systemctl disable codesession
```

## Data Storage

Persistent data is stored in:
- `/opt/codesession/.sessions/` - Bot session data
- `/opt/codesession/.worktrees/` - Git worktree data

## Updating

1. Stop the service:
   ```bash
   sudo systemctl stop codesession
   ```

2. Download new version:
   ```bash
   cd /opt/codesession
   sudo wget https://github.com/famasya/codesession/releases/latest/download/codesession-linux-amd64
   sudo chmod +x codesession-linux-amd64
   sudo mv codesession-linux-amd64 codesession
   ```

3. Start the service:
   ```bash
   sudo systemctl start codesession
   ```
