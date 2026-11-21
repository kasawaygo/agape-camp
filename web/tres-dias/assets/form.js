function attachBirthDateFormat(input) {
    if (!input) return;
    input.addEventListener("input", () => {
        const digits = input.value.replace(/\D/g, "").slice(0, 8);
        input.value = digits.length > 4
            ? `${digits.slice(0, 4)}.${digits.slice(4, 6)}${digits.length > 6 ? `.${digits.slice(6)}` : ""}`
            : digits;
    });
}

function dateForServer(value) {
    const match = /^(\d{4})\.(\d{2})\.(\d{2})$/.exec(value || "");
    return match ? `${match[1]}-${match[2]}-${match[3]}` : "";
}

document.querySelectorAll('[data-form]').forEach(form => {
    const message = form.querySelector('.form-message');
    const checks = [...form.querySelectorAll('input[name="departments"]')];
    const dateInput = form.querySelector('[name="birthDate"]');
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
        const birthDate = dateForServer(data.get('birthDate'));
        if (dateInput?.value && !birthDate) {
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
            church: data.get('church'),
            clothingSize: data.get('clothingSize'),
            hasAllergies: data.get('hasAllergies') === 'true',
            allergyDetails: data.get('allergyDetails') || '',
            allergyConsent: data.has('allergyConsent'),
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
            syncAllergy();
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
