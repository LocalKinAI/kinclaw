---
name: summarize
description: Summarize a file's contents into key bullet points
command: [cat, "{{path}}"]
schema:
  path:
    type: string
    description: Path to the file to summarize
    required: true
---
# Summarize

Reads a file and returns its contents for the LLM to summarize.
