package main

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/xuri/excelize/v2"
)

type TresDiasRegistration struct {
	Base                   map[string]any `json:"base,omitempty"`
	ID                     string         `json:"id"`
	FormType               string         `json:"formType"`
	Name                   string         `json:"name"`
	BirthDate              string         `json:"birthDate"`
	City                   string         `json:"city"`
	Phone                  string         `json:"phone"`
	Telegram               string         `json:"telegram"`
	Church                 string         `json:"church"`
	AttendedBefore         *bool          `json:"attendedBefore"`
	ClothingSize           string         `json:"clothingSize"`
	HasAllergies           *bool          `json:"hasAllergies"`
	AllergyDetails         string         `json:"allergyDetails"`
	PersonalDataConsent    *bool          `json:"personalDataConsent"`
	NeedsTransport         *bool          `json:"needsTransport"`
	CloseOnesParticipating *bool          `json:"closeOnesParticipating"`
	Departments            []string       `json:"departments"`
	PreviousDepartments    string         `json:"previousDepartments"`
	ReadRoe                string         `json:"readRoe"`
	CreatedAt              time.Time      `json:"createdAt"`
}

var tresDiasSizes = map[string]bool{"S": true, "M": true, "L": true, "XL": true, "2XL": true, "3XL": true, "4XL": true, "5XL": true}

func ensureTresDiasTransportColumn(ctx context.Context, pool *pgxpool.Pool) error {
	_, err := pool.Exec(ctx, `ALTER TABLE public.tres_dias_registrations
		ADD COLUMN IF NOT EXISTS needs_transport boolean,
		ADD COLUMN IF NOT EXISTS personal_data_consent boolean,
		ADD COLUMN IF NOT EXISTS close_ones_participating boolean`)
	return err
}

func validateTresDiasRegistration(r *TresDiasRegistration, requireTransport bool) string {
	r.FormType = strings.TrimSpace(r.FormType)
	r.Name = strings.TrimSpace(r.Name)
	r.BirthDate = strings.TrimSpace(r.BirthDate)
	r.City = strings.TrimSpace(r.City)
	r.Phone = strings.TrimSpace(r.Phone)
	r.Telegram = strings.TrimSpace(r.Telegram)
	r.Church = strings.TrimSpace(r.Church)
	r.ClothingSize = strings.ToUpper(strings.TrimSpace(r.ClothingSize))
	r.AllergyDetails = strings.TrimSpace(r.AllergyDetails)
	r.PreviousDepartments = strings.TrimSpace(r.PreviousDepartments)
	r.ReadRoe = strings.TrimSpace(r.ReadRoe)
	if r.FormType != "participant" && r.FormType != "servant" {
		return "Выбери анкету участника или служителя."
	}
	if r.Name == "" || r.City == "" || r.Phone == "" || r.Church == "" || !tresDiasSizes[r.ClothingSize] || r.HasAllergies == nil || r.PersonalDataConsent == nil || !*r.PersonalDataConsent {
		return "Заполни обязательные поля и выбери размер одежды."
	}
	if *r.HasAllergies && r.AllergyDetails == "" {
		return "Укажи аллергию и важные подробности или выбери «Нет»."
	}
	if !*r.HasAllergies {
		r.AllergyDetails = ""
	}
	if utf8.RuneCountInString(r.Name) > 120 || utf8.RuneCountInString(r.City) > 100 || utf8.RuneCountInString(r.Church) > 160 || utf8.RuneCountInString(r.AllergyDetails) > 1500 {
		return "Проверь длину введённых данных."
	}
	if !validPhone(r.Phone) {
		return "Проверь номер телефона."
	}
	if r.Telegram != "" && !validTelegramChoice(r.Telegram) {
		return "Укажи Telegram username начиная с @."
	}
	if r.BirthDate != "" {
		date, err := time.Parse("2006.01.02", r.BirthDate)
		if err != nil || date.After(time.Now()) {
			return "Проверь дату рождения. Используй формат ГГГГ.ММ.ДД."
		}
		r.BirthDate = date.Format("2006-01-02")
	}
	if r.FormType == "participant" {
		if r.BirthDate == "" {
			return "Укажи дату рождения в формате ГГГГ.ММ.ДД."
		}
		r.Departments = []string{}
		r.PreviousDepartments = ""
		r.ReadRoe = ""
		r.NeedsTransport = nil
		r.CloseOnesParticipating = nil
		return ""
	}
	if requireTransport && r.NeedsTransport == nil {
		return "Укажи, нужен ли тебе транспорт."
	}
	if requireTransport && r.CloseOnesParticipating == nil {
		return "Ответь, едут ли с тобой близкие люди."
	}
	r.AttendedBefore = nil
	allowed := map[string]bool{"Administration": true, "Decoration": true, "Kitchen (еда)": true, "Kitchen (сценки)": true, "Palanka": true, "Room control": true, "Rollo room": true, "Song Team": true, "Chapel": true, "Set up": true, "Refreshment": true, "Photo/Video": true, "Media": true}
	if len(r.Departments) < 1 || len(r.Departments) > len(allowed) {
		return "Выбери хотя бы один департамент из списка."
	}
	seen := map[string]bool{}
	for i, item := range r.Departments {
		r.Departments[i] = strings.TrimSpace(item)
		if !allowed[r.Departments[i]] || seen[r.Departments[i]] {
			return "Проверь выбранные департаменты."
		}
		seen[r.Departments[i]] = true
	}
	if r.PreviousDepartments == "" || r.ReadRoe == "" {
		return "Заполни вопросы о предыдущем служении и книге Рое."
	}
	if utf8.RuneCountInString(r.PreviousDepartments) > 1200 || utf8.RuneCountInString(r.ReadRoe) > 1200 {
		return "Один из ответов слишком длинный."
	}
	return ""
}

func registerTresDiasRoutes(mux *http.ServeMux, pool *pgxpool.Pool) {
	mux.HandleFunc("/tres-dias", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/tres-dias/", http.StatusSeeOther)
	})
	mux.HandleFunc("/tres-dias/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tres-dias/" {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, "./web/tres-dias/index.html")
	})
	mux.HandleFunc("/tres-dias/participant", func(w http.ResponseWriter, r *http.Request) {
		serveTresDiasPage(w, r, "./web/tres-dias/participant.html")
	})
	mux.HandleFunc("/tres-dias/servant", func(w http.ResponseWriter, r *http.Request) { serveTresDiasPage(w, r, "./web/tres-dias/servant.html") })
	mux.Handle("/tres-dias/assets/", http.StripPrefix("/tres-dias/assets/", http.FileServer(http.Dir("./web/tres-dias/assets"))))

	mux.HandleFunc("/admin/tres-dias", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isAgapeAdmin(r) {
			http.Redirect(w, r, "/admin/login", http.StatusSeeOther)
			return
		}
		http.ServeFile(w, r, "./web/admin/tres-dias.html")
	})
	mux.HandleFunc("/admin/tres-dias.js", func(w http.ResponseWriter, r *http.Request) { serveTresDiasPage(w, r, "./web/admin/tres-dias.js") })
	mux.HandleFunc("/admin/tres-dias.css", func(w http.ResponseWriter, r *http.Request) { serveTresDiasPage(w, r, "./web/admin/tres-dias.css") })

	mux.HandleFunc("/api/tres-dias/register", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"message": "Method not allowed"})
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
		var reg TresDiasRegistration
		dec := json.NewDecoder(r.Body)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&reg); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "Некорректные данные анкеты."})
			return
		}
		if message := validateTresDiasRegistration(&reg, true); message != "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": message})
			return
		}
		if reg.Telegram == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"message": "Укажи Telegram username начиная с @."})
			return
		}
		_, err := pool.Exec(r.Context(), `INSERT INTO public.tres_dias_registrations (form_type,name,birth_date,city,phone,telegram,church,attended_before,clothing_size,departments,previous_departments,read_roe,has_allergies,allergy_details,allergy_consent,personal_data_consent,needs_transport,close_ones_participating) VALUES ($1,$2,NULLIF($3,'')::date,$4,$5,$6,$7,$8,$9,$10,NULLIF($11,''),NULLIF($12,''),$13,NULLIF($14,''),$15,$16,$17,$18)`, reg.FormType, reg.Name, reg.BirthDate, reg.City, reg.Phone, reg.Telegram, reg.Church, reg.AttendedBefore, reg.ClothingSize, reg.Departments, reg.PreviousDepartments, reg.ReadRoe, reg.HasAllergies, reg.AllergyDetails, *reg.PersonalDataConsent, *reg.PersonalDataConsent, reg.NeedsTransport, reg.CloseOnesParticipating)
		if err != nil {
			log.Printf("TRES DIAS registration insert failed: %v", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "Не удалось сохранить анкету. Попробуй ещё раз."})
			return
		}
		writeJSON(w, http.StatusCreated, map[string]string{"message": "Анкета отправлена. Спасибо!"})
	})

	mux.HandleFunc("/api/admin/tres-dias/registrations", func(w http.ResponseWriter, r *http.Request) {
		if !isAgapeAdmin(r) {
			writeJSON(w, http.StatusUnauthorized, map[string]string{"message": "Unauthorized"})
			return
		}
		switch r.Method {
		case http.MethodGet:
			items, err := listTresDiasRegistrations(r.Context(), pool)
			if err != nil {
				log.Printf("TRES DIAS registrations query failed: %v", err)
				writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "Не удалось загрузить анкеты TRES DIAS."})
				return
			}
			writeJSON(w, http.StatusOK, items)
		case http.MethodDelete:
			id := r.URL.Query().Get("id")
			if id == "" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"message": "Не указан ID анкеты."})
				return
			}
			tag, err := pool.Exec(r.Context(), `DELETE FROM public.tres_dias_registrations WHERE id=$1`, id)
			if err != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]string{"message": "Не удалось удалить анкету."})
				return
			}
			if tag.RowsAffected() == 0 {
				writeJSON(w, http.StatusNotFound, map[string]string{"message": "Анкета не найдена."})
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"message": "Анкета удалена."})
		case http.MethodPut:
			id := strings.TrimSpace(r.URL.Query().Get("id"))
			if id == "" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"message": "Не указан ID анкеты."})
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 1<<20)
			var reg TresDiasRegistration
			dec := json.NewDecoder(r.Body)
			dec.DisallowUnknownFields()
			if err := dec.Decode(&reg); err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"message": "Некорректные данные анкеты."})
				return
			}
			// Editing an existing response does not mean the admin is collecting
			// consent again. Keep the submitted consent value as-is while using
			// the normal validation rules for all other fields.
			personalDataConsent := reg.PersonalDataConsent
			consentGranted := true
			reg.PersonalDataConsent = &consentGranted
			if message := validateTresDiasRegistration(&reg, false); message != "" {
				writeJSON(w, http.StatusBadRequest, map[string]string{"message": message})
				return
			}
			reg.PersonalDataConsent = personalDataConsent
			adminSyncEdit(w, r, pool, "tres", id, reg.Base, reg)
		default:
			writeJSON(w, http.StatusMethodNotAllowed, map[string]string{"message": "Method not allowed"})
		}
	})
	mux.HandleFunc("/api/admin/tres-dias/registrations/export", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !isAgapeAdmin(r) {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		items, err := listTresDiasRegistrations(r.Context(), pool)
		if err != nil {
			http.Error(w, "Could not export registrations", http.StatusInternalServerError)
			return
		}
		book := excelize.NewFile()
		defer book.Close()
		_ = book.SetSheetName("Sheet1", "TRES DIAS 2027")
		_, _ = book.NewSheet("Участники")
		_, _ = book.NewSheet("Служители")
		headers := []any{"ID", "Анкета", "ФИО", "Дата рождения", "Город", "Телефон", "Telegram", "Церковь", "Раньше участвовал(а)", "Размер", "Департаменты", "Прошлое служение", "Рое", "Аллергии", "Подробности аллергии", "Согласие на обработку персональных данных", "Нужен транспорт", "Родственники/друзья/ячейка едут участниками", "Дата регистрации"}
		for _, sheet := range []string{"TRES DIAS 2027", "Участники", "Служители"} {
			for col, value := range headers {
				cell, _ := excelize.CoordinatesToCellName(col+1, 1)
				_ = book.SetCellValue(sheet, cell, value)
			}
		}
		rowNum := map[string]int{"TRES DIAS 2027": 2, "Участники": 2, "Служители": 2}
		for _, item := range items {
			attended := "—"
			if item.AttendedBefore != nil {
				if *item.AttendedBefore {
					attended = "Да"
				} else {
					attended = "Нет"
				}
			}
			date, _ := time.Parse("2006-01-02", item.BirthDate)
			dateText := ""
			if !date.IsZero() {
				dateText = date.Format("2006.01.02")
			}
			allergies := "Не указано"
			if item.HasAllergies != nil {
				if *item.HasAllergies {
					allergies = "Да"
				} else {
					allergies = "Нет"
				}
			}
			consent := "Не указано"
			if item.PersonalDataConsent != nil {
				consent = "Нет"
				if *item.PersonalDataConsent {
					consent = "Да"
				}
			}
			transport := "Не указано"
			if item.NeedsTransport != nil {
				transport = "Нет"
				if *item.NeedsTransport {
					transport = "Да"
				}
			}
			closeOnes := "Не указано"
			if item.CloseOnesParticipating != nil {
				closeOnes = "Нет"
				if *item.CloseOnesParticipating {
					closeOnes = "Да"
				}
			}
			data := []any{item.ID, tresDiasLabel(item.FormType), item.Name, dateText, item.City, item.Phone, item.Telegram, item.Church, attended, item.ClothingSize, strings.Join(item.Departments, ", "), item.PreviousDepartments, item.ReadRoe, allergies, item.AllergyDetails, consent, transport, closeOnes, item.CreatedAt.Format("02.01.2006 15:04")}
			individualSheet := "Участники"
			if item.FormType == "servant" {
				individualSheet = "Служители"
			}
			for _, sheet := range []string{"TRES DIAS 2027", individualSheet} {
				for col, value := range data {
					cell, _ := excelize.CoordinatesToCellName(col+1, rowNum[sheet])
					_ = book.SetCellValue(sheet, cell, value)
				}
				rowNum[sheet]++
			}
		}
		for _, sheet := range []string{"TRES DIAS 2027", "Участники", "Служители"} {
			_ = book.SetColWidth(sheet, "A", "S", 20)
			_ = book.SetColWidth(sheet, "O", "O", 40)
		}
		var out bytes.Buffer
		if err := book.Write(&out); err != nil {
			http.Error(w, "Could not create Excel file", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
		w.Header().Set("Content-Disposition", `attachment; filename="tres_dias_2027_registrations.xlsx"`)
		_, _ = w.Write(out.Bytes())
	})
}

func serveTresDiasPage(w http.ResponseWriter, r *http.Request, path string) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.ServeFile(w, r, path)
}

func isAgapeAdmin(r *http.Request) bool {
	session, err := sessionStore.Get(r, "agape_admin")
	return err == nil && session.Values["authenticated"] == true
}

func listTresDiasRegistrations(ctx context.Context, pool *pgxpool.Pool) ([]TresDiasRegistration, error) {
	rows, err := pool.Query(ctx, `SELECT id::text,form_type,name,COALESCE(birth_date::text,''),city,phone,COALESCE(telegram,''),church,attended_before,clothing_size,departments,COALESCE(previous_departments,''),COALESCE(read_roe,''),has_allergies,COALESCE(allergy_details,''),personal_data_consent,needs_transport,close_ones_participating,created_at FROM public.tres_dias_registrations ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]TresDiasRegistration, 0)
	for rows.Next() {
		var item TresDiasRegistration
		if err := rows.Scan(&item.ID, &item.FormType, &item.Name, &item.BirthDate, &item.City, &item.Phone, &item.Telegram, &item.Church, &item.AttendedBefore, &item.ClothingSize, &item.Departments, &item.PreviousDepartments, &item.ReadRoe, &item.HasAllergies, &item.AllergyDetails, &item.PersonalDataConsent, &item.NeedsTransport, &item.CloseOnesParticipating, &item.CreatedAt); err != nil {
			return nil, err
		}
		if item.Departments == nil {
			item.Departments = []string{}
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func tresDiasLabel(formType string) string {
	if formType == "servant" {
		return "Служитель"
	}
	return "Участник"
}
