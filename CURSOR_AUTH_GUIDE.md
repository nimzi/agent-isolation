# Cursor Agent CLI Authentication Guide

## How Cursor Agent CLI Authentication Works

Cursor Agent CLI uses credentials from your **host machine's Cursor installation**. There is no login command - credentials are mounted directly from your host.

### Step 1: Prerequisites

**You must have Cursor installed on your host machine:**
- Install Cursor editor: https://cursor.sh
- Sign in to Cursor on your host machine
- Credentials are automatically stored in `$HOME/.config/cursor`

### Step 2: Verify Host Credentials

Check that Cursor config exists on your host:
```bash
ls -la $HOME/.config/cursor/
```

You should see files like:
- `settings.json`
- `User/` directory
- Other Cursor configuration files

### Step 3: Container Setup

The `ai-shell recreate` command automatically mounts your credentials:
```bash
-v $HOME/.config/cursor:/root/.config/cursor:ro
```

This mounts your host's Cursor config directory to the container's config location.

### Step 4: Verify in Container

After creating the container, verify credentials are accessible:
```bash
ai-shell up --home "$(pwd)"
ai-shell check --home "$(pwd)"
```

**What `ai-shell check` verifies:** it confirms `cursor-agent` is installed and that `/root/.config/cursor` is present/readable inside the container. It does not guarantee the directory contains valid credentials (for that, ensure your host Cursor is signed in and the host directory is populated).

**Tip:** Use `ai-shell ls` and copy the `SHORT` id; you can target a specific instance with `ai-shell enter <short>`. Prefixes must be unique; if not, `ai-shell` will print an “ambiguous target” error with candidates.

### How It Works

1. **Host Machine**: Cursor stores credentials in `$HOME/.config/cursor/`
2. **Container**: This directory is mounted to `/root/.config/cursor/` (read-only)
3. **CLI**: Cursor Agent CLI reads credentials from `/root/.config/cursor/`
4. **No Login Needed**: Credentials are automatically available

### Important Notes

- ✅ **No API keys needed** - Uses your existing Cursor installation
- ✅ **No login command** - Credentials come from host mount
- ✅ **Read-only mount** - Container can't modify your host credentials
- ✅ **Automatic** - Works as long as Cursor is configured on host

### Troubleshooting

**If Cursor CLI doesn't work:**

1. **Check host credentials exist:**
   ```bash
   ls -la $HOME/.config/cursor/
   ```
   If empty or missing, sign in to Cursor on your host first.

2. **Verify mount in container:**
   ```bash
   ai-shell ls
   ai-shell enter <short>
   # inside the container:
   ls -la /root/.config/cursor/
   ```
   Should show the same files as on host.

3. **Check container creation:**
   ```bash
   ai-shell status --home "$(pwd)"
   ```
   Should show the cursor config mount and the derived container name.

4. **Verify Cursor is installed on host:**
   - Make sure Cursor editor is installed and you're signed in
   - The CLI uses the same credentials as the editor

### Manual Container Creation

If creating manually, don't forget the mount (use unique names per workdir):
```bash
docker run -d \
    --name ai-agent-shell-<id> \
    -v $(pwd):/work \
    -v ai_agent_shell_home_<id>:/root \
    -v $HOME/.config/cursor:/root/.config/cursor:ro \
    --env-file ~/.config/ai-shell/.env \
    ai-agent-shell
```

Note: the env file is optional; you can also authenticate inside the container with `gh auth login` and then run `/docker/setup-git-ssh.sh`.
