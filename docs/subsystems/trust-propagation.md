---
status: skeleton
module: backend/internal/shared/kernel
derives-from:
  - margince-poc/docs/subsystems/trust-propagation.md @ a11d6c08
  - specs/spec/narrative/05-agent-security.md#trust-model-provenance-tiers @ 5a0b29c
  - specs/spec/narrative/03e-overlay-augmentation.md#34-how-f-1-trust-tiers--egress-applies-to-incumbent-data @ 5a0b29c
---
# Trust propagation — the untrusted label travels with the data, end to end

> Every fact the system holds carries a trust tier, and the riskiest kind — data that
> came from outside — keeps its untrusted label all the way from capture through storage
> to whatever an agent produces. The agent, and the person reading its output, always
> knows which facts are unverified and must be treated as information to weigh, never as
> instructions to obey.

## What it's for

Records that arrive through capture or through an overlay mirror were written somewhere
the product doesn't control, so an attacker could have planted text in them. That text
must never be treated as a command to an agent. This chapter owns the **propagation
mechanics**: where the label attaches, which seams carry it, and the rule that it can
never be quietly lost or upgraded along the way. The tier *definitions* — [[threat-model#T0]]
system-trusted, [[threat-model#T1]] human-entered, [[threat-model#T2]] captured or
external, data-never-instructions — are owned by the threat-model chapter and are not
restated here. Its callers are every seam that moves records: the datasource seam
(incumbent-source reads), the retrieval seam (search), the related-context assembler,
and the agent tool-output path.

## Principles it serves

- **P6 — embrace the LLMs, safely.** Untrusted content that reaches an agent arrives
  clearly labelled and flagged, so the model treats it as data rather than as something
  to obey.
- **P12 — governance designed in.** The untrusted origin of outside content stays
  visible from one end of the pipeline to the other and cannot be quietly lost along
  the way.

## How it works

The trust tier is never stored as its own column, so there is no saved level that could
drift out of date. It is worked out at the moment a record is read, from provenance the
record already carries: a record served from an overlay mirror is T2 simply because the
workspace is in overlay mode, a record with capture provenance is T2 because a connector
wrote it, and a record a person entered natively is T1. From that single point of origin
the tier travels along with the record, and it survives every hop:

- **Search and retrieval.** A record that entered through an overlay source or the
  capture pipeline keeps its T2 tier as it passes through keyword search, similarity
  matching, and scans.
- **Building up related context.** When the system assembles a picture of related
  records and the evidence behind them, each piece carries its tier along. The tier is
  purely a label here: it does not change how results are scored or ranked. (This is
  deliberately separate from the source-confidence weighting that *does* affect ranking.)
  A test pins the rankings to be identical to what they were before tiers were added,
  proving the label changes nothing about ordering.
- **Derivation never launders.** Anything *built from* labelled inputs — a summary, an
  assembled dossier, an enrichment — inherits the weakest tier among its inputs. One T2
  ingredient makes the whole derived artifact T2. There is no step anywhere in the
  pipeline that upgrades a tier; promotion is a deliberate human act and out of scope
  here.
- **Handing content to an agent — the warning.** When a tool, or the agent-facing
  response path, returns a result containing T2 records, it wraps that result so the
  agent receives the content, an explicit untrusted marker, and a short warning to treat
  the content as data and follow no instructions inside it — the labeling convention the
  threat model owns as [[threat-model#D1]]. Results made only of trusted content are
  returned unchanged.

The danger this guards against is the trifecta the threat-model chapter names: private
data, plus untrusted outside content, plus tools that act. Carrying the label all the
way through is the propagation half of the defense; the agent is always told which
content it cannot trust.

## What's configurable

Nothing an operator sets. A record's tier follows automatically from its provenance —
overlay mode or capture origin; there are no knobs to turn.

## Guarantees (enforced)

- **Carried end to end.** The tier survives every hop. The headline check is a leak
  test: a tool call that returns external-origin content must always carry the T2 label,
  and dropping it anywhere along the way turns the test red (TRUST-AC-1).
- **Never laundered.** A derived artifact inherits the weakest tier of its inputs; no
  path upgrades T2 to anything more trusted (TRUST-AC-2).
- **A label, not a ranking factor.** Tier never changes how search or related-context
  results are scored or ordered (TRUST-AC-3).
- **Worked out, not stored.** No dedicated tier column exists and no migration is
  needed; the tier is derived from provenance at the moment of reading.

The label and its warning are a **published convention, not a guarantee this subsystem
enforces on its own**: for agents built by third parties we can attach the untrusted
marker but cannot control how someone else's agent reasons about it
([[threat-model#TM-RESID-1]]). The real backstop against a hidden instruction is the
outbound egress controls, owned by the threat model as [[threat-model#D3]].

## Acceptance

Done means an observer can follow one externally originated fact through the whole
pipeline and see the untrusted marker at every stop: captured or mirrored in, stored
with its provenance, read back as T2, and delivered to an agent wrapped and warned. A
summary built over mixed inputs reads as untrusted whenever any ingredient was. Results
made only of trusted content arrive unwrapped — no false alarms. The testable form of
each claim is pinned in the Acceptance appendix; the cross-cutting floor is inherited
from the acceptance-standards chapter.

## Out of scope

The tier definitions and the labeling/egress doctrine (threat-model chapter, T0/T1/T2,
[[threat-model#D1]], [[threat-model#D3]]); the content-aware outbound gate; audit-stream
anomaly watching; enforcing that third-party agents honor the label; a human flow for
promoting captured records to trusted; per-field sensitivity classification; and trust
tagging on the native write path. This chapter covers only carrying trust through the
read pipeline, the never-launder rule for derived artifacts, and the leak test.

## Where it lives

The trust vocabulary lives in the dependency-free kernel (backend/internal/shared/kernel),
so the two seams that must carry it — datasource and retrieval — can both depend on it
without depending on each other. The related-context assembler and the agents module's
tool-output path carry it through. Read next: the threat-model chapter for the tiers and
the defense stack, and the intent-tools chapter for how compositions keep the label.

## Appendix

### Acceptance
Source: margince-poc/docs/subsystems/trust-propagation.md#guarantees-enforced @ a11d6c08; specs/spec/narrative/05-agent-security.md#verification-stage-6-gate--new-critical @ 5a0b29c

| ID | Given/When/Then | Verification |
|---|---|---|
| TRUST-AC-1 | Given a record of external origin (overlay-mirrored or capture-provenance), when it travels capture → store → read → tool output, then the [[threat-model#T2]] label is present at every hop and on the final tool result, with the untrusted marker and warning attached. | Round-trip leak test in the backend integration lane; enforced by the tier-leak gate [[threat-model#TM-VERIFY-2]]. |
| TRUST-AC-2 | Given a derived artifact (summary, assembled context, enrichment) built from mixed-tier inputs, when any input is T2, then the artifact is labelled T2 — derivation inherits the weakest input tier and never launders trust. | Weakest-input property test on the assembly and derivation paths. |
| TRUST-AC-3 | Given search and related-context results with tiers attached, when scored and ordered, then ranking is identical to the pre-label baseline — the tier is a label, never a ranking factor. | Ranking-parity golden test in the retrieval lane. |
| TRUST-AC-4 | Given a tool result containing only T0/T1 content, when returned to an agent, then it is returned unchanged — no untrusted wrapper, no warning. | Tool-output marshalling unit test. |
