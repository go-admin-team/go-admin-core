package models

import (
	"gorm.io/gorm"

	"github.com/go-admin-team/go-admin-core/v2/sdk/pkg"
)

// BaseUser is the embeddable base for a password-login user model.
type BaseUser struct {
	Username     string `json:"username" gorm:"type:varchar(100);comment:用户名"`
	Salt         string `json:"-" gorm:"type:varchar(255);comment:加盐;<-"`
	PasswordHash string `json:"-" gorm:"type:varchar(128);comment:密码hash;<-"`
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
func (u *BaseUser) Verify(db *gorm.DB, tableName string) bool {
	db.Table(tableName).Where("username = ?", u.Username).First(u)
	return u.GetPasswordHash() == u.PasswordHash
}
