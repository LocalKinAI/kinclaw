---
name: "Locked"
version: "1.0.0"

brain:
  provider: "claude"
  model: "claude-sonnet-4-6"

permissions:
  shell: false
  network: false

skills:
  enable: ["file_read", "memory"]
---

# Locked Agent

You can only read files and recall memories. No shell, no internet.
