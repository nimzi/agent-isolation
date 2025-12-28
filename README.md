# OpenAI in Docker

An AI-powered interactive shell assistant that runs commands inside a Docker container, powered by OpenAI's API.

## Features

- **AI-powered command execution**: Uses OpenAI API to propose shell commands based on your goals
- **Safe execution**: Confirmation prompts for potentially destructive operations
- **Containerized environment**: Commands run in an isolated Docker container
- **Persistent state**: Container state is saved in Docker volumes
- **Cursor Agent CLI**: Includes Cursor Agent CLI for additional AI capabilities

## Setup

### 1. Environment Variables

Create a `.env` file in this directory with your API keys:

```bash
OPENAI_API_KEY=sk-your-openai-api-key
```

**Cursor Agent CLI Authentication:**

Cursor Agent CLI uses credentials from your host machine's Cursor installation. Authentication works by mounting your Cursor config directory:

1. **Prerequisites:**
   - You must have Cursor installed and configured on your host machine
   - Cursor credentials are stored in `$HOME/.config/cursor` on your host

2. **Automatic mounting:**
   - The `recreate-container.sh` script automatically mounts `$HOME/.config/cursor` to `/root/.config/cursor` in the container
   - This gives the container access to your Cursor authentication

3. **Manual setup:**
   ```bash
   docker run -d \
       --name openai-shell \
       -v $(pwd):/work \
       -v openai_shell_home:/root \
       -v $HOME/.config/cursor:/root/.config/cursor \
       --env-file .env \
       openai-shell
   ```

4. **Note:** No API keys or login commands needed. The container uses your existing Cursor credentials from the host machine.

### 2. Build and Run

**First time setup:**
```bash
./recreate-container.sh
```

This script will:
- Build the Docker image (includes Node.js, npm, and Cursor Agent CLI)
- Create a container with your environment variables
- Mount your project directory to `/work`
- Create a persistent volume for `/root` (home directory)
- Mount your Cursor credentials from `$HOME/.config/cursor` to `/root/.config/cursor`

**Or manually:**
```bash
# Build the image
docker build -t openai-shell .

# Create and start the container
docker run -d \
    --name openai-shell \
    -v $(pwd):/work \
    -v openai_shell_home:/root \
    -v $HOME/.config/cursor:/root/.config/cursor \
    --env-file .env \
    openai-shell
```

### 3. Container Control Script

The `container-control.sh` script provides a convenient way to start or stop the Docker container without rebuilding it. This is useful for managing the container lifecycle after initial setup.

**Usage:**
```bash
# Start the container
./container-control.sh start

# Stop the container
./container-control.sh stop
```

**Features:**
- Checks if the container exists before attempting to start/stop
- Verifies container state to avoid unnecessary operations
- Provides clear error messages if the container doesn't exist or operations fail
- Respects the `OAI_SHELL_CONTAINER` environment variable for custom container names

**Note:** If the container doesn't exist, you'll need to run `./recreate-container.sh` first to create it.

### 4. Use the AI Shell

```bash
python3 ai_docker_shell.py
```

## Authentication

### OpenAI API
- Uses `OPENAI_API_KEY` from `.env` file
- Passed to container as environment variable

### Cursor Agent CLI
- **No API key required** - Uses credentials from host machine
- Credentials mounted from `$HOME/.config/cursor` on host to `/root/.config/cursor` in container
- Requires Cursor to be installed and configured on your host machine
- Credentials are read-only mounted (changes in container don't affect host)

## Container State

The container persists data in two locations:

1. **Project directory** (`/work`): Files created here appear in your local directory
2. **Docker volume** (`openai_shell_home` → `/root`): Home directory, configs, and installed packages

**Important:** When rebuilding the image, your volume data persists. Use `./recreate-container.sh` to rebuild while preserving your data.

## Usage

1. Run `python3 ai_docker_shell.py`
2. Enter your goal (e.g., "install Node.js and create a hello world script")
3. Review and approve commands one at a time
4. Continue or change goals as needed

## Configuration

Environment variables (can be set in `.env` or as container env vars):

- `OAI_SHELL_CONTAINER`: Container name (default: `openai-shell`)
- `OAI_MODEL`: OpenAI model to use (default: `gpt-5`)
- `OAI_MAX_OUTPUT_CHARS`: Max output length (default: `12000`)
- `OPENAI_API_KEY`: Your OpenAI API key (required)
- Cursor Agent CLI: No environment variables needed - uses mounted credentials from `$HOME/.config/cursor`
