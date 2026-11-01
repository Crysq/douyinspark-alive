package util

import "golang.org/x/crypto/bcrypt"

// HashPassword 生成 bcrypt 哈希。
func HashPassword(raw string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.DefaultCost)
	return string(hash), err
}

// CheckPassword 校验密码。
func CheckPassword(hash, raw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(raw)) == nil
}
