DROP INDEX IF EXISTS auth_identities_user_id_idx;
DROP TABLE IF EXISTS auth_identities;

UPDATE users
SET password_hash = ''
WHERE password_hash IS NULL;

ALTER TABLE users
  ALTER COLUMN password_hash SET NOT NULL;
