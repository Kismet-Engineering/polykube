# Agent Working Agreement

This file applies to the entire repository. Follow `CONTRIBUTING.md` for project conventions and use these instructions for agent-driven implementation work.

## Working Style

- Keep each task focused on the requested issue or concern. Do not expand into adjacent backlog items without approval.
- Prefer the smallest correct change that satisfies the issue exit criteria.
- Inspect the relevant code before proposing or making changes. Use targeted searches and symbol-level reads instead of repeatedly reading large files.
- Preserve existing architecture and behavior unless the issue explicitly changes them.
- If implementation exposes an unclear product choice, conflicting requirements, or an unexpected architectural finding, stop and ask one concise, batched clarification question before choosing a direction.

## Implementation Loop

1. Confirm the worktree state and read the issue, named files, and relevant tests.
2. Identify behavioral forks once and ask for clarification together.
3. Implement one logical slice directly.
4. Run the narrowest relevant tests for that slice.
5. Create a local atomic commit when the slice is coherent and tested.
6. Repeat for additional slices without mixing unrelated changes.
7. Run one final review and the full repository validation before pushing.
8. Push the validated commits, post evidence, and close the issue only when all exit criteria pass.

Do not push partially validated commits unless the user explicitly requests incremental pushes.

## Subagents

- Do not use an exploration subagent when the issue already identifies a small set of files or symbols. Inspect those files directly.
- Use subagents only for genuinely broad discovery, independent workstreams, or a final second opinion where delegation saves time.
- Do not delegate work that duplicates analysis already in progress.
- For a change of ordinary size, use at most one review subagent and scope it to the final diff, not the whole subsystem.
- Run independent review and validation work concurrently where possible.
- Treat subagent findings as review input. Verify whether a finding is newly introduced, in scope, and consistent with confirmed product decisions before acting on it.

## Commits And GitHub Issues

- Make atomic commits organized by behavior, tests, generated artifacts, or documentation when those are independently reviewable.
- Run targeted tests before each commit. Run `bash scripts/validate-repo.sh` before the final push.
- When API types or kubebuilder markers change, run `mise run operator:generate` and commit generated artifacts with their source change.
- Update an active issue only when there is meaningful evidence: implementation started and decisions recorded, a validated milestone pushed, or final verification completed.
- Keep issue comments concise. Include commit links, commands run, results, and any remaining work.
- Do not claim completion or close an issue before the full validation gate passes.
- Before the final report, confirm the worktree is clean and the pushed branch matches its remote.

## Tests And Documentation

- Test observable behavior and recovery, not implementation details.
- Cover refusal to mutate unowned objects whenever ownership is part of the controller contract.
- Prefer compact test helpers or table-driven cases when they reduce repetition without hiding intent.
- Update user-facing documentation when conditions, recovery behavior, permissions, API semantics, or operational boundaries change.
- Document limitations honestly; do not imply production readiness for alpha behavior.
