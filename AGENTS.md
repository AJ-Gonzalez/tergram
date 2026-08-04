# AGENTS.md

Guidance for AI agents and contributors working in this repository. This file is
authoritative; when in doubt, follow it over inline convenience.

## HARD RULE — no forced-choice questions

DO NOT use the multiple-choice / question-widget tool (`ask`, the question widget, or
any equivalent that forces the user to pick from preset options).

- Ask questions in an open, free-text format instead — present the options or context in
  prose and let the user answer naturally.
- Never force a selection. The user must always be free to answer off-list or with their
  own phrasing.
- This applies to clarifying questions during implementation, design decisions, and any
  interactive prompt.

Rationale: forced multiple-choice hides viable off-list answers and biases the user
toward preset options. Open questions preserve the full decision space.

## Git Rules

Use the local `dumbpush.sh` script, give the commit message in quotes as an argument. 

Example

`$ ./dumbpush.sh "Commit message here"`

Only trigger the github actions workflow when there is a tag. 
Otherwise make a local build for the Linux target. 
