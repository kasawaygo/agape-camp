package main

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

// Explicit opt-in. Uses disposable tables containing only synthetic records;
// no reads or writes of existing registration rows.
func TestSyncDatabaseConcurrency(t *testing.T) {
	if os.Getenv("AGAPE_SYNC_DB_TEST") != "1" {
		t.Skip("database integration test is opt-in")
	}
	env, err := godotenv.Read(".env")
	if err != nil {
		t.Fatal(err)
	}
	u := url.URL{Scheme: "postgres", User: url.UserPassword(env["DB_USER"], env["DB_PASSWORD"]), Host: net.JoinHostPort(env["DB_HOST"], env["DB_PORT"]), Path: "/" + env["DB_NAME"]}
	q := u.Query()
	q.Set("sslmode", env["DB_SSLMODE"])
	u.RawQuery = q.Encode()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatal("could not configure test connection")
	}
	defer pool.Close()
	t.Setenv("SHEETS_SYNC_TOKEN", "")
	for _, kind := range []string{"camp", "tres"} {
		original := syncSchemas[kind]
		table := fmt.Sprintf("sync_test_%s_%d", kind, time.Now().UnixNano())
		_, err = pool.Exec(ctx, "CREATE TABLE public."+table+" (LIKE public."+original.Table+" INCLUDING ALL)")
		if err != nil {
			t.Fatal(err)
		}
		func() {
			defer pool.Exec(context.Background(), "DROP TABLE public."+table)
			schema := original
			schema.Table = table
			syncSchemas[kind] = schema
			defer func() { syncSchemas[kind] = original }()
			var id string
			if kind == "camp" {
				err = pool.QueryRow(ctx, "INSERT INTO public."+table+" (role,name,phone,telegram,birth_date,church,allergies,allergy_consent,service_areas) VALUES ('servant','SYNC TEST','01011112222','@test_sync','2000-01-09','TEST','[]',false,ARRAY['Кухня']) RETURNING id::text").Scan(&id)
			} else {
				err = pool.QueryRow(ctx, "INSERT INTO public."+table+" (form_type,name,birth_date,city,phone,telegram,church,clothing_size,has_allergies,personal_data_consent) VALUES ('participant','SYNC TEST','2000-01-09','TEST','01011112222','@test_sync','TEST','M',false,true) RETURNING id::text").Scan(&id)
			}
			if err != nil {
				t.Error(err)
				return
			}
			type result struct {
				conflicts int
				err       error
			}
			done := make(chan result, 2)
			for _, name := range []string{"ADMIN ONE", "ADMIN TWO"} {
				go func(name string) {
					_, c, e := applySyncEdit(ctx, pool, kind, syncEdit{ID: id, Base: map[string]any{"name": "SYNC TEST"}, Changes: map[string]any{"name": name}}, "test")
					done <- result{len(c), e}
				}(name)
			}
			conflicts := 0
			for i := 0; i < 2; i++ {
				r := <-done
				if r.err != nil {
					t.Error(r.err)
				}
				conflicts += r.conflicts
			}
			if conflicts != 1 {
				t.Errorf("%s: want one conflict, got %d", kind, conflicts)
			}
			_, c, e := applySyncEdit(ctx, pool, kind, syncEdit{ID: id, Base: map[string]any{"phone": "01011112222"}, Changes: map[string]any{"phone": "01033334444"}}, "test")
			if e != nil || len(c) > 0 {
				t.Errorf("%s: independent edit failed: %v %v", kind, e, c)
			}
			var phone string
			if err := pool.QueryRow(ctx, "SELECT phone FROM public."+table+" WHERE id::text=$1", id).Scan(&phone); err != nil || phone != "01033334444" {
				t.Errorf("%s: update not persisted", kind)
			}
		}()
	}
}
