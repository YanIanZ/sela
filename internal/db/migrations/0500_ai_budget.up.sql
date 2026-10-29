-- sela:kind schema
-- sela:requires 0001
--
-- Durable AI spend ledger and budget admission (D12). Redis is never the
-- authority here: an outage or restart cannot reset spend. Rows carry no
-- user id, session id, prompt or completion text; only task, provider,
-- token counts and estimated cost.
CREATE TABLE ai_spend (
  id            uuid PRIMARY KEY,
  reserved_at   timestamptz NOT NULL,
  settled_at    timestamptz NULL,
  task          text NOT NULL,
  provider      text NULL,
  input_tokens  integer NOT NULL DEFAULT 0 CHECK (input_tokens >= 0),
  output_tokens integer NOT NULL DEFAULT 0 CHECK (output_tokens >= 0),
  reserved_usd  numeric(12,6) NOT NULL CHECK (reserved_usd > 0),
  actual_usd    numeric(12,6) NULL CHECK (actual_usd >= 0),
  outcome       text NULL,
  synthetic     boolean NOT NULL DEFAULT false,
  CHECK (actual_usd IS NULL OR settled_at IS NOT NULL)
);

CREATE INDEX ai_spend_reserved_at ON ai_spend (reserved_at DESC);
CREATE INDEX ai_spend_open ON ai_spend (reserved_at) WHERE settled_at IS NULL;

REVOKE ALL ON ai_spend FROM PUBLIC;
