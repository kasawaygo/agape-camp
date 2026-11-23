package main

import (
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestSyncMergeConcurrentEdits(t *testing.T) {
	schema := syncSchemas["camp"]
	current := map[string]any{"name": "Новое имя", "phone": "010-1111-2222"}
	edit := syncEdit{Base: map[string]any{"name": "Старое имя", "phone": "010-1111-2222"}, Changes: map[string]any{"name": "Старое имя", "phone": "010-3333-4444"}}
	got, conflicts, err := mergeSync(schema, current, edit)
	if err != nil || len(conflicts) != 0 || got["name"] != "Новое имя" || got["phone"] != "010-3333-4444" {
		t.Fatalf("independent edits lost: %v %v %v", got, conflicts, err)
	}
	if current["phone"] != "010-1111-2222" {
		t.Fatal("mutated input")
	}
	edit.Changes["name"] = "Третье имя"
	_, conflicts, err = mergeSync(schema, current, edit)
	if err != nil || len(conflicts) != 1 || conflicts[0].Field != "name" || conflicts[0].Current != "Новое имя" || conflicts[0].Proposed != "Третье имя" {
		t.Fatalf("conflict lost: %v %v", conflicts, err)
	}
}
func TestSyncReplayAndNormalization(t *testing.T) {
	current := map[string]any{"phone": "010-3333-4444", "birth_date": "2000-01-09", "dream": nil, "allergies": nil}
	edit := syncEdit{Base: map[string]any{"phone": "010-1111-2222", "birth_date": "2000.01.09", "dream": "", "allergies": []any{}}, Changes: map[string]any{"phone": "010-3333-4444", "birth_date": "2000.01.09", "dream": "", "allergies": []any{}}}
	got, c, err := mergeSync(syncSchemas["camp"], current, edit)
	if err != nil || len(c) > 0 || !reflect.DeepEqual(current, got) {
		t.Fatalf("replay not stable: %v %v %v", got, c, err)
	}
}
func TestSyncProtectedFieldsAndMissingBase(t *testing.T) {
	for _, field := range []string{"id", "role", "allergy_consent", "created_at", "name; DROP TABLE registrations", "unknown"} {
		_, _, err := mergeSync(syncSchemas["camp"], map[string]any{}, syncEdit{Base: map[string]any{field: "old"}, Changes: map[string]any{field: "new"}})
		if err == nil {
			t.Fatalf("accepted protected field %q", field)
		}
	}
	_, _, err := mergeSync(syncSchemas["camp"], map[string]any{}, syncEdit{Changes: map[string]any{"name": "new"}})
	if err == nil {
		t.Fatal("accepted missing baseline")
	}
}
func TestSyncAuthenticationFailClosed(t *testing.T) {
	r := httptest.NewRequest("GET", "/api/sheets-sync?kind=camp", nil)
	t.Setenv("SHEETS_SYNC_TOKEN", "")
	if syncAuthorized(r) {
		t.Fatal("enabled without token")
	}
	t.Setenv("SHEETS_SYNC_TOKEN", "01234567890123456789012345678901ab")
	if syncAuthorized(r) {
		t.Fatal("missing authorization accepted")
	}
	r.Header.Set("Authorization", "Bearer 01234567890123456789012345678901ac")
	if syncAuthorized(r) {
		t.Fatal("wrong token accepted")
	}
	r.Header.Set("Authorization", "Bearer 01234567890123456789012345678901ab")
	if !syncAuthorized(r) {
		t.Fatal("valid token rejected")
	}
}
func TestSyncDatesAndTypes(t *testing.T) {
	for _, v := range []any{"2000.02.30", "09.01.2000", true} {
		if _, err := syncNormalize(syncField{Kind: "date"}, v); err == nil {
			t.Fatalf("accepted date %v", v)
		}
	}
	if v, err := syncNormalize(syncField{Kind: "date"}, "2000.02.29"); err != nil || v != "2000-02-29" {
		t.Fatalf("rejected leap date %v %v", v, err)
	}
	if _, err := syncNormalize(syncField{Kind: "bool"}, "false"); err == nil {
		t.Fatal("string coerced to bool")
	}
}
