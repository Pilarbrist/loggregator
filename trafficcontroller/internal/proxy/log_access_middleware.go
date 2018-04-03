package proxy

import (
	"net/http"
	"regexp"

	"code.cloudfoundry.org/loggregator/trafficcontroller/internal/auth"

	"github.com/gorilla/mux"
)

type LogAccessMiddleware struct {
	authorize auth.LogAccessAuthorizer
}

func NewLogAccessMiddleware(authorize auth.LogAccessAuthorizer) *LogAccessMiddleware {
	return &LogAccessMiddleware{
		authorize: authorize,
	}
}

func (m *LogAccessMiddleware) Wrap(h http.Handler) http.Handler {
	guid := regexp.MustCompile("^[[:xdigit:]]{8}-[[:xdigit:]]{4}-[[:xdigit:]]{4}-[[:xdigit:]]{4}-[[:xdigit:]]{12}$")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appID := mux.Vars(r)["appID"]
		if !guid.MatchString(appID) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		authToken := getAuthToken(r)

		status, _ := m.authorize(authToken, appID)
		if status != http.StatusOK {
			switch status {
			case http.StatusUnauthorized:
				w.Header().Set("WWW-Authenticate", "Basic")
			case http.StatusForbidden, http.StatusNotFound:
				status = http.StatusNotFound
			default:
				status = http.StatusInternalServerError
			}

			w.WriteHeader(status)

			return
		}

		h.ServeHTTP(w, r)
	})
}
