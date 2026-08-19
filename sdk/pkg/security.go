package pkg

import (
	"crypto/rand"
	"encoding/hex"

	"golang.org/x/crypto/scrypt"
)

const (
	symbol = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*()-_=+,.?/:;{}[]`~"
	letter = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

// generateRandString draws from crypto/rand.
//
// What each caller needs differs. A password salt needs to be unique, not
// secret, and math/rand gave unique values. A token or a verification code —
// which is what the exported names here read like they are for — needs to be
// unpredictable, and math/rand is a generator whose internal state can be
// recovered from enough of its output, after which every later value is known.
// One source that satisfies both is simpler than two that each satisfy one.
func generateRandString(length int, s string) string {
	var chars = []byte(s)
	clen := len(chars)
	if clen < 2 || clen > 256 {
		panic("Wrong charset length for NewLenChars()")
	}
	maxrb := 255 - (256 % clen)
	b := make([]byte, length)
	r := make([]byte, length+(length/4)) // storage for random bytes.
	i := 0
	for {
		if _, err := rand.Read(r); err != nil {
			panic("Error reading random bytes: " + err.Error())
		}
		for _, rb := range r {
			c := int(rb)
			if c > maxrb {
				continue // Skip this number to avoid modulo bias.
			}
			b[i] = chars[c%clen]
			i++
			if i == length {
				return string(b)
			}
		}
	}
}

// GenerateRandomKey20 生成20位随机字符串
func GenerateRandomKey20() string {
	return generateRandString(20, symbol)
}

// GenerateRandomKey16 生成16为随机字符串
func GenerateRandomKey16() string {
	return generateRandString(16, symbol)
}

// GenerateRandomKey6 生成6为随机字符串
func GenerateRandomKey6() string {
	return generateRandString(6, letter)
}

// SetPassword 根据明文密码和加盐值生成密码
func SetPassword(password string, salt string) (verify string, err error) {
	var rb []byte
	rb, err = scrypt.Key([]byte(password), []byte(salt), 16384, 8, 1, 32)
	if err != nil {
		return
	}
	verify = hex.EncodeToString(rb)
	return
}
