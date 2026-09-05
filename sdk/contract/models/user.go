package models

import (
	"gorm.io/gorm"

	"github.com/go-admin-team/go-admin-core/v2/sdk/pkg"
)

// BaseUser is the embeddable base for a password-login user model.
type BaseUser struct {
	Username     string `json:"username" gorm:"type:varchar(100);comment:username"`
	Salt         string `json:"-" gorm:"type:varchar(255);comment:salt;<-"`
	PasswordHash string `json:"-" gorm:"type:varchar(128);comment:password hash;<-"`
	Password     string `json:"password" gorm:"-"`
}

// SetPassword sets the plaintext password, generating a fresh salt and
// deriving the stored hash from it.
func (u *BaseUser) SetPassword(value string) {
	u.Password = value
	u.generateSalt()
	u.PasswordHash = u.GetPasswordHash()
}

// GetPasswordHash derives the hash for the current Password and Salt.
func (u *BaseUser) GetPasswordHash() string {
	passwordHash, err := pkg.SetPassword(u.Password, u.Salt)
	if err != nil {
		return ""
	}
	return passwordHash
}

// generateSalt assigns a fresh random salt.
func (u *BaseUser) generateSalt() {
	u.Salt = pkg.GenerateRandomKey16()
}

// Verify loads the row matching u.Username from tableName and reports
// whether u.Password (already hashed into PasswordHash via SetPassword)
// matches what is stored.
//
// A failed lookup must return false, never fall through to the comparison.
// First leaves the receiver untouched when it finds no row, so a caller that
// reuses a receiver already loaded from an earlier successful login would
// otherwise compare that stale Salt and PasswordHash against themselves and
// verify true for a username that does not exist.
func (u *BaseUser) Verify(db *gorm.DB, tableName string) bool {
	if err := db.Table(tableName).Where("username = ?", u.Username).First(u).Error; err != nil {
		return false
	}
	return u.GetPasswordHash() == u.PasswordHash
}
