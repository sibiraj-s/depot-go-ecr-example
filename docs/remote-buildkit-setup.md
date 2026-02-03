# Remote BuildKit Setup Guide

This guide covers setting up an remote machine as a remote BuildKit builder for use with SSH connections.

## Prerequisites

- Remote machine running Ubuntu (tested on Ubuntu 22.04/24.04)
- SSH access with key-based authentication
- Security group allowing SSH (port 22) from your IP

## Setup Steps

### 1. Install BuildKit

```bash
# Download BuildKit (check https://github.com/moby/buildkit/releases for latest version)
BUILDKIT_VERSION="v0.27.1"
wget https://github.com/moby/buildkit/releases/download/${BUILDKIT_VERSION}/buildkit-${BUILDKIT_VERSION}.linux-amd64.tar.gz

# Extract to /usr/local
sudo tar -xzf buildkit-${BUILDKIT_VERSION}.linux-amd64.tar.gz -C /usr/local

# Verify installation
buildctl --version
```

### 2. Create Docker Group (if not exists)

```bash
sudo groupadd docker
sudo usermod -aG docker $USER
```

### 3. Set Up systemd Service

```bash
sudo tee /etc/systemd/system/buildkit.service > /dev/null <<EOF
[Unit]
Description=BuildKit
Documentation=https://github.com/moby/buildkit
After=network.target

[Service]
ExecStart=/usr/local/bin/buildkitd --group docker
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
```

### 4. Enable and Start Service

```bash
sudo systemctl daemon-reload
sudo systemctl enable buildkit
sudo systemctl start buildkit
```

### 5. Verify BuildKit is Running

```bash
# Check service status
sudo systemctl status buildkit

# Test buildctl connection
buildctl debug info
```

### 6. Fix Socket Permissions (if needed)

If you get permission errors:

```bash
# Option A: Make socket world-readable (quick fix)
sudo chmod 666 /run/buildkit/buildkitd.sock

# Option B: Ensure your user is in docker group and re-login
groups  # Should show 'docker'
# If not, logout and login again, or run:
newgrp docker
```

## Usage

From your local machine, connect using:

```
ssh://ubuntu@<REMOTE_MACHINE_IP>
``` 

Example: `ssh://ubuntu@192.168.1.100`

## Troubleshooting

### "buildctl: command not found"

BuildKit is not installed. Follow step 1.

### "permission denied" on socket

Either:

- Run `sudo chmod 666 /run/buildkit/buildkitd.sock`
- Or ensure buildkitd is running with `--group docker` and your user is in the docker group

### "connection refused"

BuildKit daemon is not running:

```bash
sudo systemctl start buildkit
sudo systemctl status buildkit
```

### SSH connection fails

- Verify your SSH key is loaded: `ssh-add -l`
- Test SSH manually: `ssh ubuntu@<REMOTE_MACHINE_IP>`
- Check Remote machine security group allows port 22

## Optional: Persist Builds Across Restarts

By default, BuildKit stores build cache in `/var/lib/buildkit`. To preserve cache:

```bash
# Ensure the directory persists
sudo mkdir -p /var/lib/buildkit

# If using a remote machine, mount a volume to /var/lib/buildkit
```

## Quick Reference

| Command                           | Description             |
| --------------------------------- | ----------------------- |
| `sudo systemctl start buildkit`   | Start BuildKit          |
| `sudo systemctl stop buildkit`    | Stop BuildKit           |
| `sudo systemctl restart buildkit` | Restart BuildKit        |
| `sudo systemctl status buildkit`  | Check status            |
| `buildctl debug info`             | Test connection locally |
| `journalctl -u buildkit -f`       | View logs               |
