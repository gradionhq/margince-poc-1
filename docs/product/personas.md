---
derives-from:
  - specs/spec/product/01-personas.md
---
# Personas — the five people the product is judged by

Five named people, not market segments. They are **normative**: every user story in
this spec names one of them in its "As a …", every journey is walked in their shoes,
and an acceptance claim that no persona would recognize as a win is suspect. Keep
them concrete — they are who the product is judged by.

**Sam, the sales rep** (PERSONA-SAM), is primary: an AE or founder-seller carrying a
number, living in inbox and calendar, context-switching constantly. Sam's ask is the
whole product in one line: *"Don't ask me to update the CRM. Read what happened and
tell me what to do next."* What wins Sam is a calendar already full of the right
meetings, a dossier waiting before each one, an inferred next step asking "correct?",
and zero manual logging.

**Riya, the revenue leader** (PERSONA-RIYA), owns the forecast and the team. Her
pains are reporting that breaks on joins and a forecast built on fields reps forgot
to update. What wins her is reporting on a clean relational core, deal health
inferred from the actual conversation rather than a stage field, and being able to
ask "explain this number." *"I need the forecast to be true, and I need to know why
each number is what it is."*

**Devin, the developer-founder** (PERSONA-DEVIN), is the SMB economic buyer and the
customizer: he bends the CRM to his business through real custom development on code
he owns — himself, or via a partner or Gradion — and connects the coding agent he
already pays for. He is allergic to config ceilings, per-AI-seat taxes, and credit
hard stops. *"I found this thing. It runs my whole marketing and sales, I connect the
agent I already pay for, and I just get results."*

**Mor, the CRM admin / sales ops** (PERSONA-MOR), sets up pipelines, roles, and
permissions, runs the HubSpot migration, keeps data clean, and governs what agents
may do. What wins Mor is opinionated defaults, a believable import, RBAC that also
bounds agents, and an audit log plus approval inbox for every outward action. *"I
want strong defaults, a clean migration, and a record of every change a human or an
agent made."*

**Pat, the prospect** (PERSONA-PAT), is external and never logs in. Pat experiences
the product through outreach, the pre-meeting interaction, and later a deal room —
so the stories judged by the buyer's experience need a named person on the other
side. Pat exists partly as a **guard**: Pat's behavior is signal only when
consent-gated and company-level, and any feature that would covertly profile an
external individual is a defect, not a feature (PERSONA-PAT-GUARD-1). *"If you're
going to reach out, know why it matters to me — and make the next step easy."*

## Appendix

### Parameters — personas
Source: product/01-personas.md @ 5a0b29c

| ID | Persona | Role | Jobs-to-be-done (kernel) | What wins them (kernel) |
|---|---|---|---|---|
| PERSONA-SAM | Sam — the Sales Rep (primary) | AE / founder-seller carrying a number; B2B deals €5k–€100k, 5–30 active deals | Show up to the right conversations; know what hurts each prospect before the call; never lose a deal to a dropped follow-up | Calendar already full of the right meetings; dossier waiting before each one; inferred next step asking "correct?"; zero manual logging |
| PERSONA-RIYA | Riya — the Revenue Leader | Head of Sales / VP Revenue / founder wearing the leader hat; owns forecast and team | Trust the pipeline number; see which deals are real; coach reps; spot risk early | Reporting on a clean relational core where joins just work; "explain this number"; deal health inferred from the actual conversation; coaching signals from real calls |
| PERSONA-DEVIN | Devin — the Developer-Founder (SMB buyer + customizer) | Technical founder of a small company; economic buyer and the person who customizes | Run marketing + sales without a team; bend the CRM via real custom development on code he owns (self, partner, or Gradion — A39); own the data and the code | Marketing-and-sales-in-a-box modules; adaptation with no config ceiling; BYO his existing Claude/Cursor for CRM work; own-your-data, leave-in-an-afternoon |
| PERSONA-MOR | Mor — the CRM Admin / Sales Ops | Ops/admin at a 10–100-person revenue org | Set up pipelines/stages; manage users, roles, permissions; run the HubSpot migration; keep data clean; govern what agents may do | Opinionated defaults needing little config; believable HubSpot import; RBAC that bounds agents (Agent Seat Passport); audit log + approval inbox for every outward action; dedupe that just works |
| PERSONA-PAT | Pat — the Prospect / Buyer (external; not a CRM user) | Champion, user, or economic buyer on the other side of the deal; never logs in | Experience relevance, not spray; a genuinely useful single place for the buying journey (deal-room vision) | Warm, contextual outreach that shows the seller knows why it matters; an easy next step |

**PERSONA-PAT-GUARD-1 — the Pat protection rule.** Pat is never covertly profiled:
buyer-side signals are captured **consent-gated and company-level** by design (P12),
and any story judged by Pat's behavior must survive that constraint. Covert profiling
of external prospects is a structural rejection, pinned once at [[scope#NEVER-8]] —
a chapter or ticket reintroducing it is a defect.
