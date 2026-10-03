package middleware

import "net/http"

const accessTokenQueryParam = "access_token"

func QueryTokenAuth(validator TokenValidator) func(http.Handler) http.Handler {
	auth := Auth(validator)

	return func(next http.Handler) http.Handler {
		authenticated := auth(next)

		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if _, ok := BearerToken(r); !ok {
				query := r.URL.Query()
				if token := query.Get(accessTokenQueryParam); token != "" {
					r = r.Clone(r.Context())
					r.Header.Set("Authorization", bearerPrefix+token)

					query.Del(accessTokenQueryParam)
					r.URL.RawQuery = query.Encode()
				}
			}

			authenticated.ServeHTTP(w, r)
		})
	}
}
