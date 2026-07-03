# ADR-0046 — AI-assisted contribution policy: human accountability, DCO, and disclosure for the open-source project

**Status:** Accepted (2026-06-25, founder). Recorded as **DECISIONS A61**. Governs *external* contributions
to the source-available project; the internal-build counterpart is ADR-0045 (the craftsmanship gate).
Composes with ADR-0029 (BUSL license), ADR-0010 (secure SDLC), P3/P12/P14. Specifies `CONTRIBUTING.md` and
`.github/PULL_REQUEST_TEMPLATE.md` to be authored in the build repo.

## Context

Margince is source-available (BUSL-1.1, ADR-0029) and invites public contribution — it is the top of the
consulting funnel (P14). Open-sourcing AI-built code in 2026 lands in a known crisis: maintainers across the
ecosystem (OCaml, QEMU, NetBSD, tldraw) are **drowning in low-quality AI pull requests** — verbose,
plausible-looking, unexplainable by their submitters, and in volumes that break the trust model of code
review (the reviewer can no longer assume the contributor understands what they submitted). Some projects
have banned AI contributions outright, citing the **Developer Certificate of Origin** (a contributor can't
certify provenance of code they didn't write and don't understand) and GPL-contamination risk.

There is an asymmetry to name explicitly: **internally, Margince's own code is AI-authored by design** (the
dark factory, A39/ADR-0002 Am.1), so an "is this AI?" disclosure rule is *moot* for our own commits — internal
trust is carried by the craftsmanship gate (ADR-0045) plus human seam-owners (CODEOWNERS, doc 11 §6). But the
moment the project is public, **external** contributors arrive under no such governance, and the same slop
flood will hit our maintainers. We need a policy that is honest about our own AI use while holding external AI
contributions to the bar the OpenSSF guidance defines.

## Decision

**Adopt an AI-assisted contribution policy built on human accountability, DCO sign-off, and proportionate
disclosure — applied to external contributors, with the craftsmanship gate (ADR-0045) as the automated first
line for everyone.** Codified in `CONTRIBUTING.md` + an anti-slop PR template.

**1. Human accountability (the non-negotiable).** A human contributor **must understand, and be able to
explain on request, every line they submit** — regardless of how it was produced. "An agent wrote it" is not
an answer to a review question. A maintainer may **close any PR whose author cannot explain it**, with a
pointer to what would make it acceptable. AI is treated as a junior collaborator: the human owns the result,
the tests, the edge cases.

**2. DCO sign-off (provenance).** Every commit carries a `Signed-off-by` line (DCO 1.1). The contributor
certifies they have the right to submit the code under the project license — which they cannot honestly do
for code they neither wrote nor understand (this is the lever, per part 1).

**3. Proportionate disclosure.** Meaningful AI assistance (substantial generation, not trivial autocomplete)
is **disclosed** in the PR — which tool, what it shaped — via the PR template. This is for reviewer context
and legal clarity, not stigma; trivial completion needs no disclosure. (Our *own* internal commits are
exempt — AI authorship is the standing assumption there; §5.)

**4. The craftsmanship gate is the automated first line (ADR-0045).** External PRs hit the same gate as
internal ones: deterministic gates → craftsmanship Critic → human acceptance. Slop is blocked mechanically
before it reaches a maintainer's attention, which is the structural answer to the volume problem ("if review
doesn't scale, build better verification, not faster reviewers"). External false positives use the same
`CRAFT-DISPUTE` → adjudication channel.

**5. The internal/external asymmetry, stated.** Internal agent-authored commits: **no disclosure** (AI
authorship is assumed), trust via ADR-0045 + seam-owner review. External contributions: parts 1–4 apply in
full. Both paths converge on the *same craftsmanship bar* — the gate does not care who wrote the code.

**6. The anti-slop PR template.** `.github/PULL_REQUEST_TEMPLATE.md` requires, before a PR is reviewable:
*what changed · why · how it was verified (tests/commands) · AI involvement (tool + scope, or "none/trivial")
· an explicit "I can explain every line of this change" acknowledgement.* The template makes the
accountability and disclosure rules a structural step, not a cultural hope.

## Consequences

- **Positive:** protects maintainer attention (the scarce resource) against the documented AI-PR flood;
  preserves the review trust model (accountability + DCO); gives the project a clear, friendly-but-firm public
  stance ("AI-assisted is welcome; unexplainable slop is not") that reads as competent rather than hostile.
  Honest about our own AI use, so the policy isn't hypocritical.
- **Negative / honest limits:** (a) disclosure and DCO are **self-attested** — enforceable only at review,
  not provable up front; the craftsmanship gate + the "explain it" challenge are the real teeth. (b) The
  "explain every line" bar will deter some drive-by contributors — *intended*, given P14 (the funnel wants
  serious contributors and consulting leads, not PR volume). (c) BUSL/GPL-contamination review remains a
  **legal** judgment (counsel), not something this policy or the gate resolves — flagged, not closed here.
- **Relationship to other decisions:** sibling of **ADR-0045** (same standard, external surface); composes
  with **ADR-0029** (BUSL — the license the DCO certifies against), **ADR-0010** (the SDLC the gate sits in),
  **P14** (the consulting funnel the contribution path feeds). Does not weaken any locked decision.
- **Scope:** `CONTRIBUTING.md` + `.github/PULL_REQUEST_TEMPLATE.md` (build repo); DCO bot in CI; folded into
  the ADR-0045 platform epic. The legal contamination review is a separate counsel gate (tracked in
  `BACKLOG.md`).
