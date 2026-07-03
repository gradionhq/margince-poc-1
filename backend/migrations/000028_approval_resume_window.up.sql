-- B-EP06.21: durable window snapshot for a suspended Surface-B run held at a 🟡
-- approval. Carries the loop cursor + observations + pending proposal so the run
-- resumes as ONE continuous trace on the human's decision. Payload stays the raw
-- tool args (Decider.Approve feeds it to execAction); the resume window is separate.
ALTER TABLE approval_item ADD COLUMN resume_window jsonb;
