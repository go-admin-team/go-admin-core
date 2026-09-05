package models

import (
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

type verifyUser struct {
	ID uint `gorm:"primarykey"`
	BaseUser
}

func (verifyUser) TableName() string { return "verify_user" }

func newVerifyDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&verifyUser{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	u := &verifyUser{}
	u.Username = "alice"
	u.SetPassword("correct-horse")
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("seed: %v", err)
	}
	return db
}

func TestVerifyAcceptsTheStoredPassword(t *testing.T) {
	db := newVerifyDB(t)
	u := &verifyUser{}
	u.Username = "alice"
	u.Password = "correct-horse"
	if !u.Verify(db, "verify_user") {
		t.Fatal("the stored password did not verify")
	}
}

func TestVerifyRejectsAWrongPassword(t *testing.T) {
	db := newVerifyDB(t)
	u := &verifyUser{}
	u.Username = "alice"
	u.Password = "wrong"
	if u.Verify(db, "verify_user") {
		t.Fatal("a wrong password verified")
	}
}

// A receiver reused after a successful login still holds that login's Salt
// and PasswordHash. First writes nothing when it finds no row, so ignoring
// its error let the comparison run against those stale fields and report a
// username that does not exist as verified.
func TestVerifyRejectsAMissingUserOnAReusedReceiver(t *testing.T) {
	db := newVerifyDB(t)
	u := &verifyUser{}
	u.Username = "alice"
	u.Password = "correct-horse"
	if !u.Verify(db, "verify_user") {
		t.Fatal("precondition: the first verify must succeed to load the salt")
	}

	u.Username = "mallory-does-not-exist"
	if u.Verify(db, "verify_user") {
		t.Fatal("a username with no row verified true off the previous login's salt")
	}
}
