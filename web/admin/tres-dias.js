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

let registrations = [];
let currentFilter = "all";

async function loadRegistrations() {
    loadingMessage.hidden = false;
    loadingMessage.textContent = "Загружаем анкеты TRES DIAS...";
    emptyMessage.hidden = true;
    table.replaceChildren();
    try {
        const response = await fetch("/api/admin/tres-dias/registrations");
        if (response.status === 401) {
            window.location.href = "/admin/login";
            return;
        }
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
        const text = [item.name, item.phone, item.church, item.city].join(" ").toLocaleLowerCase("ru");
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
        const role = item.formType === "servant" ? "Служитель" : "Участник";
        const values = [item.name, role, item.city || "—", item.phone || "—", item.clothingSize || "—", formatDate(item.createdAt)];
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
        const details = document.createElement("button");
        details.className = "details-button";
        details.textContent = "Подробнее →";
        details.addEventListener("click", () => showDetails(item));
        const remove = document.createElement("button");
        remove.className = "delete-button";
        remove.textContent = "Удалить";
        remove.addEventListener("click", () => deleteRegistration(item));
        actions.append(details, remove);
        row.append(actions);
        table.append(row);
    });
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
        ["Дата рождения", item.birthDate || "—"], ["Город", item.city], ["Телефон", item.phone],
        ["Церковь", item.church], ["Участвовал(а) ранее", item.attendedBefore == null ? "—" : item.attendedBefore ? "Да" : "Нет"],
        ["Размер одежды", item.clothingSize], ["Департаменты", (item.departments || []).join(", ") || "—"],
        ["Предыдущее служение", item.previousDepartments], ["Читал(а) Рое", item.readRoe],
        ["Пожелания по размещению", item.accommodationNotes], ["Дата регистрации", item.createdAt ? new Date(item.createdAt).toLocaleString("ru-RU") : "—"]
    ];
    const section = document.createElement("section");
    section.className = "modal-section";
    details.forEach(([label, value]) => {
        const line = document.createElement("div");
        line.className = "modal-field";
        const caption = document.createElement("span");
        caption.textContent = label;
        const answer = document.createElement("strong");
        answer.textContent = value || "—";
        line.append(caption, answer);
        section.append(line);
    });
    modalBody.append(section);
    modal.hidden = false;
    document.body.style.overflow = "hidden";
}

function closeModal() {
    modal.hidden = true;
    document.body.style.overflow = "";
}

async function deleteRegistration(item) {
    if (!window.confirm(`Удалить анкету ${item.name}? Это действие нельзя отменить.`)) return;
    try {
        const response = await fetch(`/api/admin/tres-dias/registrations?id=${encodeURIComponent(item.id)}`, { method: "DELETE" });
        if (response.status === 401) return window.location.assign("/admin/login");
        if (!response.ok) throw new Error("Не удалось удалить анкету.");
        registrations = registrations.filter(entry => entry.id !== item.id);
        updateStatistics();
        renderRegistrations();
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
document.querySelector("#logoutButton").addEventListener("click", async () => {
    await fetch("/admin/logout", { method: "POST" });
    window.location.href = "/admin/login";
});
document.querySelector("#closeModalButton").addEventListener("click", closeModal);
document.querySelector("#closeModalFooter").addEventListener("click", closeModal);
modal.addEventListener("click", event => { if (event.target === modal) closeModal(); });
document.addEventListener("keydown", event => { if (event.key === "Escape" && !modal.hidden) closeModal(); });

loadRegistrations();
