# Contributing to Margince

Margince is open source and AI-native. We welcome contributions — held to the same
craftsmanship bar as our own AI-authored code (ADR-0045/0046).

## Human accountability

**You are accountable for every line you submit, and must be able to explain every line.** AI
assistance is welcome and expected; unexplainable, slop-flooded contributions are not. If you
cannot explain why a line is there, what it does, and why it is correct, it is not ready.

This is the project's one non-negotiable. It is why we ask for the disclosures below — not to
discourage AI use, but to keep a human answerable for the result.

## AI disclosure

Disclose AI involvement proportionately in the PR (the template asks for it):

- **Assisted** — you wrote/directed it with AI help (autocomplete, review, refactor). The default.
- **Generated** — AI produced substantial portions you then reviewed and own.

There is a deliberate **internal/external asymmetry**: Margince's own build agents author by
design and do not disclose per-PR (it is the stated practice, A39); external contributors disclose
so a human reviewer knows what they are accountable for. Either way, the **same craftsmanship gate**
applies (below).

## Developer Certificate of Origin (DCO)

Every commit must be signed off, certifying you have the right to submit it under the project
license (see [developercertificate.org](https://developercertificate.org)):

```
git commit -s -m "your message"
```

This adds a `Signed-off-by: Your Name <you@example.com>` trailer. The **DCO check is required** —
a commit without a sign-off blocks the merge. Amend with `git commit --amend -s` if you forget.

## The craftsmanship gate

Your PR runs the **same gate as our internal PRs** — the automated first line of review:

1. The deterministic gates (`make check`) must be green.
2. The **craftsmanship reviewer** then reviews your diff against
   [docs/quality/craftsmanship.md](docs/quality/craftsmanship.md). On a high-confidence, objective BLOCK it writes
   `CRAFT-FIX[<id>]` markers into your branch; fix the code and delete the marker (the residue gate
   keeps the merge blocked until you do).
3. If a finding is genuinely wrong, replace its marker with `CRAFT-DISPUTE[<id>]: <reasoning>` — it
   routes that one finding to **human adjudication** (it is not a merge override).

There is no override path on a craftsmanship BLOCK; the gate is calibrated to block only on
high-confidence objective slop, so a block means the code needs a change, not an argument.

## Before you open a PR

- Read [AGENTS.md](AGENTS.md) `## Craftsmanship` and run its pre-submit self-check.
- Keep the PR scoped and let it tell a story (title + what/why/how-verified).
- `make check` is green and every commit is signed off.
