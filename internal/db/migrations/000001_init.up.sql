-- Phase 1 DB schema: users + peers
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE IF NOT EXISTS users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email text NOT NULL UNIQUE,
  password_hash text NOT NULL,
  is_pro boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS peers (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  location text NOT NULL,
  wg_public_key text NOT NULL,
  status text NOT NULL DEFAULT 'pending',
  created_at timestamptz NOT NULL DEFAULT now()
);

-- One peer per user per location (keeps provisioning logic simple).
CREATE UNIQUE INDEX IF NOT EXISTS peers_user_location_unique ON peers(user_id, location);

CREATE INDEX IF NOT EXISTS peers_user_id_idx ON peers(user_id);

