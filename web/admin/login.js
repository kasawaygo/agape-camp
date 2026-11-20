const loginForm = document.querySelector("#loginForm");
const passwordInput = document.querySelector("#password");
const loginButton = document.querySelector("#loginButton");
const loginMessage = document.querySelector("#loginMessage");

loginForm.addEventListener("submit", async (event) => {
    event.preventDefault();

    const password = passwordInput.value;

    loginButton.disabled = true;
    loginButton.textContent = "Проверяем...";
    loginMessage.textContent = "";

    try {
        const response = await fetch("/admin/login", {
            method: "POST",
            headers: {
                "Content-Type": "application/json"
            },
            body: JSON.stringify({ password })
        });

        const result = await response.json();

        if (!response.ok) {
            throw new Error(result.message || "Ошибка входа");
        }

        window.location.href = "/admin";

    } catch (error) {
        loginMessage.textContent =
            error.message || "Не удалось выполнить вход";

        passwordInput.value = "";
        passwordInput.focus();

    } finally {
        loginButton.disabled = false;
        loginButton.textContent = "Войти";
    }
});
