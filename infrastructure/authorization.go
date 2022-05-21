package infrastructure

import (
	"strings"
)

const AuthToken = "c2VtcmFib25hb3ZhbGVuYWRh="

func IsAuthorized(authorizationHeader string) bool {
	token := strings.TrimSpace(strings.Replace(authorizationHeader, "Bearer", "", 1))
	return token == AuthToken
}
