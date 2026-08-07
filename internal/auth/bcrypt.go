package auth

import "golang.org/x/crypto/bcrypt"

// HashPasswordBcrypt is distinct from HashPassword (argon2id, used for
// dashboard login) because Caddy's http_basic auth provider specifically
// requires bcrypt hashes -- used only for per-deployment password
// protection enforced at the proxy layer.
func HashPasswordBcrypt(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(hash), err
}
