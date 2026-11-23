-- Run before setting SHEETS_SYNC_TOKEN. Additive: no registration rows change.
CREATE TABLE IF NOT EXISTS public.registration_sync_audit (
 id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 kind text NOT NULL CHECK (kind IN ('camp','tres')),
 registration_id text NOT NULL,
 request_id text,
 source text NOT NULL,
 before_data jsonb NOT NULL,
 after_data jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(kind,request_id)
);
ALTER TABLE public.registration_sync_audit ENABLE ROW LEVEL SECURITY;
REVOKE ALL ON public.registration_sync_audit FROM anon, authenticated;
