# Contributing to Silo

Hey, thanks for wanting to contribute!

Let's be real: this project is built almost entirely with AI assistance. Claude, Codex, whatever you've got — we're not pretending otherwise. But there's a big difference between *using AI well* and *submitting AI slop*. We care a lot about the first one, and we'll push back hard on the second.

- AI-generated code is fine. AI slop is not.
- You are responsible for everything you submit, even if an AI wrote it.
- Actually read the code. Actually run the tests. Actually understand what it does.
- Don't send big changes without talking about them first.

## Things Are Moving Fast

Silo is in heavy active development. Features get rewritten, APIs shift, whole sections get reworked — sometimes day to day. **If you want to work on something, reach out first.** Open an issue or drop a message so:

- You don't build on something that's already been rewritten locally but not pushed yet.
- I can avoid breaking something you're actively working on.
- We can check the area is stable enough to be worth building on right now.

## Before You Start

Small stuff (typo fixes, minor bugs) — just open a merge request. No ceremony.

For anything bigger, start with an issue first. New features, API changes, schema migrations, large refactors, behavior changes — talk about it before writing code. Design docs live under `docs/superpowers/specs/` and `docs/superpowers/plans/` if you need examples.

## Don't Submit AI Slop

AI writes most of the code here. That's fine. What's not fine is copy-pasting output without understanding it.

1. **Read every line of your diff.** If you can't explain it, don't submit it.
2. **Run the tests** — but don't blindly trust them. The tests were also AI-written and they have blind spots.
3. **Test the app yourself locally.** Spin it up, click around, try the thing you changed. There is no substitute for this.
4. **Run code-review** on your own work before submitting. `superpowers:requesting-code-review` or whatever tooling you have.
5. **Watch for AI-introduced bugs.** Silent behavior changes, dead code, subtle regressions — look for them.
6. **Understand the bigger picture.** A change that looks fine in isolation can break something three layers away.

"The AI suggested it" is not an acceptable answer in review. You should be able to explain the reasoning and tradeoffs.

## Merge Requests

A good MR answers: what problem does this solve, why this approach, how was it tested, anything to watch out for. For non-trivial changes, link the issue and note any migration/compatibility concerns. Screenshots for UI changes.

Small, well-explained MRs get reviewed fast. Big unexplained ones sit.

If AI meaningfully shaped the implementation, say so in the description. Not gatekeeping — just helps reviewers understand intent.

## Development Setup

See the README for full setup. Common checks:

```sh
go test ./...                        # Go tests
golangci-lint run                    # Go lint
cd web && bun test                   # Frontend tests
cd web && bun run lint               # Frontend lint
cd web && bun run format:check       # Frontend formatting
```

If your change spans `Silo` and `silo-plugin-sdk`, local iteration through [`go.work`](go.work) is expected. Do not rely on that workspace in repo-tracked config or release pipelines. CI validates this repo with `GOWORK=off`, and any new SDK package or symbol must come from a pushed, tagged `github.com/Silo-Server/silo-plugin-sdk` release before the change is ready to merge.

## Style

- One thing per MR. Don't mix unrelated changes.
- Follow existing patterns.
- Comments for non-obvious things only.

## For AI Agents

If you're an LLM working on this codebase: read `CLAUDE.md` (or `AGENTS.md` when available) for project-specific instructions, architecture reference, and verification requirements. The rules in this file apply to you too — especially the parts about not submitting slop and running verification before declaring work complete.

## Be Realistic

Opening a merge request doesn't create an obligation on my side. I might close it, ignore it, ask you to shrink it, or reimplement the idea myself later. The codebase is moving fast and sometimes the best response to a good PR is "thanks, but I already went a different direction."

If you're fine with that, welcome aboard.

If you're not sure whether something is in scope, open an issue and ask. Always better than building something that needs to be reshaped.
