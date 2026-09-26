# Issue tracker: Linear

Issues and specs for this repo live in Linear — team **Personal** (key `PER`), project **Pironman 5** (id `P-PER-1`). Use the `mcp__linear-server__*` tools for all Linear operations.

## Conventions

- **Create an issue**: `mcp__linear-server__save_issue` with `team: "Personal"`, `project: "Pironman 5"`, `title`, `description`.
- **Read an issue**: `mcp__linear-server__get_issue` by identifier (e.g. `PER-12`).
- **List issues**: `mcp__linear-server__list_issues`, filtered by `team: "Personal"` and `project: "Pironman 5"`, plus `state` as needed.
- **Comment on an issue**: `mcp__linear-server__save_comment`.
- **Apply/remove labels**: look up or create the label with `mcp__linear-server__list_issue_labels` / `create_issue_label`, then set it via `save_issue`.
- **Close**: `mcp__linear-server__save_issue` with `state` set to a Done/Canceled status (find the id via `mcp__linear-server__list_issue_statuses`).

## Pull requests

This repo has a GitHub remote (`dnitros/pironman`) and uses `gh` for PR operations — one branch and one PR per Linear ticket, using the ticket's `gitBranchName`.

**Don't duplicate status/QA/review-summary comments on the PR.** Linear is the source of truth for ticket history — post those there only. PR comments are for content a human reviewer would actually read inline on the diff, not recaps of what's already on the ticket.

## When a skill says "publish to the issue tracker"

Create a Linear issue via `save_issue`, scoped to team Personal / project Pironman 5.

## When a skill says "fetch the relevant ticket"

`mcp__linear-server__get_issue` by identifier.
