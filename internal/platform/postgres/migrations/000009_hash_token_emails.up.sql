-- Store account.Hash(email) instead of plaintext (SHA-256 hex of lower(trim(email))).
CREATE EXTENSION IF NOT EXISTS pgcrypto;

UPDATE tokens
SET email = encode(digest(lower(trim(email)), 'sha256'), 'hex')
WHERE email !~ '^[0-9a-f]{64}$';

UPDATE serializd_tokens
SET email = encode(digest(lower(trim(email)), 'sha256'), 'hex')
WHERE email !~ '^[0-9a-f]{64}$';
