-- Identity is also a consumer of action-bound proofs for administrator
-- mutations. The primary key is the atomic replay decision; no read-before-
-- write check is permitted.
CREATE TABLE step_up_proof_uses (
    jti         UUID PRIMARY KEY,
    expires_at  TIMESTAMPTZ NOT NULL,
    consumed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX step_up_proof_uses_expiry_idx
    ON step_up_proof_uses(expires_at);
