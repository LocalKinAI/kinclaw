---
name: translate
description: Translate text between languages using the system's translate command
command: [echo, "{{text}}"]
schema:
  text:
    type: string
    description: Text to translate
    required: true
  target:
    type: string
    description: Target language (e.g. English, Chinese, Spanish)
    required: true
---
# Translate

Passes text to the LLM for translation. The target language is provided as context.
