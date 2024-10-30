package agent

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
)

func mockHandler(w http.ResponseWriter, r *http.Request) {
	validURL := regexp.MustCompile(
		`^(http|https)://[a-zA-Z0-9\-\.]+\.[a-zA-Z]{2,}(/.*)?$`)

	if !validURL.MatchString(r.URL.String()) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}

func TestReport(t *testing.T) {
	mockServ := httptest.NewServer(http.HandlerFunc(mockHandler))
	defer mockServ.Close()
}

func TestCollect(t *testing.T) {

}
