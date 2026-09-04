package models

import "testing"

func TestMenuTypeConstants(t *testing.T) {
	cases := map[string]string{
		Directory: "M",
		Menu:      "C",
		Button:    "F",
	}
	for got, want := range cases {
		if got != want {
			t.Errorf("menu constant = %q, want %q", got, want)
		}
	}
}

func TestMigrationTableName(t *testing.T) {
	if got := (Migration{}).TableName(); got != "sys_migration" {
		t.Fatalf("Migration.TableName() = %q, want sys_migration", got)
	}
}

func TestResponseReturnOKAndReturnError(t *testing.T) {
	res := (&Response{}).ReturnOK()
	if res.Code != 200 {
		t.Fatalf("ReturnOK() Code = %d, want 200", res.Code)
	}
	res = res.ReturnError(500)
	if res.Code != 500 {
		t.Fatalf("ReturnError(500) Code = %d, want 500", res.Code)
	}
}

func TestControlBySetters(t *testing.T) {
	var c ControlBy
	c.SetCreateBy(1)
	c.SetUpdateBy(2)
	if c.CreateBy != 1 || c.UpdateBy != 2 {
		t.Fatalf("ControlBy = %+v, want CreateBy=1 UpdateBy=2", c)
	}
}

func TestBaseUserSetPasswordRoundTrip(t *testing.T) {
	var u BaseUser
	u.Username = "alice"
	u.SetPassword("s3cr3t")

	if u.Salt == "" {
		t.Fatal("SetPassword did not generate a salt")
	}
	if u.PasswordHash == "" {
		t.Fatal("SetPassword did not derive a hash")
	}
	// GetPasswordHash must be deterministic for the same Password+Salt, since
	// Verify re-derives it from a freshly loaded row and compares.
	if got := u.GetPasswordHash(); got != u.PasswordHash {
		t.Fatalf("GetPasswordHash() = %q, want %q (same Password+Salt)", got, u.PasswordHash)
	}

	other := BaseUser{Username: "alice", Password: "wrong", Salt: u.Salt}
	if other.GetPasswordHash() == u.PasswordHash {
		t.Fatal("a different password produced the same hash under the same salt")
	}
}
