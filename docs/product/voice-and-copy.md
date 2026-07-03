---
derives-from:
  - margince specs/spec/narrative/voice-and-tone-spec.md
  - margince specs/spec/narrative/customization-portability.md#6-corrected-external-claim-wording
---
# Voice & copy — every generated string is product surface

This chapter governs **every user-facing string a worker generates**: button labels,
empty states, error messages, emails the product drafts or sends, onboarding copy,
and marketing surfaces inside the product. Tone is normative here for the same
reason schema shapes are normative elsewhere — in a factory-built product, copy is
not hand-polished afterward; whatever register the workers write in *is* the product
the user meets. A wrong string is a defect like a wrong column.

The register is DACH B2B. The audience is a German buying committee — CRO, CISO,
DPO, CIO/CTO, CFO, plus the Betriebsrat — that trusts precision and data and
distrusts hype and drama. The voice: factual, short, precise, confident. The fact
carries the weight, not the wording. Two failure modes are rejected equally:
American hype (adjectives, urgency, slogans) and humble wordiness (hedging tails,
over-explaining). The four core rules plus the mechanics are pinned as
VOICE-RULE-1..5, and the claim discipline as VOICE-RULE-6..10.

Copy also reads machine-written when it carries the recognizable AI tics — reflexive
tricolons, "X, not Y" constructions, empty punchy kickers, uniform cadence. The
strip list is pinned verbatim (VOICE-STRIP-1..6); workers strip these from generated
copy before it ships.

Honesty over hype is not just taste — it is the same doctrine the screens obey. The
honest-states floor (STATE-1..5, owned by the acceptance-standards chapter) requires
a surface to render its empty, degraded, and denied states truthfully; this chapter
requires the *words* on those surfaces to do the same: no fabricated certainty, no
overclaim a CISO would catch. Guarantees the system architecturally enforces may be
stated absolutely; anything the product's own configuration can contradict must be
scoped or softened. A small set of marketing claims is banned outright with the
corrected wording pinned beside each (VOICE-BAN-1..3) — most prominently the
"no lock-in" family, which is false for customizations and unnecessary because the
honest claim already wins.

One scope note: German/English copy parity is a V1 requirement owned by the
records-depth localization work (S-E15.10) — this chapter governs the register in
both languages but does not specify localization mechanics.

Finally, the no-dash rule (VOICE-RULE-5) applies to generated user-facing copy, not
to these spec documents, which follow the docs conventions.

## Appendix

### Parameters — writing rules
Source: narrative/voice-and-tone-spec.md#the-four-rules @ 5a0b29c; narrative/voice-and-tone-spec.md#claim--fact-discipline @ 5a0b29c

| ID | Rule |
|---|---|
| VOICE-RULE-1 | Short declaratives. Cut every word that is not load-bearing. |
| VOICE-RULE-2 | Assert, do not justify. Drop "so that / because / in order to" tails. |
| VOICE-RULE-3 | Force comes from flatness and concrete facts, never from adjectives. |
| VOICE-RULE-4 | No warmth or reassurance padding ("we help you...", "without anyone deciding..."). |
| VOICE-RULE-5 | No em dash or en dash anywhere in user-facing copy. Use a plain hyphen ( - ). |
| VOICE-RULE-6 | Architectural guarantees the system enforces may be stated absolutely and build trust ("the AI never acts without your approval"). |
| VOICE-RULE-7 | Location/outcome absolutes that the product's own configuration contradicts are overclaim - a CISO catches them. Scope or soften. |
| VOICE-RULE-8 | Never fake proof. Pre-launch with no references: state conviction honestly (a dated commitment / dogfooding), and label illustrative numbers as illustrative. |
| VOICE-RULE-9 | Keep honest hedges that build trust: "proven with you before go-live", "not a measured customer result". |
| VOICE-RULE-10 | Cite sources for any stat or legal claim. |

### Parameters — strip list
Source: narrative/voice-and-tone-spec.md#strip-these-ai--gpt-tics-they-read-machine-written @ 5a0b29c

| ID | AI tic to strip |
|---|---|
| VOICE-STRIP-1 | Reflexive rule-of-three / tricolons on every line. |
| VOICE-STRIP-2 | "X, not Y" negation-to-emphasize. Keep at most one or two of the best; make the rest plain statements. |
| VOICE-STRIP-3 | Empty punchy fragment-kickers ("nothing drifts out of sync", "every figure traceable"). |
| VOICE-STRIP-4 | Uniform cadence (claim -> parallel elaboration -> kicker) on every line. Vary the rhythm. |
| VOICE-STRIP-5 | Essay-style chiasmus ("the capability that makes it work is the one that breaks it"). |
| VOICE-STRIP-6 | Repeating the same adjective list slide after slide. State it once, strongly. |

### Parameters — banned claims
Source: narrative/customization-portability.md#6-corrected-external-claim-wording @ 5a0b29c; narrative/voice-and-tone-spec.md#claim--fact-discipline @ 5a0b29c

| ID | Banned phrase | Corrected wording |
|---|---|---|
| VOICE-BAN-1 | "no lock-in", "zero lock-in", "never locked in" | Tagline: "Own your data. Own your customization source." Positioning: "Less lock-in than HubSpot on data *and* on customization - because you own the code." |
| VOICE-BAN-2 | Any phrasing implying customizations are portable to non-Gradion systems | Precise form: "Trivial data export; customizations are source-available code you keep and can fork forever (and the core opens to Apache-2.0 within two years)." |
| VOICE-BAN-3 | "Your data never leaves your boundary." (and similar location absolutes the deployment config can contradict) | "You decide where your data lives." |

The full long-form replacement claim for VOICE-BAN-1/2 (the "Own your data, own
your customizations" paragraph, including the honest caveat that customizations run
only on a Gradion-lineage core) lives in the corpus source cited above; use it
verbatim where long-form copy is needed rather than paraphrasing.
