# GitHub publication drafts

These are local drafts, not published Issues or Pull Requests.

The current authorization was rejected by GitHub. Once the owner completes official login:

1. Read the remote repository's default branch, existing Issues, PRs and labels.
2. Push `codex/reliability-baseline` containing commit `0524ab6`.
3. Find or create the baseline Issue using its stable marker in `issues.json`; open the baseline PR with the actual Issue reference and the body from `baseline-pr.md`.
4. Push the separate workflow branch and publish the workflow Issue/PR, specifying the baseline dependency.
5. Publish remaining Issues in `issues.json` order, replacing draft dependency keys with actual Issue links. Preserve existing equivalent Issues rather than duplicating them.
6. Record actual URLs in this directory or the roadmap. An Issue remains open until its acceptance criteria are met and its PR is merged.

Check required CI before treating any PR as complete. The owner authorized project publishing; login is the remaining external dependency, not a request to approve the wording again.
