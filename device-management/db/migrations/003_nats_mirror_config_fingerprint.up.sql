-- Track which NATS mirror URL set was last applied so config changes trigger edit-data-flow-component.
ALTER TABLE devices
  ADD COLUMN IF NOT EXISTS nats_mirror_config_fingerprint TEXT;

COMMENT ON COLUMN devices.nats_mirror_config_fingerprint IS
  'SHA-256 hex of sorted deployment NATS URLs last applied to UNS-to-NATS-mirror on this device';
