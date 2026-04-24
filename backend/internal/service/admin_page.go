package service

import (
	_ "embed"
	"net/http"
)

//go:embed admin_page.html
var adminPageHTML []byte

func (s *AdminService) Page(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(adminPageHTML)
}
