package main

// The Google bridge is opt-in. Public registration never calls Google: a
// scheduled Apps Script reconciles the database and the two workbooks.
import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type syncField struct {
	Key      string `json:"key"`
	Label    string `json:"label"`
	Kind     string `json:"kind"`
	Editable bool   `json:"editable"`
}
type syncSchema struct {
	Table  string
	Fields []syncField
}

var syncSchemas = map[string]syncSchema{
	"camp": {"registrations", []syncField{
		{"id", "ID", "text", false}, {"name", "ФИО", "text", true}, {"role", "Анкета", "role", false},
		{"birth_date", "Дата рождения", "date", true}, {"phone", "Телефон", "text", true}, {"telegram", "Telegram", "text", true},
		{"church", "Церковь", "text", true}, {"dream", "Мечта", "text", true}, {"parent_phone", "Телефон родителей", "text", true},
		{"allergies", "Аллергии", "list", true}, {"other_allergy", "Другие аллергии", "text", true},
		{"service_areas", "Служения", "list", true}, {"other_service", "Другое служение", "text", true},
		{"allergy_consent", "Согласие на сведения об аллергии", "bool", false}, {"created_at", "Дата заявки", "text", false},
	}},
	"tres": {"tres_dias_registrations", []syncField{
		{"id", "ID", "text", false}, {"name", "ФИО", "text", true}, {"form_type", "Анкета", "role", false},
		{"birth_date", "Дата рождения", "date", true}, {"city", "Город", "text", true}, {"phone", "Телефон", "text", true},
		{"telegram", "Telegram", "text", true}, {"church", "Церковь", "text", true}, {"clothing_size", "Размер одежды", "text", true},
		{"departments", "Департаменты", "list", true}, {"previous_departments", "Служили ранее", "text", true}, {"read_roe", "Рое", "text", true},
		{"has_allergies", "Есть аллергии", "bool", true}, {"allergy_details", "Подробности аллергий", "text", true},
		{"needs_transport", "Нужен транспорт", "bool", true}, {"close_ones_participating", "Едут близкие", "bool", true},
		{"personal_data_consent", "Согласие на обработку данных", "bool", false}, {"created_at", "Дата заявки", "text", false},
	}},
}

type syncEdit struct {
	ID        string         `json:"id"`
	RequestID string         `json:"requestId"`
	Base      map[string]any `json:"base"`
	Changes   map[string]any `json:"changes"`
}
type syncConflict struct {
	Field    string `json:"field"`
	Base     any    `json:"base"`
	Current  any    `json:"current"`
	Proposed any    `json:"proposed"`
}

func syncAuthorized(r *http.Request) bool {
	token := os.Getenv("SHEETS_SYNC_TOKEN")
	if len(token) < 32 {
		return false
	}
	want, got := sha256.Sum256([]byte("Bearer "+token)), sha256.Sum256([]byte(r.Header.Get("Authorization")))
	return subtle.ConstantTimeCompare(want[:], got[:]) == 1
}

func syncNormalize(f syncField, v any) (any, error) {
	switch f.Kind {
	case "text", "date", "role":
		if v == nil {
			return "", nil
		}
		s, ok := v.(string)
		if !ok {
			return nil, fmt.Errorf("%s: требуется текст", f.Label)
		}
		s = strings.TrimSpace(s)
		if len(s) > 12000 {
			return nil, fmt.Errorf("%s: слишком длинный текст", f.Label)
		}
		if f.Kind == "date" {
			s = strings.ReplaceAll(s, ".", "-")
			if s != "" {
				if _, err := time.Parse("2006-01-02", s); err != nil {
					return nil, fmt.Errorf("%s: формат ГГГГ.ММ.ДД", f.Label)
				}
			}
		}
		return s, nil
	case "bool":
		if v == nil {
			return nil, nil
		}
		if _, ok := v.(bool); ok {
			return v, nil
		}
		return nil, fmt.Errorf("%s: выбери Да или Нет", f.Label)
	case "list":
		if v == nil {
			return []any{}, nil
		}
		a, ok := v.([]any)
		if !ok {
			return nil, fmt.Errorf("%s: требуется список", f.Label)
		}
		out := make([]any, 0, len(a))
		for _, x := range a {
			s, ok := x.(string)
			if !ok {
				return nil, fmt.Errorf("%s: некорректный список", f.Label)
			}
			s = strings.TrimSpace(s)
			if s != "" {
				out = append(out, s)
			}
		}
		return out, nil
	}
	return v, nil
}

// Three-way merge: preserve other administrators' edits; reject the entire
// mutation if the same field changed differently. Replays are harmless.
func mergeSync(schema syncSchema, current map[string]any, edit syncEdit) (map[string]any, []syncConflict, error) {
	merged := map[string]any{}
	for k, v := range current {
		merged[k] = v
	}
	allowed := map[string]syncField{}
	for _, f := range schema.Fields {
		allowed[f.Key] = f
	}
	conflicts := []syncConflict{}
	for k, v := range edit.Changes {
		f, ok := allowed[k]
		if !ok || !f.Editable {
			return nil, nil, fmt.Errorf("Поле %s нельзя менять", k)
		}
		b, ok := edit.Base[k]
		if !ok {
			return nil, nil, fmt.Errorf("Нет исходного значения %s. Обнови анкету", k)
		}
		proposed, err := syncNormalize(f, v)
		if err != nil {
			return nil, nil, err
		}
		base, err := syncNormalize(f, b)
		if err != nil {
			return nil, nil, err
		}
		now, err := syncNormalize(f, current[k])
		if err != nil {
			return nil, nil, err
		}
		if reflect.DeepEqual(proposed, base) {
			continue
		}
		if !reflect.DeepEqual(now, base) && !reflect.DeepEqual(now, proposed) {
			conflicts = append(conflicts, syncConflict{k, base, now, proposed})
			continue
		}
		merged[k] = proposed
	}
	sort.Slice(conflicts, func(i, j int) bool { return conflicts[i].Field < conflicts[j].Field })
	return merged, conflicts, nil
}

// Reuse the same validation as the public forms, without collecting consent
// again or allowing editors to manufacture a participant's consent.
func validateSync(kind string, row map[string]any) error {
	mapping := map[string]string{"birth_date": "birthDate", "parent_phone": "parentPhone", "other_allergy": "otherAllergy", "allergy_consent": "allergyConsent", "service_areas": "serviceAreas", "other_service": "otherService", "form_type": "formType", "clothing_size": "clothingSize", "previous_departments": "previousDepartments", "read_roe": "readRoe", "has_allergies": "hasAllergies", "allergy_details": "allergyDetails", "personal_data_consent": "personalDataConsent", "needs_transport": "needsTransport", "close_ones_participating": "closeOnesParticipating"}
	payload := map[string]any{}
	for k, v := range row {
		if dst, ok := mapping[k]; ok {
			payload[dst] = v
		} else {
			payload[k] = v
		}
	}
	raw, _ := json.Marshal(payload)
	if kind == "camp" {
		var reg Registration
		if err := json.Unmarshal(raw, &reg); err != nil {
			return err
		}
		if msg := validateRegistration(&reg); msg != "" {
			return errors.New(msg)
		}
	} else {
		var reg TresDiasRegistration
		if err := json.Unmarshal(raw, &reg); err != nil {
			return err
		}
		reg.BirthDate = strings.ReplaceAll(reg.BirthDate, "-", ".")
		yes := true
		reg.PersonalDataConsent = &yes
		if msg := validateTresDiasRegistration(&reg, false); msg != "" {
			return errors.New(msg)
		}
		// Normalize conditional fields in the same way as an admin edit.
		row["allergy_details"] = reg.AllergyDetails
		if reg.FormType == "participant" {
			row["departments"] = []any{}
			row["previous_departments"] = nil
			row["read_roe"] = nil
			row["needs_transport"] = nil
			row["close_ones_participating"] = nil
		}
	}
	return nil
}

func applySyncEdit(ctx context.Context, pool *pgxpool.Pool, kind string, edit syncEdit, source string) (map[string]any, []syncConflict, error) {
	schema, ok := syncSchemas[kind]
	if !ok {
		return nil, nil, errors.New("Неизвестная анкета")
	}
	if edit.ID == "" || len(edit.ID) > 100 || len(edit.Changes) > 30 {
		return nil, nil, errors.New("Некорректный запрос")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, nil, err
	}
	defer tx.Rollback(ctx)
	if len(edit.RequestID) > 100 {
		return nil, nil, errors.New("Некорректный номер изменения")
	}
	audit := len(os.Getenv("SHEETS_SYNC_TOKEN")) >= 32
	if audit && edit.RequestID != "" {
		if _, err = tx.Exec(ctx, "SELECT pg_advisory_xact_lock(hashtextextended($1,0))", kind+":"+edit.RequestID); err != nil {
			return nil, nil, err
		}
		var saved []byte
		err = tx.QueryRow(ctx, "SELECT after_data FROM public.registration_sync_audit WHERE kind=$1 AND request_id=$2", kind, edit.RequestID).Scan(&saved)
		if err == nil {
			var row map[string]any
			err = json.Unmarshal(saved, &row)
			return row, nil, err
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			return nil, nil, err
		}
	}
	var raw []byte
	// The row lock serializes edits from either website and Google Sheets.
	err = tx.QueryRow(ctx, "SELECT to_jsonb(r) FROM public."+schema.Table+" r WHERE id::text=$1 FOR UPDATE", edit.ID).Scan(&raw)
	if err != nil {
		return nil, nil, err
	}
	current := map[string]any{}
	if err = json.Unmarshal(raw, &current); err != nil {
		return nil, nil, err
	}
	merged, conflicts, err := mergeSync(schema, current, edit)
	if err != nil || len(conflicts) > 0 {
		return current, conflicts, err
	}
	if err = validateSync(kind, merged); err != nil {
		return current, nil, err
	}
	columns := []string{}
	for _, f := range schema.Fields {
		if f.Editable && !reflect.DeepEqual(current[f.Key], merged[f.Key]) {
			columns = append(columns, f.Key)
		}
	}
	if len(columns) > 0 {
		encoded, _ := json.Marshal(merged)
		names := strings.Join(columns, ",")
		_, err = tx.Exec(ctx, "UPDATE public."+schema.Table+" SET ("+names+") = (SELECT "+names+" FROM jsonb_populate_record(NULL::public."+schema.Table+", $1::jsonb)) WHERE id::text=$2", string(encoded), edit.ID)
		if err != nil {
			return nil, nil, err
		}
	}
	if audit {
		after, _ := json.Marshal(merged)
		_, err = tx.Exec(ctx, "INSERT INTO public.registration_sync_audit(kind,registration_id,request_id,source,before_data,after_data) VALUES($1,$2,NULLIF($3,''),$4,$5::jsonb,$6::jsonb)", kind, edit.ID, edit.RequestID, source, string(raw), string(after))
		if err != nil {
			return nil, nil, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return nil, nil, err
	}
	return merged, nil, nil
}

var syncJSONNames = map[string]string{"birth_date": "birthDate", "parent_phone": "parentPhone", "other_allergy": "otherAllergy", "allergy_consent": "allergyConsent", "service_areas": "serviceAreas", "other_service": "otherService", "form_type": "formType", "clothing_size": "clothingSize", "previous_departments": "previousDepartments", "read_roe": "readRoe", "has_allergies": "hasAllergies", "allergy_details": "allergyDetails", "personal_data_consent": "personalDataConsent", "needs_transport": "needsTransport", "close_ones_participating": "closeOnesParticipating"}

func adminSyncEdit(w http.ResponseWriter, r *http.Request, pool *pgxpool.Pool, kind, id string, base map[string]any, proposed any) {
	if base == nil {
		writeJSON(w, 409, map[string]string{"message": "Обнови страницу и открой анкету заново перед редактированием."})
		return
	}
	raw, _ := json.Marshal(proposed)
	var values map[string]any
	_ = json.Unmarshal(raw, &values)
	edit := syncEdit{ID: id, Base: map[string]any{}, Changes: map[string]any{}}
	for _, f := range syncSchemas[kind].Fields {
		if !f.Editable {
			continue
		}
		key := f.Key
		if v, ok := syncJSONNames[key]; ok {
			key = v
		}
		v, ok := values[key]
		if !ok {
			continue
		}
		b, ok := base[key]
		if !ok {
			writeJSON(w, 409, map[string]string{"message": "Обнови анкету: отсутствует исходное поле " + f.Label})
			return
		}
		edit.Base[f.Key] = b
		edit.Changes[f.Key] = v
	}
	_, conflicts, err := applySyncEdit(r.Context(), pool, kind, edit, "admin")
	if len(conflicts) > 0 {
		writeJSON(w, 409, map[string]any{"message": "Анкету уже изменили. Твои правки не затёрты: оставь окно открытым и сравни данные с обновлённой анкетой в другой вкладке.", "conflicts": conflicts})
		return
	}
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, 404, map[string]string{"message": "Анкета не найдена"})
		return
	}
	if err != nil {
		writeJSON(w, 422, map[string]string{"message": safeSyncError(err)})
		return
	}
	writeJSON(w, 200, map[string]string{"message": "Изменения сохранены"})
}

func registerSheetsSync(mux *http.ServeMux, pool *pgxpool.Pool) {
	mux.HandleFunc("/api/sheets-sync", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if !syncAuthorized(r) {
			writeJSON(w, 401, map[string]string{"message": "Unauthorized"})
			return
		}
		kind := r.URL.Query().Get("kind")
		schema, ok := syncSchemas[kind]
		if !ok {
			writeJSON(w, 400, map[string]string{"message": "Unknown form"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
		defer cancel()
		switch r.Method {
		case http.MethodGet:
			rows, err := pool.Query(ctx, "SELECT to_jsonb(r) FROM public."+schema.Table+" r ORDER BY created_at,id")
			if err != nil {
				writeJSON(w, 503, map[string]string{"message": "База временно недоступна"})
				return
			}
			defer rows.Close()
			items := []map[string]any{}
			for rows.Next() {
				var raw []byte
				if err = rows.Scan(&raw); err != nil {
					break
				}
				row := map[string]any{}
				if err = json.Unmarshal(raw, &row); err != nil {
					break
				}
				filtered := map[string]any{}
				for _, f := range schema.Fields {
					filtered[f.Key] = row[f.Key]
				}
				items = append(items, filtered)
			}
			if err != nil || rows.Err() != nil {
				writeJSON(w, 503, map[string]string{"message": "Не удалось прочитать анкеты"})
				return
			}
			writeJSON(w, 200, map[string]any{"fields": schema.Fields, "rows": items})
		case http.MethodPost:
			r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
			var edit syncEdit
			d := json.NewDecoder(r.Body)
			d.DisallowUnknownFields()
			if d.Decode(&edit) != nil {
				writeJSON(w, 400, map[string]string{"message": "Некорректный запрос"})
				return
			}
			row, conflicts, err := applySyncEdit(ctx, pool, kind, edit, "sheets")
			if errors.Is(err, pgx.ErrNoRows) {
				writeJSON(w, 404, map[string]string{"message": "Анкета удалена на сайте"})
				return
			}
			if len(conflicts) > 0 {
				writeJSON(w, 409, map[string]any{"message": "Конфликт: это поле уже изменили", "conflicts": conflicts, "row": row})
				return
			}
			if err != nil {
				writeJSON(w, 422, map[string]string{"message": "Изменения не сохранены: " + safeSyncError(err)})
				return
			}
			writeJSON(w, 200, map[string]any{"row": row})
		default:
			writeJSON(w, 405, map[string]string{"message": "Method not allowed"})
		}
	})
}

func safeSyncError(err error) string {
	// Database failures must never expose connection details or row contents.
	var sqlErr interface{ SQLState() string }
	if errors.As(err, &sqlErr) {
		return "проверь значения полей; ошибка базы данных"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "время ожидания истекло; повтори позже"
	}
	return err.Error()
}
