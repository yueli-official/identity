# Repository instructions

- When adjacent `../workspace` exists and its `repos.lock.yaml` includes this repository, read and follow
  `../workspace/docs/multi-project-development.md` before cross-repository work or any local process lifecycle action. Product
  sessions treat Workspace contracts and `.doctor/` as read-only, use the Workspace CLI for generated state, use a separate Git
  worktree for concurrent writes to this repository, and never own shared Provider processes.
- Route long-running or resumable work through `flightdeck/deck.md` and the focused `flightdeck/work/*/index.md`; keep the Markdown handoff aligned with repository reality.
