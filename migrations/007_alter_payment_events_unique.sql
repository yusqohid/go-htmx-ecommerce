-- 007_alter_payment_events_unique.sql
ALTER TABLE payment_events DROP CONSTRAINT IF EXISTS payment_events_event_id_key;
ALTER TABLE payment_events ADD CONSTRAINT payment_events_provider_event_id_key UNIQUE (provider, event_id);
