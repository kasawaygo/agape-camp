function attachBirthDateFormat(input) {
    if (!input) return;
    input.addEventListener("input", () => {
        const digits = input.value.replace(/\D/g, "").slice(0, 8);
        input.value = digits.length > 4
            ? `${digits.slice(0, 4)}.${digits.slice(4, 6)}${digits.length > 6 ? `.${digits.slice(6)}` : ""}`
            : digits;
    });
}

document.querySelectorAll('[data-form]').forEach(form => {
    const message = form.querySelector('.form-message');
    const checks = [...form.querySelectorAll('input[name="departments"]')];
    const dateInput = form.querySelector('[name="birthDate"]');
    const tgInput = form.querySelector('[name="telegram"]');
    const tgNone = form.querySelector('[name="noTelegram"]');
    const syncTelegram = () => {
        const absent = !!tgNone?.checked;
        tgInput.disabled = absent;
        tgInput.required = !absent;
        if (absent) tgInput.value = "";
    };
    tgNone?.addEventListener("change", syncTelegram);
    syncTelegram();
    attachBirthDateFormat(dateInput);
    const allergyRadios = [...form.querySelectorAll('input[name="hasAllergies"]')];
    const allergyDetails = form.querySelector('[name="allergyDetails"]');
    const syncAllergy = () => {
        const yes = form.querySelector('[name="hasAllergies"]:checked')?.value === 'true';
        allergyDetails.disabled = !yes;
        allergyDetails.required = yes;
        if (!yes) allergyDetails.value = '';
    };
    allergyRadios.forEach(input => input.addEventListener('change', syncAllergy));
    syncAllergy();

    const closeOnesRadios = [...form.querySelectorAll('input[name="closeOnesParticipating"]')];
    const mananita = form.querySelector('.mananita-reveal');
    const firstConfirm = form.querySelector('.cancel-confirm:not(.cancel-confirm-final)');
    const finalConfirm = form.querySelector('.cancel-confirm-final');
    const syncMananita = () => {
        const yes = form.querySelector('[name="closeOnesParticipating"]:checked')?.value === 'true';
        if (mananita) mananita.hidden = !yes;
        if (!yes) {
            if (firstConfirm) firstConfirm.hidden = true;
            if (finalConfirm) finalConfirm.hidden = true;
        }
    };
    closeOnesRadios.forEach(input => input.addEventListener('change', syncMananita));
    syncMananita();
    form.querySelector('.cancel-yes')?.addEventListener('click', () => {
        if (firstConfirm) firstConfirm.hidden = false;
    });
    form.querySelectorAll('[data-cancel-step]').forEach(button => button.addEventListener('click', () => {
        const step = button.dataset.cancelStep;
        if (step === '1') {
            if (firstConfirm) firstConfirm.hidden = true;
            if (finalConfirm) finalConfirm.hidden = false;
        } else if (step === '2') {
            closeOnesRadios.forEach(input => { input.checked = false; });
            syncMananita();
        } else {
            if (firstConfirm) firstConfirm.hidden = true;
            if (finalConfirm) finalConfirm.hidden = true;
        }
    }));
    form.querySelector('.ghost-service')?.addEventListener('click', event => {
        const button = event.currentTarget;
        button.classList.add('ghost-service-away');
        button.disabled = true;
        window.setTimeout(() => button.remove(), 800);
    });

    form.addEventListener('submit', async event => {
        event.preventDefault();
        message.className = 'form-message';
        message.textContent = '';
        const selected = checks.filter(input => input.checked);
        const warning = form.querySelector('.inline-error');
        if (checks.length && (selected.length < 1 || selected.length > checks.length)) {
            warning.hidden = false;
            checks[0].focus();
            return;
        }
        if (warning) warning.hidden = true;
        const data = new FormData(form);
        const birthDate = (data.get('birthDate') || '').trim();
        if (dateInput?.value && !/^\d{4}\.\d{2}\.\d{2}$/.test(birthDate)) {
            dateInput.setCustomValidity('Введи дату как ГГГГ.ММ.ДД, например 2000.01.09.');
            dateInput.reportValidity();
            dateInput.setCustomValidity('');
            return;
        }
        const payload = {
            formType: form.dataset.form,
            name: data.get('name'),
            birthDate,
            city: data.get('city'),
            phone: data.get('phone'),
            telegram: tgNone?.checked ? 'Нет Telegram' : data.get('telegram'),
            church: data.get('church'),
            clothingSize: data.get('clothingSize'),
            needsTransport: data.has('needsTransport') ? data.get('needsTransport') === 'true' : null,
            closeOnesParticipating: data.has('closeOnesParticipating') ? data.get('closeOnesParticipating') === 'true' : null,
            hasAllergies: data.get('hasAllergies') === 'true',
            allergyDetails: data.get('allergyDetails') || '',
            personalDataConsent: data.has('personalDataConsent'),
            departments: selected.map(input => input.value),
            previousDepartments: data.get('previousDepartments') || '',
            readRoe: data.get('readRoe') || ''
        };
        const button = form.querySelector('button[type="submit"]');
        button.disabled = true;
        button.textContent = 'Отправляем…';
        try {
            const response = await fetch('/api/tres-dias/register', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify(payload)
            });
            const result = await response.json();
            if (!response.ok) throw new Error(result.message || 'Не получилось отправить анкету.');
            form.reset();
            syncTelegram();
            syncAllergy();
            syncMananita();
            message.textContent = result.message;
            message.classList.add('success');
        } catch (error) {
            message.textContent = error.message;
            message.classList.add('error');
        } finally {
            button.disabled = false;
            button.textContent = 'Отправить анкету';
        }
    });
});
