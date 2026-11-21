-- Run once in SQL Editor for the SAME Supabase database used by AGAPE CAMP.
-- This creates a separate table; camp registrations remain unchanged.
CREATE TABLE IF NOT EXISTS public.tres_dias_registrations (
    id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    form_type text NOT NULL CHECK (form_type IN ('participant', 'servant')),
    name text NOT NULL,
    birth_date date,
    city text NOT NULL,
    phone text NOT NULL,
    telegram text,
    church text NOT NULL,
    attended_before boolean,
    clothing_size text NOT NULL,
    has_allergies boolean,
    allergy_details text,
    allergy_consent boolean NOT NULL DEFAULT false,
    personal_data_consent boolean,
    needs_transport boolean,
    departments text[] NOT NULL DEFAULT '{}',
    previous_departments text,
    read_roe text,
    created_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT tres_dias_registration_fields CHECK (
        (form_type = 'participant' AND birth_date IS NOT NULL
            AND cardinality(departments) = 0 AND previous_departments IS NULL AND read_roe IS NULL)
        OR
        (form_type = 'servant' AND attended_before IS NULL AND cardinality(departments) BETWEEN 1 AND 10
            AND previous_departments IS NOT NULL AND read_roe IS NOT NULL)
    )
);

ALTER TABLE public.tres_dias_registrations
    ADD COLUMN IF NOT EXISTS needs_transport boolean,
    ADD COLUMN IF NOT EXISTS personal_data_consent boolean;

-- Also safely updates the size constraint if this table was created earlier.
ALTER TABLE public.tres_dias_registrations
    DROP CONSTRAINT IF EXISTS tres_dias_registrations_clothing_size_check;
ALTER TABLE public.tres_dias_registrations
    DROP CONSTRAINT IF EXISTS tres_dias_clothing_size_check;
ALTER TABLE public.tres_dias_registrations
    ADD CONSTRAINT tres_dias_registrations_clothing_size_check
    CHECK (clothing_size IN ('S', 'M', 'L', 'XL', '2XL', '3XL', '4XL', '5XL'));

CREATE INDEX IF NOT EXISTS tres_dias_registrations_created_at_idx
    ON public.tres_dias_registrations (created_at DESC);
CREATE INDEX IF NOT EXISTS tres_dias_registrations_form_type_idx
    ON public.tres_dias_registrations (form_type);

-- Safe additive migration for an existing TRES DIAS table. Existing answers
-- remain NULL for has_allergies so they are not misrepresented as "No".
ALTER TABLE public.tres_dias_registrations
    ADD COLUMN IF NOT EXISTS has_allergies boolean,
    ADD COLUMN IF NOT EXISTS allergy_details text,
    ADD COLUMN IF NOT EXISTS allergy_consent boolean NOT NULL DEFAULT false;
