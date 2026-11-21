-- Add Telegram for new TRES DIAS registrations without modifying old answers.
ALTER TABLE public.tres_dias_registrations
    ADD COLUMN IF NOT EXISTS telegram text;
