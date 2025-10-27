package outbox

const activityCreatedSchema = `{
  "type": "object",
  "title": "ActivityCreated",
  "properties": {
    "activity_id": {"type": "string"},
    "tenant_id": {"type": "string"},
    "user_id": {"type": "string"},
    "activity_type": {"type": "string"},
    "started_at": {"type": "string", "format": "date-time"},
    "duration_min": {"type": "integer"},
    "source": {"type": "string"},
    "version": {"type": "string"}
  },
  "required": ["activity_id", "tenant_id", "user_id", "activity_type", "started_at", "duration_min", "source", "version"],
  "additionalProperties": false
}`

const activityStateChangedSchema = `{
  "type": "object",
  "title": "ActivityStateChanged",
  "properties": {
    "activity_id": {"type": "string"},
    "tenant_id": {"type": "string"},
    "user_id": {"type": "string"},
    "state": {"type": "string"},
    "occurred_at": {"type": "string", "format": "date-time"},
    "reason": {"type": "string"}
  },
  "required": ["activity_id", "tenant_id", "user_id", "state", "occurred_at"],
  "additionalProperties": false
}`
