---
name: git_commit
description: Stage all changes and create a git commit with a message
command: [sh, -c, "git add -A && git commit -m '{{message}}'"]
schema:
  message:
    type: string
    description: The commit message
    required: true
timeout: 15
---
# Git Commit

Stages all changes and commits with the given message.
