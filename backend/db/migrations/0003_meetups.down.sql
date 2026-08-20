DROP TRIGGER IF EXISTS meetup_safety_state_set_updated_at ON meetup_safety_state;
DROP TRIGGER IF EXISTS device_tokens_set_updated_at ON device_tokens;

DROP TABLE IF EXISTS device_tokens;
DROP TABLE IF EXISTS meetup_feedback;
DROP TABLE IF EXISTS meetup_safety_state;
DROP TABLE IF EXISTS meetup_requests;
DROP TABLE IF EXISTS meetups;

DROP TYPE IF EXISTS intent_type;
DROP TYPE IF EXISTS meetup_request_status;
DROP TYPE IF EXISTS meetup_status;
