#!/usr/bin/env python3
import os
import subprocess
import sys
from typing import Tuple

from dotenv import load_dotenv
from openai import OpenAI

# Load environment variables from .env in the current working directory (or parents if configured)
load_dotenv()

CONTAINER_NAME = os.environ.get("OAI_SHELL_CONTAINER", "openai-shell")
MODEL = os.environ.get("OAI_MODEL", "gpt-5")  # change if desired
MAX_OUTPUT_CHARS = int(os.environ.get("OAI_MAX_OUTPUT_CHARS", "12000"))

SYSTEM_PROMPT = """You are a careful terminal assistant.
You will propose ONE shell command at a time to run inside a Docker container.
Rules:
- Return only the command text (no backticks, no explanation).
- Prefer safe, readable commands.
- If a command could be destructive (rm, mv, dd, mkfs, chmod -R, chown -R, etc.), ask for confirmation by outputting: CONFIRM: <command>
- Use /work as the working directory when relevant.
- If you need multiple steps, output the next best single command.
"""

def run_in_container(cmd: str) -> Tuple[int, str]:
    """
    Run a bash command inside the container and return (exit_code, combined_output).
    Uses 'bash -lc' so PATH, shell expansions, and chaining work as expected.
    """
    full = ["docker", "exec", "-i", CONTAINER_NAME, "bash", "-lc", cmd]
    p = subprocess.run(full, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    out = p.stdout or ""
    if len(out) > MAX_OUTPUT_CHARS:
        out = "\n[...output truncated...]\n" + out[-MAX_OUTPUT_CHARS:]
    return p.returncode, out

def ensure_container_running() -> None:
    p = subprocess.run(
        ["docker", "inspect", "-f", "{{.State.Running}}", CONTAINER_NAME],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
    )
    if p.returncode != 0 or "true" not in (p.stdout or ""):
        details = (p.stderr or p.stdout or "").strip()
        raise RuntimeError(
            f"Container '{CONTAINER_NAME}' is not running.\n"
            f"Start it with:\n"
            f"  docker start {CONTAINER_NAME}\n"
            f"Or recreate it with your docker run command.\n"
            f"Details: {details}"
        )

def ask_model(goal: str, transcript: str) -> str:
    """
    Ask the model for the next single shell command to run.
    The model must return ONLY the command (or CONFIRM: <command>).
    """
    client = OpenAI()
    resp = client.responses.create(
        model=MODEL,
        input=[
            {"role": "system", "content": SYSTEM_PROMPT},
            {
                "role": "user",
                "content": (
                    f"Goal:\n{goal}\n\n"
                    f"Transcript so far:\n{transcript}\n\n"
                    f"Next command (single command only):"
                ),
            },
        ],
    )
    return (resp.output_text or "").strip()

def main() -> None:
    # Verify API key exists (loaded from .env or environment)
    key = os.environ.get("OPENAI_API_KEY", "").strip()
    if not key:
        print(
            "Missing OPENAI_API_KEY.\n"
            "Create a .env file in this directory containing:\n"
            "  OPENAI_API_KEY=sk-...\n",
            file=sys.stderr,
        )
        sys.exit(2)

    ensure_container_running()

    print(f"Using container: {CONTAINER_NAME}")
    print("This tool will propose one command at a time and run it inside the container.")
    print("Type 'exit' at any prompt to quit.\n")

    goal = input("Goal> ").strip()
    if not goal or goal.lower() == "exit":
        return

    transcript = ""
    while True:
        cmd = ask_model(goal, transcript)

        if not cmd:
            print("\nModel returned an empty command. Stopping.")
            return

        # If model requests confirmation for potentially destructive ops
        if cmd.startswith("CONFIRM:"):
            proposed = cmd[len("CONFIRM:"):].strip()
            ans = input(f"\nProposed (needs confirm): {proposed}\nRun it? [y/N] ").strip().lower()
            if ans != "y":
                transcript += f"\nMODEL> {cmd}\nUSER> declined\n"
                continue
            cmd = proposed

        print(f"\n$ {cmd}")
        rc, out = run_in_container(cmd)
        print(out, end="" if out.endswith("\n") else "\n")
        print(f"[exit={rc}]")

        transcript += f"\n$ {cmd}\n{out}\n[exit={rc}]\n"

        nxt = input("\nEnter to continue, or type a new goal, or 'exit'> ").strip()
        if nxt.lower() == "exit":
            return
        if nxt:
            goal = nxt

if __name__ == "__main__":
    main()
