package seed

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/go-admin-team/go-admin-core/v2/sdk/contract/models"
)

// resetSeeder clears the process-wide registration for the duration of one
// test. Production code has no legitimate reason to unregister a seeder -
// this exists so tests do not leak state into one another.
func resetSeeder(t *testing.T) {
	t.Helper()
	seederMu.Lock()
	previous := seeder
	seeder = nil
	seederMu.Unlock()
	t.Cleanup(func() {
		seederMu.Lock()
		seeder = previous
		seederMu.Unlock()
	})
}

// fakeSeeder records every call it receives, so tests can assert on what
// SeedMenus forwarded rather than on any real table.
type fakeSeeder struct {
	calls []fakeCall
	err   error
}

type fakeCall struct {
	appCode string
	menus   []MenuSpec
	apis    []ApiSpec
}

func (f *fakeSeeder) SeedMenus(tx *gorm.DB, appCode string, menus []MenuSpec, apis []ApiSpec) error {
	f.calls = append(f.calls, fakeCall{appCode: appCode, menus: menus, apis: apis})
	return f.err
}

func testDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	return db
}

// SeedMenus must fail loudly, not silently do nothing, when no host has
// registered a Seeder - an application built against a host with no seeding
// concept should see a normal error from its own migration.
func TestSeedMenusWithoutRegisteredSeederReturnsError(t *testing.T) {
	resetSeeder(t)

	err := SeedMenus(testDB(t), "order", []MenuSpec{{Code: "root"}}, nil)
	if !errors.Is(err, ErrNoSeeder) {
		t.Fatalf("got error %v, want ErrNoSeeder", err)
	}
}

// The mandated shape: appCode, menus and apis all reach the host's Seeder
// exactly as the application supplied them.
func TestSeedMenusForwardsToRegisteredSeeder(t *testing.T) {
	resetSeeder(t)

	fake := &fakeSeeder{}
	RegisterSeeder(fake)

	menus := []MenuSpec{
		{Code: "root", Kind: models.Directory, Title: "Order"},
		{Code: "list", Parent: "root", Kind: models.Menu, Title: "Order list", Path: "/order", Component: "apps/order/index", ApiCodes: []string{"list"}},
		{Code: "add", Parent: "list", Kind: models.Button, Permission: "order:add"},
	}
	apis := []ApiSpec{
		{Code: "list", Title: "list orders", Path: "/api/v1/order", Method: "GET"},
	}

	if err := SeedMenus(testDB(t), "order", menus, apis); err != nil {
		t.Fatalf("SeedMenus: %v", err)
	}

	if len(fake.calls) != 1 {
		t.Fatalf("Seeder.SeedMenus called %d times, want 1", len(fake.calls))
	}
	call := fake.calls[0]
	if call.appCode != "order" {
		t.Errorf("appCode = %q, want %q", call.appCode, "order")
	}
	if len(call.menus) != 3 || call.menus[1].ApiCodes[0] != "list" {
		t.Errorf("menus were not forwarded unchanged: %+v", call.menus)
	}
	if len(call.apis) != 1 || call.apis[0].Path != "/api/v1/order" {
		t.Errorf("apis were not forwarded unchanged: %+v", call.apis)
	}
}

// An error from the host's own Seeder implementation must reach the caller,
// named against the application that triggered it.
func TestSeedMenusWrapsSeederError(t *testing.T) {
	resetSeeder(t)

	fake := &fakeSeeder{err: fmt.Errorf("duplicate menu_id 8800")}
	RegisterSeeder(fake)

	err := SeedMenus(testDB(t), "order", []MenuSpec{{Code: "root"}}, nil)
	if err == nil {
		t.Fatal("SeedMenus must return the Seeder's error, got nil")
	}
	if !strings.Contains(err.Error(), "order") || !strings.Contains(err.Error(), "duplicate menu_id 8800") {
		t.Errorf("error %q must name both the app and the underlying cause", err.Error())
	}
}

func TestSeedMenusRejectsEmptyAppCode(t *testing.T) {
	resetSeeder(t)
	RegisterSeeder(&fakeSeeder{})

	if err := SeedMenus(testDB(t), "", []MenuSpec{{Code: "root"}}, nil); err == nil {
		t.Error("SeedMenus must reject an empty appCode")
	}
}

// The counterproof for this package's registry: a second host trying to
// install its own Seeder must fail clearly, not silently discard the first
// implementation or the second one.
func TestRegisterSeederPanicsOnSecondCall(t *testing.T) {
	resetSeeder(t)
	RegisterSeeder(&fakeSeeder{})

	defer func() {
		if recover() == nil {
			t.Fatal("RegisterSeeder must panic when called a second time")
		}
	}()
	RegisterSeeder(&fakeSeeder{})
}

func TestRegisterSeederPanicsOnNil(t *testing.T) {
	resetSeeder(t)

	defer func() {
		if recover() == nil {
			t.Fatal("RegisterSeeder must panic on a nil Seeder")
		}
	}()
	RegisterSeeder(nil)
}
