// Package hash 提供密码哈希与摘要工具：bcrypt、shake256（对齐原版的 SHAKE256_LENGTH）。
package hash

import (
	"crypto/sha3"
	"encoding/hex"
	"golang.org/x/crypto/bcrypt"
)

// SHAKE256_LENGTH 是原版 shake256 摘要长度（字节数）。
const SHAKE256_LENGTH = 64

// BcryptHash 将明文密码 bcrypt 哈希，返回 $2a$ 前缀的哈希字符串。
func BcryptHash(password string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

// BcryptCheck 校验明文密码与 bcrypt 哈希是否匹配。
func BcryptCheck(password, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

// Shake256 返回 shake256(password) 的十六进制摘要字符串（与原版 password-hash.ts 一致）。
func Shake256(password string) string {
	h := sha3.NewSHAKE256()
	h.Write([]byte(password))
	b := make([]byte, SHAKE256_LENGTH)
	h.Read(b)
	return hex.EncodeToString(b)
}

// NeedRehash 判断 bcrypt 哈希是否需要重新哈希（成本因子不足或格式异常）。
func NeedRehash(hash string) bool {
	if len(hash) < 20 {
		return true
	}
	// bcrypt 成本因子在 $2a$XX$ 中，XX < 10 时需要重哈希
	cost := hash[4:6]
	if cost < "10" {
		return true
	}
	return false
}
