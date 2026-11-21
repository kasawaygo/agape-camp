const table = document.querySelector("#registrationTable");
const totalCount = document.querySelector("#totalCount");
const participantCount = document.querySelector("#participantCount");
const servantCount = document.querySelector("#servantCount");
const searchInput = document.querySelector("#searchInput");
const loadingMessage = document.querySelector("#loadingMessage");
const emptyMessage = document.querySelector("#emptyMessage");
const refreshButton = document.querySelector("#refreshButton");
const exportButton = document.querySelector("#exportButton");
const filterButtons = document.querySelectorAll(".filter");
const modal = document.querySelector("#registrationModal");
const modalTitle = document.querySelector("#modalTitle");
const modalBody = document.querySelector("#modalBody");
const departmentChoices = [
    ["Administration", "Administration (управление программой)"],
    ["Decoration", "Decoration (декорации, оформление)"],
    ["Kitchen (еда)", "Kitchen (приготовление еды)"],
    ["Kitchen (сценки)", "Kitchen (сценки)"],
    ["Palanka", "Palanka (подарки, письма)"],
    ["Room control", "Room control (порядок в корпусах)"],
    ["Rollo room", "Rollo room (основной зал, юлтон)"],
    ["Song Team", "Song Team (прославление)"],
    ["Chapel", "Chapel (молитва)"],
    ["Set up", "Set up (подготовка пространства)"]
];

let registrations = [];
let currentFilter = "all";

async function loadRegistrations() {
    loadingMessage.hidden = false;
    loadingMessage.textContent = "Загружаем анкеты TRES DIAS...";
    emptyMessage.hidden = true;
    table.replaceChildren();
    try {
        const response = await fetch("/api/admin/tres-dias/registrations");
        if (response.status === 401) return window.location.assign("/admin/login");
        if (!response.ok) throw new Error("Не удалось загрузить анкеты. Проверь подключение к базе данных.");
        registrations = await response.json();
        if (!Array.isArray(registrations)) throw new Error("Сервер вернул неожиданный ответ.");
        updateStatistics();
        renderRegistrations();
    } catch (error) {
        loadingMessage.textContent = error.message || "Не удалось загрузить анкеты.";
    }
}

function updateStatistics() {
    totalCount.textContent = registrations.length;
    participantCount.textContent = registrations.filter(item => item.formType === "participant").length;
    servantCount.textContent = registrations.filter(item => item.formType === "servant").length;
}

function renderRegistrations() {
    const query = searchInput.value.trim().toLocaleLowerCase("ru");
    const filtered = registrations.filter(item => {
        const matchesType = currentFilter === "all" || item.formType === currentFilter;
        const text = [item.name, item.phone, item.telegram, item.church, item.city].join(" ").toLocaleLowerCase("ru");
        return matchesType && text.includes(query);
    });
    table.replaceChildren();
    loadingMessage.hidden = true;
    emptyMessage.hidden = registrations.length !== 0;
    if (registrations.length > 0 && filtered.length === 0) {
        emptyMessage.textContent = "По этим условиям анкеты не найдены.";
        emptyMessage.hidden = false;
    } else {
        emptyMessage.textContent = "Пока нет отправленных анкет TRES DIAS. Анкеты лагеря находятся в соседнем разделе.";
    }

    filtered.forEach(item => {
        const row = document.createElement("tr");
        const values = [item.name, item.formType === "servant" ? "Служитель" : "Участник", item.city || "—", item.phone || "—", item.telegram || "—", item.clothingSize || "—", formatDate(item.createdAt)];
        values.forEach((value, index) => {
            const cell = document.createElement("td");
            if (index === 1) {
                const badge = document.createElement("span");
                badge.className = `role-tag ${item.formType === "servant" ? "servant" : "participant"}`;
                badge.textContent = value;
                cell.append(badge);
            } else cell.textContent = value;
            row.append(cell);
        });
        const actions = document.createElement("td");
        actions.className = "row-actions";
        const details = document.createElement("button");
        details.className = "details-button";
        details.textContent = "Подробнее";
        details.addEventListener("click", () => showDetails(item));
        const edit = document.createElement("button");
        edit.className = "edit-button";
        edit.textContent = "Изменить";
        edit.addEventListener("click", () => showEditor(item));
        const remove = document.createElement("button");
        remove.className = "delete-button";
        remove.textContent = "Удалить";
        remove.addEventListener("click", () => deleteRegistration(item));
        actions.append(details, edit, remove);
        row.append(actions);
        table.append(row);
    });
}

function inputBirthDate(value) {
    return /^\d{4}-\d{2}-\d{2}$/.test(value || "") ? value.replaceAll("-", ".") : "";
}

function dateToISO(value) {
    const match = /^(\d{4})\.(\d{2})\.(\d{2})$/.exec(value || "");
    return match ? `${match[1]}-${match[2]}-${match[3]}` : "";
}

function displayBirthDate(value) {
    return inputBirthDate(value) || "—";
}

function formatDate(value) {
    if (!value) return "—";
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? "—" : date.toLocaleDateString("ru-RU");
}

function showDetails(item) {
    modalTitle.textContent = `${item.formType === "servant" ? "Анкета служителя" : "Анкета участника"}: ${item.name}`;
    modalBody.replaceChildren();
    const details = [
        ["ФИО", item.name], ["Тип анкеты", item.formType === "servant" ? "Служитель" : "Участник"],
        ["Дата рождения", displayBirthDate(item.birthDate)], ["Город", item.city], ["Телефон", item.phone], ["Telegram", item.telegram],
        ["Церковь", item.church],
        ...(item.attendedBefore == null ? [] : [["Раньше участвовал(а)", item.attendedBefore ? "Да" : "Нет"]]),
        ["Размер одежды", item.clothingSize], ["Департаменты", (item.departments || []).join(", ") || "—"],
        ["Предыдущее служение", item.previousDepartments], ["Читал(а) Рое", item.readRoe],
        ["Аллергии", item.hasAllergies == null ? "Не указано (старая анкета)" : item.hasAllergies ? "Да" : "Нет"],
        ["Подробности аллергии", item.allergyDetails], ["Согласие на обработку сведений об аллергии", item.allergyConsent ? "Да" : "Нет"],
        ["Дата регистрации", item.createdAt ? new Date(item.createdAt).toLocaleString("ru-RU") : "—"]
    ];
    const section = document.createElement("section");
    section.className = "modal-section";
    details.forEach(([label, value]) => {
        const line = document.createElement("div");
        line.className = "modal-field";
        const caption = document.createElement("span"); caption.textContent = label;
        const answer = document.createElement("strong"); answer.textContent = value || "—";
        line.append(caption, answer); section.append(line);
    });
    modalBody.append(section);
    openModal();
}

function showEditor(item) {
    const role = item.formType === "servant" ? "служителя" : "участника";
    modalTitle.textContent = `Редактировать анкету ${role}`;
    const departmentHtml = departmentChoices.map(([value, label]) => `<label class="edit-check"><input type="checkbox" name="departments" value="${value}"><span>${label}</span></label>`).join("");
    modalBody.innerHTML = `
        <form class="edit-form" id="editRegistrationForm">
            <label class="edit-field"><span>ФИО *</span><input name="name" maxlength="120" required></label>
            <label class="edit-field"><span>Дата рождения ${item.formType === "participant" ? "*" : ""}</span><small>Формат: ГГГГ.ММ.ДД</small><input name="birthDate" type="text" inputmode="numeric" maxlength="10" placeholder="ГГГГ.ММ.ДД" ${item.formType === "participant" ? "required" : ""}></label>
            <label class="edit-field"><span>Город *</span><input name="city" maxlength="100" required></label>
            <label class="edit-field"><span>Телефон *</span><input name="phone" type="tel" required></label>
            <label class="edit-field"><span>Telegram</span><input name="telegram" type="text" placeholder="@username" autocomplete="off" autocapitalize="none" spellcheck="false" minlength="6" maxlength="33" pattern="@[A-Za-z][A-Za-z0-9_]{4,31}" title="Укажи Telegram username начиная с @"></label>
            <label class="edit-field"><span>Церковь *</span><input name="church" maxlength="160" required></label>
            <label class="edit-field"><span>Размер одежды *</span><select name="clothingSize" required><option value="">Выбрать</option>${["S","M","L","XL","2XL","3XL","4XL","5XL"].map(size => `<option value="${size}">${size}</option>`).join("")}</select></label>
            <fieldset class="edit-field" ${item.formType === "servant" ? "" : "hidden"}><legend>Департаменты *</legend><small>Можно выбрать от 1 до 10.</small><div class="edit-check-grid">${departmentHtml}</div><p class="inline-error" id="editDepartmentError" hidden>Выбери от 1 до 10 департаментов.</p></fieldset>
            <label class="edit-field" ${item.formType === "servant" ? "" : "hidden"}><span>Предыдущее служение *</span><textarea name="previousDepartments" rows="3" maxlength="1200"></textarea></label>
            <label class="edit-field" ${item.formType === "servant" ? "" : "hidden"}><span>Читал(а) Рое? *</span><textarea name="readRoe" rows="3" maxlength="1200"></textarea></label>
            <fieldset class="edit-field"><legend>Есть ли аллергии? *</legend><label class="radio"><input type="radio" name="hasAllergies" value="false" required> Нет</label><label class="radio"><input type="radio" name="hasAllergies" value="true"> Да</label><small>Если да, перечисли аллергены.</small><textarea name="allergyDetails" rows="3" maxlength="1500" placeholder="Аллергены и важные подробности"></textarea><label class="consent"><input type="checkbox" name="allergyConsent"> Согласие на обработку сведений об аллергиях получено</label></fieldset>
            <p class="edit-message" aria-live="polite"></p>
            <div class="edit-actions"><button type="submit">Сохранить изменения</button><button type="button" class="modal-close-button" id="cancelEditButton">Отмена</button></div>
        </form>`;
    const form = document.querySelector("#editRegistrationForm");
    form.elements.name.value = item.name || "";
    form.elements.birthDate.value = inputBirthDate(item.birthDate);
    form.elements.city.value = item.city || "";
    form.elements.phone.value = item.phone || "";
    form.elements.telegram.value = item.telegram || "";
    form.elements.church.value = item.church || "";
    form.elements.clothingSize.value = item.clothingSize || "";
    form.elements.previousDepartments.value = item.previousDepartments || "";
    form.elements.readRoe.value = item.readRoe || "";
    (item.departments || []).forEach(value => { const checkbox = [...form.querySelectorAll('[name="departments"]')].find(input => input.value === value); if (checkbox) checkbox.checked = true; });
    if (item.hasAllergies != null) form.querySelector(`[name="hasAllergies"][value="${item.hasAllergies}"]`).checked = true;
    form.elements.allergyDetails.value = item.allergyDetails || "";
    form.elements.allergyConsent.checked = Boolean(item.allergyConsent);
    syncEditorAllergy(form);
    form.elements.birthDate.addEventListener("input", event => {
        const digits = event.target.value.replace(/\D/g, "").slice(0, 8);
        event.target.value = digits.length > 4 ? `${digits.slice(0,4)}.${digits.slice(4,6)}${digits.length > 6 ? `.${digits.slice(6)}` : ""}` : digits;
    });
    [...form.querySelectorAll('[name="hasAllergies"]')].forEach(input => input.addEventListener("change", () => syncEditorAllergy(form)));
    form.querySelector("#cancelEditButton").addEventListener("click", closeModal);
    form.addEventListener("submit", event => saveEdit(event, item, form));
    openModal();
}

function syncEditorAllergy(form) {
    const yes = form.querySelector('[name="hasAllergies"]:checked')?.value === "true";
    form.elements.allergyDetails.disabled = !yes;
    form.elements.allergyDetails.required = yes;
    if (!yes) form.elements.allergyDetails.value = "";
}

async function saveEdit(event, item, form) {
    event.preventDefault();
    const message = form.querySelector(".edit-message");
    const departments = [...form.querySelectorAll('[name="departments"]:checked')].map(input => input.value);
    if (item.formType === "servant" && (departments.length < 1 || departments.length > departmentChoices.length)) {
        form.querySelector("#editDepartmentError").hidden = false;
        return;
    }
    const data = new FormData(form);
    const allergyChoice = data.get("hasAllergies");
    const payload = {
        formType: item.formType,
        name: data.get("name"),
        birthDate: dateToISO(data.get("birthDate")),
        city: data.get("city"), phone: data.get("phone"), telegram: data.get("telegram"), church: data.get("church"),
        attendedBefore: item.attendedBefore,
        clothingSize: data.get("clothingSize"), departments,
        previousDepartments: data.get("previousDepartments") || "", readRoe: data.get("readRoe") || "",
        hasAllergies: allergyChoice === "true", allergyDetails: data.get("allergyDetails") || "",
        allergyConsent: data.has("allergyConsent")
    };
    const dateInput = form.elements.birthDate;
    if (dateInput.value && !payload.birthDate) { dateInput.setCustomValidity("Введи дату как ГГГГ.ММ.ДД."); dateInput.reportValidity(); dateInput.setCustomValidity(""); return; }
    const button = form.querySelector('[type="submit"]'); button.disabled = true; button.textContent = "Сохраняем…";
    try {
        const response = await fetch(`/api/admin/tres-dias/registrations?id=${encodeURIComponent(item.id)}`, { method: "PUT", headers: { "Content-Type": "application/json" }, body: JSON.stringify(payload) });
        const result = await response.json();
        if (response.status === 401) return window.location.assign("/admin/login");
        if (!response.ok) throw new Error(result.message || "Не удалось сохранить изменения.");
        closeModal(); await loadRegistrations();
    } catch (error) { message.textContent = error.message; message.className = "edit-message error"; }
    finally { button.disabled = false; button.textContent = "Сохранить изменения"; }
}

function openModal() { modal.hidden = false; document.body.style.overflow = "hidden"; }
function closeModal() { modal.hidden = true; document.body.style.overflow = ""; }

async function deleteRegistration(item) {
    if (!window.confirm(`Удалить анкету ${item.name}? Это действие нельзя отменить.`)) return;
    try {
        const response = await fetch(`/api/admin/tres-dias/registrations?id=${encodeURIComponent(item.id)}`, { method: "DELETE" });
        if (response.status === 401) return window.location.assign("/admin/login");
        if (!response.ok) throw new Error("Не удалось удалить анкету.");
        registrations = registrations.filter(entry => entry.id !== item.id);
        updateStatistics(); renderRegistrations();
    } catch (error) { window.alert(error.message); }
}

searchInput.addEventListener("input", renderRegistrations);
filterButtons.forEach(button => button.addEventListener("click", () => {
    currentFilter = button.dataset.filter;
    filterButtons.forEach(item => item.classList.toggle("active", item === button));
    renderRegistrations();
}));
refreshButton.addEventListener("click", loadRegistrations);
exportButton.addEventListener("click", () => { window.location.href = "/api/admin/tres-dias/registrations/export"; });
document.querySelector("#logoutButton").addEventListener("click", async () => { await fetch("/admin/logout", { method: "POST" }); window.location.href = "/admin/login"; });
document.querySelector("#closeModalButton").addEventListener("click", closeModal);
document.querySelector("#closeModalFooter").addEventListener("click", closeModal);
modal.addEventListener("click", event => { if (event.target === modal) closeModal(); });
document.addEventListener("keydown", event => { if (event.key === "Escape" && !modal.hidden) closeModal(); });
loadRegistrations();
