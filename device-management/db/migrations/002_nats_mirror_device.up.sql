-- Persistent NATS mirror state per DCD (survives actions table cleanup).
ALTER TABLE devices
  ADD COLUMN IF NOT EXISTS nats_mirror_deployed_at TIMESTAMP WITH TIME ZONE;

COMMENT ON COLUMN devices.nats_mirror_deployed_at IS
  'Set when UNS-to-NATS-mirror deploy-data-flow-component succeeded on the edge; NULL = not yet deployed.';
