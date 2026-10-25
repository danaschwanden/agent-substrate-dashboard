# Make bulk lifecycle operations non-atomic

Bulk Resume and Suspend will issue bounded parallel Control API calls and return one outcome per selected actor. The backend will not attempt rollback when some calls fail, because actor state can change concurrently and Agent Substrate exposes lifecycle operations per actor rather than as a fleet transaction.
