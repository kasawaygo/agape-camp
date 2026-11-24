
// ===== ELEMENTS =====
const parentPhoneField = document.querySelector("#parentPhoneField");
const parentPhoneInput = document.querySelector("#parentPhone");

const form = document.querySelector("#registrationForm");
const birthDateInput = document.querySelector("#birthDate");
birthDateInput.addEventListener("input", () => {
    const digits = birthDateInput.value.replace(/\D/g, "").slice(0, 8);
    birthDateInput.value = digits.length > 4
        ? `${digits.slice(0, 4)}.${digits.slice(4, 6)}${digits.length > 6 ? `.${digits.slice(6)}` : ""}`
        : digits;
});

const roleCards = document.querySelectorAll(".role-card");
const dreamField = document.querySelector("#dreamField");
const serviceField = document.querySelector("#serviceField");
const allergyField = document.querySelector("#allergyField");

const serviceCheckboxes = document.querySelectorAll(
    'input[name="serviceAreas"]'
);

const allergyCheckboxes = document.querySelectorAll(
    'input[name="allergies"]'
);

const otherService = document.querySelector("#otherService");
const otherServiceText = document.querySelector("#otherServiceText");

const otherAllergy = document.querySelector("#otherAllergy");
const otherAllergyText = document.querySelector("#otherAllergyText");

const telegramInput = document.querySelector("#telegram");
const noTelegramCheckbox = document.querySelector("#noTelegram");
const alternateContact = document.querySelector("#alternateContact");
const alternateContactLabel = document.querySelector("#alternateContactLabel");
function syncTelegramAvailability() {
    const absent = noTelegramCheckbox.checked;
    telegramInput.disabled = absent;
    telegramInput.required = !absent;
    alternateContactLabel.hidden = !absent;
    alternateContact.disabled = !absent;
    alternateContact.required = absent;
    if (absent) {
        telegramInput.value = "";
        telegramInput.setCustomValidity("");
    }
}
noTelegramCheckbox.addEventListener("change", syncTelegramAvailability);
syncTelegramAvailability();

// ===== ROLE SWITCHING =====


function updateRole() {
    const selectedRole = document.querySelector(
        'input[name="role"]:checked'
    );

    if (!selectedRole) return;

    const isServant = selectedRole.value === "servant";

    dreamField.hidden = isServant;
    serviceField.hidden = !isServant;
    allergyField.hidden = isServant;

    parentPhoneField.hidden = isServant;
    parentPhoneInput.required = !isServant;

    if (isServant) {
        document.querySelector("#dream").value = "";
        parentPhoneInput.value = "";

        allergyCheckboxes.forEach(item => {
            item.checked = false;
        });

        otherAllergyText.value = "";
        otherAllergyText.hidden = true;

        document.querySelector("#allergyConsent").checked = false;
    } else {
        serviceCheckboxes.forEach(item => {
            item.checked = false;
        });

        otherServiceText.value = "";
        otherServiceText.hidden = true;
    }

    roleCards.forEach(card => {
        const radio = card.querySelector('input[type="radio"]');

        card.classList.toggle("selected", radio.checked);
    });
}


roleCards.forEach(card => {
    const radio = card.querySelector('input[type="radio"]');
    radio.addEventListener("change", updateRole);
});

// ===== OTHER SERVICE =====

otherService.addEventListener("change", () => {
    otherServiceText.hidden = !otherService.checked;

    if (!otherService.checked) {
        otherServiceText.value = "";
    } else {
        otherServiceText.focus();
    }
});

// ===== OTHER ALLERGY =====

otherAllergy.addEventListener("change", () => {
    otherAllergyText.hidden = !otherAllergy.checked;

    if (!otherAllergy.checked) {
        otherAllergyText.value = "";
    } else {
        otherAllergyText.focus();
    }
});

// ===== TELEGRAM VALIDATION =====

function validateTelegram(username) {
    return /^@[A-Za-z][A-Za-z0-9_]{4,31}$/.test(username);
}

telegramInput.addEventListener("input", () => {
    telegramInput.setCustomValidity("");
});

// ===== FORM SUBMISSION =====


form.addEventListener("submit", async event => {
    event.preventDefault();

    const selectedRole = document.querySelector(
        'input[name="role"]:checked'
    );

    if (!selectedRole) return;

    const telegram = noTelegramCheckbox.checked ? `Нет Telegram; Связь: ${alternateContact.value.trim()}` : telegramInput.value.trim();

    if (!noTelegramCheckbox.checked && !validateTelegram(telegram)) {
        telegramInput.setCustomValidity(
            "Введи корректный Telegram username, начиная с @"
        );
        telegramInput.reportValidity();
        return;
    }

    telegramInput.setCustomValidity("");

    if (selectedRole.value === "servant") {
        const selectedServices = [...serviceCheckboxes]
            .filter(item => item.checked);

        if (selectedServices.length === 0) {
            alert("Выбери хотя бы одно направление служения!");
            return;
        }

        if (otherService.checked && !otherServiceText.value.trim()) {
            alert("Напиши свой вариант служения!");
            otherServiceText.focus();
            return;
        }
    }

    if (selectedRole.value === "participant") {
        const selectedAllergies = [...allergyCheckboxes]
            .filter(item => item.checked);

        if (otherAllergy.checked && !otherAllergyText.value.trim()) {
            alert("Укажи свою аллергию!");
            otherAllergyText.focus();
            return;
        }

        if (selectedAllergies.length > 0 &&
            !document.querySelector("#allergyConsent").checked) {
            alert("Необходимо согласие на обработку информации об аллергиях.");
            return;
        }
    }

    const dottedBirthDate = birthDateInput.value.trim();
    if (!/^\d{4}\.\d{2}\.\d{2}$/.test(dottedBirthDate)) {
        birthDateInput.setCustomValidity("Введи дату как ГГГГ.ММ.ДД, например 2000.01.09.");
        birthDateInput.reportValidity();
        birthDateInput.setCustomValidity("");
        return;
    }

    const data = {
        role: selectedRole.value,
        name: document.querySelector("#name").value.trim(),
        phone: document.querySelector("#phone").value.trim(),
        telegram: telegram,
        birthDate: dottedBirthDate.replaceAll(".", "-"),
        church: document.querySelector("#church").value.trim(),

        parentPhone: selectedRole.value === "participant"
            ? parentPhoneInput.value.trim()
            : "",

        dream: selectedRole.value === "participant"
            ? document.querySelector("#dream").value.trim()
            : "",

        allergies: selectedRole.value === "participant"
            ? [...allergyCheckboxes]
                .filter(item => item.checked)
                .map(item => item.value)
            : [],

        otherAllergy: selectedRole.value === "participant"
            ? otherAllergyText.value.trim()
            : "",

        allergyConsent: selectedRole.value === "participant"
            ? document.querySelector("#allergyConsent").checked
            : false,

        serviceAreas: selectedRole.value === "servant"
            ? [...serviceCheckboxes]
                .filter(item => item.checked)
                .map(item => item.value)
            : [],

        otherService: selectedRole.value === "servant"
            ? otherServiceText.value.trim()
            : ""
    };

    const submitButton = form.querySelector('button[type="submit"]');
    const message = document.querySelector("#formMessage");

    submitButton.disabled = true;
    message.textContent = "Отправляем анкету...";

    try {
        const response = await fetch("/api/register", {
            method: "POST",
            headers: {
                "Content-Type": "application/json"
            },
            body: JSON.stringify(data)
        });

        const result = await response.json();

        if (!response.ok) {
            throw new Error(result.message || "Не удалось сохранить анкету");
        }

        message.textContent = "Анкета успешно отправлена! Спасибо за регистрацию ❤️";
        form.reset();
        syncTelegramAvailability();
        updateRole();

    } catch (error) {
        console.error("Registration error:", error);
        message.textContent =
            "Не удалось отправить анкету. Попробуй ещё раз.";
    } finally {
        submitButton.disabled = false;
    }
});


// ===== SNOWFALL =====

const snowContainer = document.querySelector("#snowContainer");

const snowSymbols = ["❄", "❅", "❆", "✧"];

const snowflakeCount = window.matchMedia(
    "(max-width: 550px)"
).matches ? 22 : 38;

for (let i = 0; i < snowflakeCount; i++) {
    const snowflake = document.createElement("span");

    snowflake.className = "snowflake";

    snowflake.textContent =
        snowSymbols[Math.floor(Math.random() * snowSymbols.length)];

    snowflake.style.setProperty("--left", `${Math.random() * 100}%`);
    snowflake.style.setProperty("--size", `${10 + Math.random() * 13}px`);
    snowflake.style.setProperty("--duration", `${9 + Math.random() * 13}s`);
    snowflake.style.setProperty("--delay", `${-Math.random() * 20}s`);
    snowflake.style.setProperty("--sway", `${-35 + Math.random() * 70}px`);
    snowflake.style.setProperty("--opacity", `${0.3 + Math.random() * 0.55}`);

    snowContainer.appendChild(snowflake);
}
