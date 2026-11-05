package schematest

import (
	authdomain "github.com/garda-inflasi/api/internal/auth/domain"
)

func init() {
	registerModels("auth", &authdomain.Pengguna{}, &authdomain.RefreshToken{}, &authdomain.LoginAttempt{}, &authdomain.ResetToken{}, &authdomain.KodeCadanganMFA{}, &authdomain.SSOLoginState{})
}
