#!/usr/bin/env bash
# PreToolUse hook for the Bash tool.
# Blocks dev-server launches and state-changing git commands per
# .claude/CLAUDE.md, which already says these require explicit user
# confirmation.
set -euo pipefail

cmd="$(jq -r '.tool_input.command // empty')"

[ -z "$cmd" ] && exit 0

deny() {
    echo "BLOCKED by project hook: $1" >&2
    echo "This project's CLAUDE.md requires explicit user confirmation for this action. Ask the user to run it, or confirm with them first." >&2
    exit 2
}

if echo "$cmd" | grep -qE '(^|[;&|]) *go run\b'; then
    deny "launching a dev process via 'go run' is not allowed on your own initiative."
fi

if echo "$cmd" | grep -qE 'npm run start-dev\b'; then
    deny "launching the frontend dev server via 'npm run start-dev' is not allowed on your own initiative."
fi

if echo "$cmd" | grep -qE '(^|[;&|]) *(npx +)?vite\b'; then
    deny "launching vite directly is not allowed on your own initiative."
fi

if echo "$cmd" | grep -qE '\bmise run start(-[A-Za-z0-9_-]*)?\b'; then
    deny "launching a dev process via 'mise run start*' is not allowed on your own initiative."
fi

if echo "$cmd" | grep -qE '\bgit\b(\s+-[A-Za-z-]+(\s+\S+)?)*\s+(commit|push|checkout|merge|rebase|reset|stash)\b'; then
    deny "this is a state-changing git command (commit/push/checkout/merge/rebase/reset/stash)."
fi

if echo "$cmd" | grep -qE '\bgit\s+worktree\s+(add|remove)\b'; then
    deny "git worktree add/remove changes repository state."
fi

if echo "$cmd" | grep -qE '\bgit\s+branch\b.*-D\b'; then
    deny "force-deleting a branch (git branch -D) is a destructive git command."
fi

exit 0
