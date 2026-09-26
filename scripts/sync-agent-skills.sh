#!/usr/bin/env bash
# sync-agent-skills.sh: Ensures skills are shared symmetrically across Antigravity and Cursor.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
GLOBAL_GEMINI_SKILLS="$HOME/.gemini/config/skills"
GLOBAL_CURSOR_SKILLS="$HOME/.cursor/skills-cursor"
WORKSPACE_AGENTS_SKILLS="$REPO_ROOT/.agents/skills"
WORKSPACE_CURSOR_SKILLS="$REPO_ROOT/.cursor/skills"

mkdir -p "$GLOBAL_GEMINI_SKILLS" "$GLOBAL_CURSOR_SKILLS" "$WORKSPACE_AGENTS_SKILLS" "$WORKSPACE_CURSOR_SKILLS"

echo "Syncing canonical skills from $GLOBAL_GEMINI_SKILLS..."
for skill_dir in "$GLOBAL_GEMINI_SKILLS"/*; do
  [ -d "$skill_dir" ] || continue
  skill_name="$(basename "$skill_dir")"
  
  # Link into Workspace .agents/skills (Antigravity / cross-agent root)
  ln -sfn "$skill_dir" "$WORKSPACE_AGENTS_SKILLS/$skill_name"
  
  # Link into Workspace .cursor/skills (Cursor project root)
  if [ "$skill_name" != "ui-ux-pro-max" ] || [ ! -d "$WORKSPACE_CURSOR_SKILLS/ui-ux-pro-max" ]; then
    ln -sfn "$skill_dir" "$WORKSPACE_CURSOR_SKILLS/$skill_name"
  fi
  
  # Link into Global Cursor skills (~/.cursor/skills-cursor)
  ln -sfn "$skill_dir" "$GLOBAL_CURSOR_SKILLS/$skill_name"
done

echo "Skills successfully unified across Antigravity, Cursor, and .agents!"
