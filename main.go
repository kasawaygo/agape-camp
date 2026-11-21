package main

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/gorilla/sessions"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/xuri/excelize/v2"
	"strings"
)

type Registration struct {
	Role           string   `json:"role"`
	Name           string   `json:"name"`
	Phone          string   `json:"phone"`
	Telegram       string   `json:"telegram"`
	BirthDate      string   `json:"birthDate"`
	Church         string   `json:"church"`
	Dream          string   `json:"dream"`
	ParentPhone    string   `json:"parentPhone"`
	Allergies      []string `json:"allergies"`
	OtherAllergy   string   `json:"otherAllergy"`
	AllergyConsent bool     `json:"allergyConsent"`
	ServiceAreas   []string `json:"serviceAreas"`
	OtherService   string   `json:"otherService"`
}

type LoginRequest struct {
	Password string `json:"password"`
}

var (
	telegramRegex = regexp.MustCompile(`^@[A-Za-z][A-Za-z0-9_]{4,31}$`)
	phoneRegex    = regexp.MustCompile(`^[0-9+() -]{8,20}$`)
	digitsRegex   = regexp.MustCompile(`[0-9]`)
)

func validPhone(phone string) bool {
	phone = strings.TrimSpace(phone)

	return phoneRegex.MatchString(phone) &&
		len(digitsRegex.FindAllString(phone, -1)) >= 8
}

func validateRegistration(data *Registration) string {
	// Убираем пробелы по краям
	data.Role = strings.TrimSpace(data.Role)
	data.Name = strings.TrimSpace(data.Name)
	data.Phone = strings.TrimSpace(data.Phone)
	data.Telegram = strings.TrimSpace(data.Telegram)
	data.BirthDate = strings.TrimSpace(data.BirthDate)
	data.Church = strings.TrimSpace(data.Church)
	data.Dream = strings.TrimSpace(data.Dream)
	data.ParentPhone = strings.TrimSpace(data.ParentPhone)
	data.OtherAllergy = strings.TrimSpace(data.OtherAllergy)
	data.OtherService = strings.TrimSpace(data.OtherService)

	for i := range data.Allergies {
		data.Allergies[i] = strings.TrimSpace(data.Allergies[i])
	}

	for i := range data.ServiceAreas {
		data.ServiceAreas[i] = strings.TrimSpace(data.ServiceAreas[i])
	}

	// Проверяем роль
	if data.Role != "participant" && data.Role != "servant" {
		return "Invalid role"
	}

	// Обязательные поля
	if data.Name == "" || data.Phone == "" ||
		data.Telegram == "" || data.BirthDate == "" ||
		data.Church == "" {
		return "Please fill in all required fields"
	}

	// Ограничение длины
	if utf8.RuneCountInString(data.Name) > 100 {
		return "Name is too long"
	}

	if utf8.RuneCountInString(data.Church) > 100 {
		return "Church name is too long"
	}

	if utf8.RuneCountInString(data.Dream) > 1000 {
		return "Dream text is too long"
	}

	if utf8.RuneCountInString(data.OtherAllergy) > 200 ||
		utf8.RuneCountInString(data.OtherService) > 200 {
		return "Additional text is too long"
	}

	// Телефон
	if !validPhone(data.Phone) {
		return "Invalid phone number"
	}

	// Telegram
	if !telegramRegex.MatchString(data.Telegram) {
		return "Invalid Telegram username"
	}

	// Дата рождения
	birthDate, err := time.Parse("2006-01-02", data.BirthDate)
	if err != nil || birthDate.After(time.Now()) {
		return "Invalid birth date"
	}

	// Ограничение количества элементов
	if len(data.Allergies) > 20 || len(data.ServiceAreas) > 20 {
		return "Too many selected options"
	}

	// Проверяем участника
	if data.Role == "participant" {
		if data.ParentPhone == "" {
			return "Parent phone is required"
		}

		if !validPhone(data.ParentPhone) {
			return "Invalid parent phone number"
		}

		if len(data.Allergies) > 0 && !data.AllergyConsent {
			return "Allergy consent is required"
		}
	}

	// Проверяем служителя
	if data.Role == "servant" && len(data.ServiceAreas) == 0 {
		return "Select at least one service area"
	}

	return ""
}

func originProtection(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// GET, HEAD и OPTIONS не изменяют данные
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}

		origin := r.Header.Get("Origin")

		// Если Origin отсутствует, проверяем Referer
		if origin == "" {
			referer := r.Header.Get("Referer")
			if referer != "" {
				u, err := url.Parse(referer)
				if err == nil {
					origin = u.Scheme + "://" + u.Host
				}
			}
		}

		u, err := url.Parse(origin)
		production := os.Getenv("RENDER") == "true" ||
			strings.EqualFold(os.Getenv("APP_ENV"), "production")

		if err != nil ||
			u.Host == "" ||
			u.User != nil ||
			(u.Scheme != "http" && u.Scheme != "https") ||
			(production && u.Scheme != "https") ||
			!strings.EqualFold(u.Host, r.Host) ||
			(r.TLS != nil && u.Scheme != "https") {

			writeJSON(w, http.StatusForbidden, map[string]string{
				"message": "Request origin not allowed",
			})
			return
		}

		next.ServeHTTP(w, r)
	})
}

type loginAttempt struct {
	count        int
	windowStart  time.Time
	blockedUntil time.Time
}

var (
	loginAttempts = make(map[string]*loginAttempt)
	loginMutex    sync.Mutex
)

func getClientIP(r *http.Request) string {
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func isLoginBlocked(ip string) bool {
	loginMutex.Lock()
	defer loginMutex.Unlock()

	attempt, exists := loginAttempts[ip]
	if !exists {
		return false
	}

	if time.Now().Before(attempt.blockedUntil) {
		return true
	}

	if !attempt.blockedUntil.IsZero() {
		delete(loginAttempts, ip)
		return false
	}

	if time.Since(attempt.windowStart) >= 15*time.Minute {
		delete(loginAttempts, ip)
		return false
	}

	return false
}

func recordLoginFailure(ip string) {
	loginMutex.Lock()
	defer loginMutex.Unlock()

	now := time.Now()
	attempt, exists := loginAttempts[ip]

	if !exists || now.Sub(attempt.windowStart) >= 15*time.Minute {
		loginAttempts[ip] = &loginAttempt{
			count:       1,
			windowStart: now,
		}
		return
	}

	attempt.count++

	if attempt.count >= 5 {
		attempt.blockedUntil = now.Add(15 * time.Minute)
	}
}

func resetLoginAttempts(ip string) {
	loginMutex.Lock()
	defer loginMutex.Unlock()

	delete(loginAttempts, ip)
}

var sessionStore *sessions.CookieStore

func main() {
	// Загружаем настройки
	if err := godotenv.Load(); err != nil {
		log.Println("Warning: .env file not found")
	}

	adminPassword := strings.TrimSpace(os.Getenv("ADMIN_PASSWORD"))
	sessionSecret := os.Getenv("SESSION_SECRET")

	if adminPassword == "" || sessionSecret == "" {
		log.Fatal("ADMIN_PASSWORD and SESSION_SECRET must be configured")
	}

	// Настраиваем сессии
	sessionStore = sessions.NewCookieStore([]byte(sessionSecret))
	secureCookie := os.Getenv("COOKIE_SECURE") == "true" ||
		os.Getenv("RENDER") == "true" ||
		strings.EqualFold(os.Getenv("APP_ENV"), "production")
	sessionStore.Options = &sessions.Options{
		Path:     "/",
		MaxAge:   8 * 60 * 60,
		HttpOnly: true,
		Secure:   secureCookie,
		SameSite: http.SameSiteLaxMode,
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Подключаем PostgreSQL
	dbHost := strings.TrimSpace(os.Getenv("DB_HOST"))
	dbPort := strings.TrimSpace(os.Getenv("DB_PORT"))
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbName := strings.TrimSpace(os.Getenv("DB_NAME"))
	dbSSLMode := strings.TrimSpace(os.Getenv("DB_SSLMODE"))
	if dbHost == "" || dbPort == "" || dbUser == "" || dbPassword == "" || dbName == "" || dbSSLMode == "" {
		log.Fatal("DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME and DB_SSLMODE must be configured")
	}
	dbURL := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(dbUser, dbPassword),
		Host:   net.JoinHostPort(dbHost, dbPort),
		Path:   "/" + dbName,
	}
	query := dbURL.Query()
	query.Set("sslmode", dbSSLMode)
	dbURL.RawQuery = query.Encode()
	connString := dbURL.String()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, connString)
	if err != nil {
		log.Fatal("Database configuration failed:", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		log.Fatal("Database connection failed:", err)
	}

	log.Println("Successfully connected to Supabase!")

	mux := http.NewServeMux()

	// Render uses this endpoint to confirm the service and database are ready.
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	// =========================
	// ADMIN LOGIN PAGE
	// =========================

	mux.HandleFunc("/admin/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			http.ServeFile(w, r, "./web/admin/login.html")
			return
		}

		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
				"message": "Method not allowed",
			})
			return
		}

		ip := getClientIP(r)

		if isLoginBlocked(ip) {
			writeJSON(w, http.StatusTooManyRequests, map[string]string{
				"message": "Too many login attempts. Try again in 15 minutes.",
			})
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 4096)

		var data LoginRequest
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"message": "Invalid request",
			})
			return
		}

		if subtle.ConstantTimeCompare(
			[]byte(data.Password),
			[]byte(adminPassword),
		) != 1 {
			recordLoginFailure(ip)

			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"message": "Неверный пароль",
			})
			return
		}

		resetLoginAttempts(ip)

		session, err := sessionStore.Get(r, "agape_admin")
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"message": "Session error",
			})
			return
		}

		session.Values["authenticated"] = true

		if err := session.Save(r, w); err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"message": "Could not create session",
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{
			"message": "Login successful",
		})
	})

	// =========================
	// ADMIN STATIC FILES
	// =========================

	adminFiles := http.StripPrefix(
		"/admin/",
		http.FileServer(http.Dir("./web/admin")),
	)

	mux.Handle("/admin/login.css", adminFiles)
	mux.Handle("/admin/login.js", adminFiles)
	mux.Handle("/admin/dashboard.css", adminFiles)
	mux.Handle("/admin/dashboard.js", adminFiles)

	// =========================
	// PROTECTED ADMIN DASHBOARD
	// =========================

	mux.HandleFunc("/admin", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		session, err := sessionStore.Get(r, "agape_admin")

		if err != nil || session.Values["authenticated"] != true {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}

		http.ServeFile(w, r, "./web/admin/dashboard.html")
	})

	mux.HandleFunc("/admin/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/admin/" {
			http.Redirect(w, r, "/admin", http.StatusSeeOther)
			return
		}

		http.NotFound(w, r)
	})

	// =========================
	// ADMIN LOGOUT
	// =========================

	mux.HandleFunc("/admin/logout", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		session, err := sessionStore.Get(r, "agape_admin")
		if err != nil {
			http.Error(w, "Session error", http.StatusInternalServerError)
			return
		}

		session.Options.MaxAge = -1
		delete(session.Values, "authenticated")

		if err := session.Save(r, w); err != nil {
			http.Error(w, "Could not end session", http.StatusInternalServerError)
			return
		}

		http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
	})

	// =========================
	// REGISTRATION API
	// =========================

	mux.HandleFunc("/api/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
				"message": "Method not allowed",
			})
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

		var data Registration
		if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"message": "Invalid request data",
			})
			return
		}

		if message := validateRegistration(&data); message != "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"message": message,
			})
			return
		}

		query := `
		INSERT INTO public.registrations (
			role, name, phone, telegram, birth_date, church,
			dream, parent_phone, allergies, other_allergy,
			allergy_consent, service_areas, other_service
		)
		VALUES (
			$1, $2, $3, $4, $5, $6,
			$7, $8, $9, $10, $11, $12, $13
		)
	`

		_, err := pool.Exec(r.Context(), query,
			data.Role,
			data.Name,
			data.Phone,
			data.Telegram,
			data.BirthDate,
			data.Church,
			data.Dream,
			nullableString(data.ParentPhone),
			data.Allergies,
			nullableString(data.OtherAllergy),
			data.AllergyConsent,
			data.ServiceAreas,
			nullableString(data.OtherService),
		)

		if err != nil {
			log.Println("Registration insert failed:", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"message": "Could not save registration",
			})
			return
		}

		writeJSON(w, http.StatusCreated, map[string]string{
			"message": "Registration saved successfully",
		})
	})

	// =========================
	// ADMIN REGISTRATIONS API
	// =========================

	mux.HandleFunc("/api/admin/registrations", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
				"message": "Method not allowed",
			})
			return
		}

		// Проверяем авторизацию
		session, err := sessionStore.Get(r, "agape_admin")
		if err != nil || session.Values["authenticated"] != true {
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"message": "Unauthorized",
			})
			return
		}

		// Получаем регистрации из Supabase
		rows, err := pool.Query(r.Context(), `
        SELECT
            id,
            role,
            name,
            phone,
            telegram,
            birth_date,
            church,
            dream,
            parent_phone,
            allergies,
            other_allergy,
            allergy_consent,
            service_areas,
            other_service,
            created_at
        FROM public.registrations
        ORDER BY created_at DESC
    `)

		if err != nil {
			log.Println("Failed to fetch registrations:", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"message": "Could not fetch registrations",
			})
			return
		}
		defer rows.Close()

		registrations := make([]map[string]any, 0)

		for rows.Next() {
			var (
				id             string
				role           string
				name           string
				phone          string
				telegram       string
				birthDate      time.Time
				church         string
				dream          *string
				parentPhone    *string
				allergies      []byte
				otherAllergy   *string
				allergyConsent bool
				serviceAreas   []string
				otherService   *string
				createdAt      time.Time
			)

			err := rows.Scan(
				&id,
				&role,
				&name,
				&phone,
				&telegram,
				&birthDate,
				&church,
				&dream,
				&parentPhone,
				&allergies,
				&otherAllergy,
				&allergyConsent,
				&serviceAreas,
				&otherService,
				&createdAt,
			)

			if err != nil {
				log.Println("Failed to scan registration:", err)
				writeJSON(w, http.StatusInternalServerError, map[string]string{
					"message": "Could not read registration",
				})
				return
			}

			registrations = append(registrations, map[string]any{
				"id":             id,
				"role":           role,
				"name":           name,
				"phone":          phone,
				"telegram":       telegram,
				"birthDate":      birthDate.Format("2006-01-02"),
				"church":         church,
				"dream":          dream,
				"parentPhone":    parentPhone,
				"allergies":      json.RawMessage(allergies),
				"otherAllergy":   otherAllergy,
				"allergyConsent": allergyConsent,
				"serviceAreas":   serviceAreas,
				"otherService":   otherService,
				"createdAt":      createdAt,
			})
		}

		if err := rows.Err(); err != nil {
			log.Println("Registration rows error:", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"message": "Could not read registrations",
			})
			return
		}

		writeJSON(w, http.StatusOK, registrations)
	})

	// =========================
	// DELETE REGISTRATION
	// =========================

	mux.HandleFunc("/api/admin/registrations/delete", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodDelete {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
				"message": "Method not allowed",
			})
			return
		}

		// Проверяем авторизацию
		session, err := sessionStore.Get(r, "agape_admin")
		if err != nil || session.Values["authenticated"] != true {
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"message": "Unauthorized",
			})
			return
		}

		// Получаем ID регистрации
		id := r.URL.Query().Get("id")

		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"message": "Registration ID is required",
			})
			return
		}

		// Удаляем регистрацию
		tag, err := pool.Exec(
			r.Context(),
			"DELETE FROM public.registrations WHERE id = $1",
			id,
		)

		if err != nil {
			log.Println("Failed to delete registration:", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"message": "Could not delete registration",
			})
			return
		}

		if tag.RowsAffected() == 0 {
			writeJSON(w, http.StatusNotFound, map[string]string{
				"message": "Registration not found",
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{
			"message": "Registration deleted successfully",
		})
	})

	// =========================
	// UPDATE REGISTRATION
	// =========================

	mux.HandleFunc("/api/admin/registrations/update", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
				"message": "Method not allowed",
			})
			return
		}

		// Проверяем авторизацию
		session, err := sessionStore.Get(r, "agape_admin")
		if err != nil || session.Values["authenticated"] != true {
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"message": "Unauthorized",
			})
			return
		}

		// Получаем ID
		id := r.URL.Query().Get("id")
		if id == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"message": "Registration ID is required",
			})
			return
		}

		// Получаем новые данные
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)

		var reg Registration
		if err := json.NewDecoder(r.Body).Decode(&reg); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"message": "Invalid request data",
			})
			return
		}

		if message := validateRegistration(&reg); message != "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"message": message,
			})
			return
		}

		// Обновляем запись в Supabase
		_, err = pool.Exec(r.Context(), `
        UPDATE public.registrations
        SET
            role = $1,
            name = $2,
            phone = $3,
            telegram = $4,
            birth_date = $5,
            church = $6,
            dream = $7,
            parent_phone = $8,
            allergies = $9::jsonb,
            other_allergy = $10,
            allergy_consent = $11,
            service_areas = $12,
            other_service = $13
        WHERE id = $14
    `,
			reg.Role,
			reg.Name,
			reg.Phone,
			reg.Telegram,
			reg.BirthDate,
			reg.Church,
			nullableString(reg.Dream),
			nullableString(reg.ParentPhone),
			mustJSON(reg.Allergies),
			nullableString(reg.OtherAllergy),
			reg.AllergyConsent,
			reg.ServiceAreas,
			nullableString(reg.OtherService),
			id,
		)

		if err != nil {
			log.Println("Failed to update registration:", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{
				"message": "Could not update registration",
			})
			return
		}

		writeJSON(w, http.StatusOK, map[string]string{
			"message": "Registration updated successfully",
		})
	})

	// =========================
	// EXPORT REGISTRATIONS TO EXCEL
	// =========================

	mux.HandleFunc("/api/admin/registrations/export", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{
				"message": "Method not allowed",
			})
			return
		}

		// Проверяем авторизацию
		session, err := sessionStore.Get(r, "agape_admin")
		if err != nil || session.Values["authenticated"] != true {
			writeJSON(w, http.StatusUnauthorized, map[string]string{
				"message": "Unauthorized",
			})
			return
		}

		// Получаем данные из Supabase
		rows, err := pool.Query(r.Context(), `
        SELECT
            id, role, name, phone, telegram, birth_date,
            church, dream, parent_phone, allergies,
            other_allergy, allergy_consent, service_areas,
            other_service, created_at
        FROM public.registrations
        ORDER BY created_at DESC
    `)

		if err != nil {
			log.Println("Export query error:", err)
			http.Error(w, "Could not export registrations", http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		file := excelize.NewFile()

		err = file.SetSheetName("Sheet1", "Все регистрации")
		if err != nil {
			http.Error(w, "Excel error", http.StatusInternalServerError)
			return
		}

		_, err = file.NewSheet("Участники")
		if err != nil {
			http.Error(w, "Excel error", http.StatusInternalServerError)
			return
		}

		_, err = file.NewSheet("Служители")
		if err != nil {
			http.Error(w, "Excel error", http.StatusInternalServerError)
			return
		}

		headers := []any{
			"ID", "Роль", "Имя", "Телефон", "Telegram",
			"Дата рождения", "Церковь", "Мечта",
			"Телефон родителя", "Аллергии", "Другие аллергии",
			"Согласие", "Направления служения",
			"Другое служение", "Дата регистрации",
		}

		sheets := []string{
			"Все регистрации",
			"Участники",
			"Служители",
		}

		// Заголовки таблиц
		for _, sheet := range sheets {
			for col, value := range headers {
				cell, _ := excelize.CoordinatesToCellName(col+1, 1)
				file.SetCellValue(sheet, cell, value)
			}
		}

		nextRow := map[string]int{
			"Все регистрации": 2,
			"Участники":       2,
			"Служители":       2,
		}

		// Запись одной строки в Excel
		writeRow := func(sheet string, row int, values []any) {
			for col, value := range values {
				cell, _ := excelize.CoordinatesToCellName(col+1, row)
				file.SetCellValue(sheet, cell, value)
			}
		}

		for rows.Next() {
			var (
				id             string
				role           string
				name           string
				phone          string
				telegram       string
				birthDate      time.Time
				church         string
				dream          *string
				parentPhone    *string
				allergies      []byte
				otherAllergy   *string
				allergyConsent bool
				serviceAreas   []string
				otherService   *string
				createdAt      time.Time
			)

			err := rows.Scan(
				&id, &role, &name, &phone, &telegram,
				&birthDate, &church, &dream, &parentPhone,
				&allergies, &otherAllergy, &allergyConsent,
				&serviceAreas, &otherService, &createdAt,
			)

			if err != nil {
				log.Println("Export scan error:", err)
				http.Error(w, "Could not read registrations", http.StatusInternalServerError)
				return
			}

			allergyList := []string{}
			if len(allergies) > 0 {
				if err := json.Unmarshal(allergies, &allergyList); err != nil {
					log.Println("Allergy JSON error:", err)
				}
			}

			roleName := "Служитель"
			if role == "participant" {
				roleName = "Участник"
			}

			var dreamValue, parentValue, otherAllergyValue, otherServiceValue string

			if dream != nil {
				dreamValue = *dream
			}
			if parentPhone != nil {
				parentValue = *parentPhone
			}
			if otherAllergy != nil {
				otherAllergyValue = *otherAllergy
			}
			if otherService != nil {
				otherServiceValue = *otherService
			}

			consent := "Нет"
			if allergyConsent {
				consent = "Да"
			}

			data := []any{
				id,
				roleName,
				name,
				phone,
				telegram,
				birthDate.Format("2006-01-02"),
				church,
				dreamValue,
				parentValue,
				strings.Join(allergyList, ", "),
				otherAllergyValue,
				consent,
				strings.Join(serviceAreas, ", "),
				otherServiceValue,
				createdAt.Format("02.01.2006 15:04"),
			}

			writeRow("Все регистрации", nextRow["Все регистрации"], data)
			nextRow["Все регистрации"]++

			if role == "participant" {
				writeRow("Участники", nextRow["Участники"], data)
				nextRow["Участники"]++
			} else if role == "servant" {
				writeRow("Служители", nextRow["Служители"], data)
				nextRow["Служители"]++
			}
		}

		if err := rows.Err(); err != nil {
			log.Println("Export rows error:", err)
			http.Error(w, "Could not export registrations", http.StatusInternalServerError)
			return
		}

		// Настраиваем ширину колонок
		for _, sheet := range sheets {
			file.SetColWidth(sheet, "A", "O", 20)
			file.SetColWidth(sheet, "H", "H", 35)
		}

		// Формируем Excel-файл
		buffer, err := file.WriteToBuffer()
		if err != nil {
			log.Println("Excel generation error:", err)
			http.Error(w, "Could not generate Excel file", http.StatusInternalServerError)
			return
		}

		w.Header().Set(
			"Content-Type",
			"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
		)
		w.Header().Set(
			"Content-Disposition",
			`attachment; filename="agape_registrations.xlsx"`,
		)

		w.WriteHeader(http.StatusOK)
		w.Write(buffer.Bytes())
	})

	// =========================
	// PUBLIC FRONTEND
	// =========================
	registerTresDiasRoutes(mux, pool)

	mux.Handle("/", http.FileServer(http.Dir("./web")))

	log.Println("AGAPE CAMP server running on port", port)
	server := &http.Server{
		Addr:              ":" + port,
		Handler:           originProtection(mux),
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Fatal(server.ListenAndServe())
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func mustJSON(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		log.Println("JSON encoding error:", err)
		return "[]"
	}

	return string(data)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
