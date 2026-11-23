
const table = document.querySelector("#registrationTable");
const totalCount = document.querySelector("#totalCount");
const participantCount = document.querySelector("#participantCount");
const servantCount = document.querySelector("#servantCount");

const searchInput = document.querySelector("#searchInput");
const churchFilter = document.querySelector("#churchFilter");
const loadingMessage = document.querySelector("#loadingMessage");
const emptyMessage = document.querySelector("#emptyMessage");

const refreshButton = document.querySelector("#refreshButton");
const exportButton = document.querySelector("#exportButton");
// ===== EXPORT EXCEL =====

exportButton.addEventListener("click", async () => {
    exportButton.disabled = true;
    exportButton.textContent = "Подготовка...";

    try {
        const response = await fetch("/api/admin/registrations/export");

        if (response.status === 401) {
            window.location.href = "/admin/login";
            return;
        }

        if (!response.ok) {
            throw new Error("Не удалось создать Excel-файл");
        }

        const blob = await response.blob();
        const url = URL.createObjectURL(blob);

        const link = document.createElement("a");
        link.href = url;
        link.download = "agape_registrations.xlsx";
        link.click();

        URL.revokeObjectURL(url);

    } catch (error) {
        alert(error.message);
    } finally {
        exportButton.disabled = false;
        exportButton.textContent = "↓ Экспорт Excel";
    }
});
const logoutButton = document.querySelector("#logoutButton");
const filterButtons = document.querySelectorAll(".filter");

let registrations = [];
let currentFilter = "all";

// ===== LOAD REGISTRATIONS =====

async function loadRegistrations() {
    loadingMessage.hidden = false;
    emptyMessage.hidden = true;
    table.innerHTML = "";

    try {
        const response = await fetch("/api/admin/registrations");

        if (response.status === 401) {
            window.location.href = "/admin/login";
            return;
        }

        if (!response.ok) {
            throw new Error("Не удалось загрузить регистрации");
        }

        registrations = await response.json();

        updateStatistics();
        updateChurchFilter();
        renderRegistrations();      

    } catch (error) {
        loadingMessage.textContent = error.message;
        return;
    }

    loadingMessage.hidden = true;
}

// ===== STATISTICS =====

function updateStatistics() {
    const participants = registrations.filter(
        item => item.role === "participant"
    );

    const servants = registrations.filter(
        item => item.role === "servant"
    );

    totalCount.textContent = registrations.length;
    participantCount.textContent = participants.length;
    servantCount.textContent = servants.length;
}

// ===== CHURCH FILTER =====

function updateChurchFilter() {
    const currentValue = churchFilter.value;

    const churches = [...new Set(
        registrations
            .map(item => item.church?.trim())
            .filter(Boolean)
    )].sort((a, b) => a.localeCompare(b, "ru"));

    churchFilter.innerHTML = "";

    const allOption = document.createElement("option");
    allOption.value = "all";
    allOption.textContent = "Все церкви";
    churchFilter.appendChild(allOption);

    churches.forEach(church => {
        const option = document.createElement("option");
        option.value = church;
        option.textContent = church;
        churchFilter.appendChild(option);
    });

    if (churches.includes(currentValue)) {
        churchFilter.value = currentValue;
    }
}

churchFilter.addEventListener("change", renderRegistrations);

// ===== RENDER TABLE =====

// ===== RENDER TABLE =====

function renderRegistrations() {
    const search = searchInput.value.trim().toLowerCase();
    const selectedChurch = churchFilter.value;

    const filtered = registrations.filter(item => {
        const matchesRole =
            currentFilter === "all" || item.role === currentFilter;

        const matchesChurch =
            selectedChurch === "all" || item.church === selectedChurch;

        const searchableText = [
            item.name,
            item.phone,
            item.telegram,
            item.church
        ].join(" ").toLowerCase();

        const matchesSearch = searchableText.includes(search);

        return matchesRole && matchesChurch && matchesSearch;
    });

    table.innerHTML = "";

    loadingMessage.hidden = true;
    emptyMessage.hidden = filtered.length !== 0;

    filtered.forEach(item => {
        const row = document.createElement("tr");

        const roleName =
            item.role === "participant" ? "Участник" : "Служитель";

        const roleClass =
            item.role === "participant" ? "participant" : "servant";

        const date = new Date(item.createdAt).toLocaleDateString("ru-RU");

        const cells = [
            item.name,
            roleName,
            item.church || "—",
            item.phone || "—",
            date
        ];

        cells.forEach((value, index) => {
            const cell = document.createElement("td");

            if (index === 1) {
                const badge = document.createElement("span");
                badge.className = `role-tag ${roleClass}`;
                badge.textContent = value;
                cell.appendChild(badge);
            } else {
                cell.textContent = value;
            }

            row.appendChild(cell);
        });

        const actionCell = document.createElement("td");

        const detailsButton = document.createElement("button");
        detailsButton.className = "details-button";
        detailsButton.textContent = "Подробнее →";
        detailsButton.addEventListener("click", () => {
            showRegistration(item);
        });

        const deleteButton = document.createElement("button");
        deleteButton.className = "delete-button";
        deleteButton.textContent = "Удалить";
        deleteButton.addEventListener("click", () => {
            deleteRegistration(item);
        });

        actionCell.append(detailsButton, deleteButton);
        row.appendChild(actionCell);

        table.appendChild(row);
    });
}

// ===== REGISTRATION DETAILS MODAL =====

const registrationModal = document.querySelector("#registrationModal");
const modalBody = document.querySelector("#modalBody");
const modalTitle = document.querySelector("#modalTitle");
const closeModalButton = document.querySelector("#closeModalButton");
const closeModalFooter = document.querySelector("#closeModalFooter");
const editRegistrationButton = document.querySelector("#editRegistrationButton");

let selectedRegistration = null;
let editingRegistration = false;

function showRegistration(item) {
    selectedRegistration = item;
    editingRegistration = false;
    renderRegistrationDetails();
    registrationModal.hidden = false;
    document.body.style.overflow = "hidden";
}

function formatBirthDate(value) {
    return (value || "").replace(/^(\d{4})-(\d{2})-(\d{2})$/, "$1.$2.$3") || "—";
}

function addSection(title, fields) {
    const section = document.createElement("section");
    section.className = "modal-section";

    const heading = document.createElement("h3");
    heading.textContent = title;
    section.appendChild(heading);

    fields.forEach(([label, value]) => {
        const row = document.createElement("div");
        row.className = "modal-field";

        const name = document.createElement("span");
        name.textContent = label;

        const content = document.createElement("strong");
        content.textContent = value || "—";

        row.append(name, content);
        section.appendChild(row);
    });

    modalBody.appendChild(section);
}

function addInput(container, label, name, value, type = "text") {
    const wrapper = document.createElement("label");
    wrapper.className = "edit-field";

    const caption = document.createElement("span");
    caption.textContent = label;

    const input = document.createElement("input");
    input.type = type;
    input.name = name;
    input.value = value || "";
    if (name === "birthDate") {
        input.type = "text";
        input.inputMode = "numeric";
        input.maxLength = 10;
        input.pattern = "[0-9]{4}\\.[0-9]{2}\\.[0-9]{2}";
        input.placeholder = "ГГГГ.ММ.ДД";
        input.value = input.value.replace(/^(\d{4})-(\d{2})-(\d{2})$/, "$1.$2.$3");
        input.addEventListener("input", () => {
            const digits = input.value.replace(/\D/g, "").slice(0, 8);
            input.value = digits.length > 4
                ? `${digits.slice(0, 4)}.${digits.slice(4, 6)}${digits.length > 6 ? `.${digits.slice(6)}` : ""}`
                : digits;
        });
    }

    wrapper.append(caption, input);
    container.appendChild(wrapper);
}

function addTextarea(container, label, name, value) {
    const wrapper = document.createElement("label");
    wrapper.className = "edit-field";

    const caption = document.createElement("span");
    caption.textContent = label;

    const textarea = document.createElement("textarea");
    textarea.name = name;
    textarea.value = Array.isArray(value)
        ? value.join(", ")
        : value || "";
    textarea.rows = 3;

    wrapper.append(caption, textarea);
    container.appendChild(wrapper);
}

function renderRegistrationDetails() {
    const item = selectedRegistration;
    const isParticipant = item.role === "participant";

    modalTitle.textContent = editingRegistration
        ? "Редактирование анкеты"
        : isParticipant ? "Анкета участника" : "Анкета служителя";

    modalBody.innerHTML = "";

    editRegistrationButton.hidden = editingRegistration;

    if (!editingRegistration) {
        addSection("Личная информация", [
            ["Имя", item.name],
            ["Телефон", item.phone],
            ["Telegram", item.telegram],
            ["Дата рождения", formatBirthDate(item.birthDate)],
            ["Церковь", item.church]
        ]);

        if (isParticipant) {
            addSection("Информация участника", [
                ["Мечта", item.dream],
                ["Аллергии", item.allergies?.join(", ")],
                ["Другие аллергии", item.otherAllergy],
                ["Согласие", item.allergyConsent ? "Да" : "Нет"],
                ["Телефон родителя", item.parentPhone]
            ]);
        } else {
            addSection("Служение", [
                ["Направления", item.serviceAreas?.join(", ")],
                ["Другое направление", item.otherService]
            ]);
        }

        addSection("Регистрация", [
            ["Дата и время", item.createdAt
                ? new Date(item.createdAt).toLocaleString("ru-RU")
                : "—"]
        ]);

        return;
    }

    const form = document.createElement("form");
    form.id = "registrationEditForm";

    addInput(form, "Имя", "name", item.name);
    addInput(form, "Телефон", "phone", item.phone);
    addInput(form, "Telegram", "telegram",
        item.telegram === "Нет Telegram" ? "" : item.telegram);
    const tgLabel = document.createElement("label");
    tgLabel.className = "edit-checkbox";
    const tgNone = document.createElement("input");
    tgNone.type = "checkbox";
    tgNone.name = "noTelegram";
    tgNone.checked = item.telegram === "Нет Telegram";
    tgLabel.append(tgNone, document.createTextNode(" У меня нет Telegram"));
    form.appendChild(tgLabel);
    const tgInput = form.querySelector('input[name="telegram"]');
    const syncEditorTelegram = () => {
        tgInput.disabled = tgNone.checked;
        if (tgNone.checked) tgInput.value = "";
    };
    tgNone.addEventListener("change", syncEditorTelegram);
    syncEditorTelegram();
    addInput(form, "Дата рождения (ГГГГ.ММ.ДД)", "birthDate", item.birthDate);
    addInput(form, "Церковь", "church", item.church);

    if (isParticipant) {
        addTextarea(form, "Мечта", "dream", item.dream);
        addTextarea(form, "Аллергии (через запятую)", "allergies", item.allergies);
        addInput(form, "Другие аллергии", "otherAllergy", item.otherAllergy);
        addInput(form, "Телефон родителя", "parentPhone", item.parentPhone);

        const consentLabel = document.createElement("label");
        consentLabel.className = "edit-checkbox";

        const consent = document.createElement("input");
        consent.type = "checkbox";
        consent.name = "allergyConsent";
        consent.checked = Boolean(item.allergyConsent);

        consentLabel.append(consent, document.createTextNode(
            " Согласие на обработку информации об аллергиях"
        ));

        form.appendChild(consentLabel);
    } else {
        addTextarea(form, "Направления служения (через запятую)", "serviceAreas", item.serviceAreas);
        addInput(form, "Другое направление", "otherService", item.otherService);
    }

    modalBody.appendChild(form);

    const cancelButton = document.createElement("button");
    cancelButton.type = "button";
    cancelButton.className = "modal-close-button";
    cancelButton.textContent = "Отмена";

    cancelButton.addEventListener("click", () => {
        editingRegistration = false;
        renderRegistrationDetails();
    });

    const saveButton = document.createElement("button");
    saveButton.type = "button";
    saveButton.className = "edit-button";
    saveButton.textContent = "Сохранить";

    saveButton.addEventListener("click", saveRegistration);

    const footer = document.querySelector(".modal-footer");
    footer.replaceChildren(cancelButton, saveButton);
}

async function saveRegistration() {
    const form = document.querySelector("#registrationEditForm");
    if (!form.reportValidity()) return;

    const formData = new FormData(form);
    const item = selectedRegistration;

    const data = {
        base: item,
        role: item.role,
        name: formData.get("name").trim(),
        phone: formData.get("phone").trim(),
        telegram: formData.has("noTelegram") ? "Нет Telegram" : (formData.get("telegram") || "").trim(),
        birthDate: (formData.get("birthDate") || "").replace(/^(\d{4})\.(\d{2})\.(\d{2})$/, "$1-$2-$3"),
        church: formData.get("church").trim(),
        dream: formData.get("dream") || "",
        parentPhone: formData.get("parentPhone") || "",
        allergies: (formData.get("allergies") || "")
            .split(",").map(x => x.trim()).filter(Boolean),
        otherAllergy: formData.get("otherAllergy") || "",
        allergyConsent: formData.get("allergyConsent") === "on",
        serviceAreas: (formData.get("serviceAreas") || "")
            .split(",").map(x => x.trim()).filter(Boolean),
        otherService: formData.get("otherService") || ""
    };

    try {
        const response = await fetch(
            `/api/admin/registrations/update?id=${encodeURIComponent(item.id)}`,
            {
                method: "PUT",
                headers: { "Content-Type": "application/json" },
                body: JSON.stringify(data)
            }
        );

        if (response.status === 401) {
            window.location.href = "/admin/login";
            return;
        }

        if (!response.ok) {
            const result = await response.json();
            throw new Error(result.message || "Ошибка сохранения");
        }

        alert("Анкета успешно обновлена!");

        closeRegistrationModal();
        await loadRegistrations();

    } catch (error) {
        alert(error.message);
    }
}

function closeRegistrationModal() {
    registrationModal.hidden = true;
    document.body.style.overflow = "";

    const footer = document.querySelector(".modal-footer");
    footer.innerHTML = `
        <button class="edit-button" id="editRegistrationButton">
            ✎ Редактировать
        </button>
        <button class="modal-close-button" id="closeModalFooter">
            Закрыть
        </button>
    `;

    // Повторно подключаем обработчики после восстановления кнопок
    footer.querySelector("#editRegistrationButton")
        .addEventListener("click", startEditing);

    footer.querySelector("#closeModalFooter")
        .addEventListener("click", closeRegistrationModal);
}

function startEditing() {
    editingRegistration = true;
    renderRegistrationDetails();
}

editRegistrationButton.addEventListener("click", startEditing);
closeModalButton.addEventListener("click", closeRegistrationModal);
closeModalFooter.addEventListener("click", closeRegistrationModal);

registrationModal.addEventListener("click", event => {
    if (event.target === registrationModal) {
        closeRegistrationModal();
    }
});

document.addEventListener("keydown", event => {
    if (event.key === "Escape" && !registrationModal.hidden) {
        closeRegistrationModal();
    }
});

// ===== DELETE REGISTRATION =====

async function deleteRegistration(item) {
    const confirmed = confirm(
        `Ты действительно хочешь удалить анкету ${item.name}?\n\nЭто действие нельзя отменить.`
    );

    if (!confirmed) return;

    try {
        const response = await fetch(
            `/api/admin/registrations/delete?id=${encodeURIComponent(item.id)}`,
            {
                method: "DELETE"
            }
        );

        if (response.status === 401) {
            window.location.href = "/admin/login";
            return;
        }

        if (!response.ok) {
            throw new Error("Не удалось удалить регистрацию");
        }

        alert("Анкета успешно удалена!");

        await loadRegistrations();

    } catch (error) {
        alert(error.message);
    }
}

// ===== FILTERS =====

filterButtons.forEach(button => {
    button.addEventListener("click", () => {
        currentFilter = button.dataset.filter;

        filterButtons.forEach(item => {
            item.classList.toggle("active", item === button);
        });

        renderRegistrations();
    });
});

// ===== SEARCH =====

searchInput.addEventListener("input", renderRegistrations);

// ===== REFRESH =====

refreshButton.addEventListener("click", loadRegistrations);

// ===== LOGOUT =====

logoutButton.addEventListener("click", async () => {
    try {
        const response = await fetch("/admin/logout", {
            method: "POST"
        });

        if (response.ok || response.redirected) {
            window.location.href = "/admin/login";
        }
    } catch (error) {
        alert("Не удалось выйти из аккаунта.");
    }
});

// ===== START =====

loadRegistrations();
