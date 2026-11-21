-- Safe, additive migration. Existing TRES DIAS registrations are preserved.
-- NULL means older submissions did not answer the allergy question.
ALTER TABLE public.tres_dias_registrations
    ADD COLUMN IF NOT EXISTS has_allergies boolean,
    ADD COLUMN IF NOT EXISTS allergy_details text,
    ADD COLUMN IF NOT EXISTS allergy_consent boolean NOT NULL DEFAULT false;
