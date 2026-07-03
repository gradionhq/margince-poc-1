package crmgdpr

// Object/entity types the GDPR engine operates over.
const (
	objectPerson   = "person"
	objectLead     = "lead"
	objectDeal     = "deal"
	objectActivity = "activity"
)

// Retention-ladder actions and other reused literals.
const (
	actionErase      = "erase"
	actionArchive    = "archive"
	actionAnonymize  = "anonymize"
	actorSystem      = "system"
	statusLost       = "lost"
	sourceTranscript = "transcript"
)
