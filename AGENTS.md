# GitHub Copilot Instructions

- Avoid AI sycophancy. Even if there could be multiple views or the user gets easily offended, at least express a mild doubt

## Coding Conventions
- Do not remove comments you did not add
- Avoid Speculative Safety: evaluate code by context, not hypothetical reuse (e.g. python aliasing is acceptable if it works)
- No Premature Generalization
- When updating the repository, check and update documentation to reflect any changes in architecture or conventions
  * But keep it concise. Document nuances that are not obvious from scanning the code, but skip trivia or restating implementation details visible in the modules
  * Prefer inline documentation over separate files, except when those files already exist or explicitly requested

## Complex tasks approach
- Create a `tasks/<name>.md` file with:
  1. End goal
    * Do not update the goal: if the goal shifts/does not match anymore for any reason, create a new task file
  2. Starting state, constraints (but do not reiterate what you got from `AGENTS.md` or skills)
  3. Failed attempts: what has been tried already (if applicable). This should be updated as you learn more, and must be updated after each failed attempt
  3. Step-by-step plan, each step with a verifiable outcome.
- After implementing each step, mark it as "verifying" and test (or ask the user to test with exact instructions).
  * If it works, mark it as "done", deduplicate failed attempts and plan, then proceed to the next step.
  * If it doesn't work:
    1. Update the failed attempts, remove the failed step from the plan
    2. Undo changes from the failed step
- When starting/continuing a task, review the file and re-analyze the repository contents to understand the context and avoid repeating past mistakes, and to check if the file is up-to-date
- ONLY when the task is done AND you got the user's confirmation, append the summary to the file. If the task appears incomplete but the task file has summary, move new information from it (if any) to failed attempts to avoid repeating the mistake, and remove the summary.
