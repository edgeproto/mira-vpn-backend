CREATE TABLE IF NOT EXISTS guest_devices (
  device_id text PRIMARY KEY,
  user_id uuid NOT NULL UNIQUE REFERENCES users(id) ON DELETE CASCADE,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS guest_devices_user_id_idx ON guest_devices(user_id);
