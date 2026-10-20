package storage

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/panttony/kbase/internal/model"
)

func sampleDoc() model.Document {
	return model.Document{
		Title:    "Postgres Tuning",
		Category: "backend",
		Tags:     []string{"go", "postgres"},
		Body:     "# Postgres Tuning\nSet work_mem for join queries.",
		Source:   "notes.sql",
	}
}

func TestRepoCreateAndGet(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, sampleDoc())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if created.ID == uuid.Nil {
		t.Fatal("Create returned nil id")
	}
	if created.Version != 1 {
		t.Errorf("Version = %d, want 1", created.Version)
	}
	if created.UpdatedAt.IsZero() || created.CreatedAt.IsZero() {
		t.Error("timestamps must be set")
	}

	got, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != created.Title || got.Category != created.Category {
		t.Errorf("Get = %+v, want fields from create", got)
	}
	if !reflect.DeepEqual(got.Tags, []string{"go", "postgres"}) {
		t.Errorf("Tags = %v, want sorted [go postgres]", got.Tags)
	}
	if got.Version != 1 {
		t.Errorf("Version = %d, want 1", got.Version)
	}
}

func TestRepoUpdateCreatesVersion(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, sampleDoc())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	changed := created
	changed.Body = "# Postgres Tuning\nV2 content."
	changed.Tags = []string{"go", "postgres", "performance"}
	updated, err := repo.Update(ctx, changed)
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Version != 2 {
		t.Errorf("Version = %d, want 2", updated.Version)
	}
	if updated.Body != changed.Body {
		t.Errorf("Body = %q, want new body", updated.Body)
	}

	versions, err := repo.ListVersions(ctx, created.ID)
	if err != nil {
		t.Fatalf("ListVersions: %v", err)
	}
	if len(versions) != 2 {
		t.Fatalf("len(versions) = %d, want 2", len(versions))
	}

	if versions[0].Number != 1 || versions[1].Number != 2 {
		t.Errorf("version order numbers = [%d %d], want [1 2]", versions[0].Number, versions[1].Number)
	}
	if versions[0].Body != sampleDoc().Body {
		t.Errorf("v1 body = %q, want original body", versions[0].Body)
	}

	one, err := repo.GetVersion(ctx, created.ID, 1)
	if err != nil {
		t.Fatalf("GetVersion(1): %v", err)
	}
	if one.Body != sampleDoc().Body {
		t.Errorf("GetVersion(1) body = %q, want original", one.Body)
	}
	if !reflect.DeepEqual(one.Meta.Tags, []string{"go", "postgres"}) {
		t.Errorf("v1 meta tags = %v, want the original [go postgres]", one.Meta.Tags)
	}

	latest, err := repo.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if latest.Version != 2 || latest.Body != changed.Body {
		t.Errorf("Get latest = version %d body %q", latest.Version, latest.Body)
	}
	if !reflect.DeepEqual(latest.Tags, []string{"go", "performance", "postgres"}) {
		t.Errorf("latest tags = %v, want [go performance postgres]", latest.Tags)
	}
}

func TestRepoSoftDeleteKeepsHistory(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	created, err := repo.Create(ctx, sampleDoc())
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	if err := repo.Delete(ctx, created.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := repo.Get(ctx, created.ID); err == nil {
		t.Fatal("Get after delete should fail")
	} else {
		var nf ErrNotFound
		if !errors.As(err, &nf) {
			t.Errorf("Get err = %v, want ErrNotFound", err)
		}
	}

	if err := repo.Delete(ctx, created.ID); err == nil {
		t.Error("second delete should fail")
	}

	var cnt int
	if err := repo.Pool().QueryRow(ctx,
		`SELECT count(*) FROM versions WHERE document_id = $1`, created.ID).Scan(&cnt); err != nil {
		t.Fatalf("count versions: %v", err)
	}
	if cnt != 1 {
		t.Errorf("retained versions = %d, want 1 (soft delete must keep history)", cnt)
	}
}

func TestRepoNotFound(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	missing := uuid.New()
	if _, err := repo.Get(ctx, missing); err == nil {
		t.Fatal("Get(missing) should fail")
	} else {
		var nf ErrNotFound
		if !errors.As(err, &nf) {
			t.Errorf("err = %v, want ErrNotFound", err)
		}
	}
	if _, err := repo.GetVersion(ctx, missing, 1); err == nil {
		t.Fatal("GetVersion(missing) should fail")
	}
}

func TestRepoMigrationIdempotent(t *testing.T) {
	ctx := context.Background()
	if _, err := New(ctx, newTestRepo(t).Pool()); err != nil {
		t.Fatalf("re-running migrate should be a no-op: %v", err)
	}
}
