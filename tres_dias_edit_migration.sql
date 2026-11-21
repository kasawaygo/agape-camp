-- Keep existing TRES DIAS submissions. Allow the participant form to omit the
-- former attendance question and allow servants to select 1 through 10 departments.
ALTER TABLE public.tres_dias_registrations
    DROP CONSTRAINT IF EXISTS tres_dias_registration_fields;
ALTER TABLE public.tres_dias_registrations
    ADD CONSTRAINT tres_dias_registration_fields CHECK (
        (form_type = 'participant' AND birth_date IS NOT NULL
            AND cardinality(departments) = 0 AND previous_departments IS NULL AND read_roe IS NULL)
        OR
        (form_type = 'servant' AND attended_before IS NULL
            AND cardinality(departments) BETWEEN 1 AND 10
            AND previous_departments IS NOT NULL AND read_roe IS NOT NULL)
    );
