BEGIN;

DROP TABLE IF EXISTS consent_event;
DROP FUNCTION IF EXISTS consent_event_immutable();
DROP TABLE IF EXISTS person_consent;
DROP TABLE IF EXISTS consent_purpose;

COMMIT;
