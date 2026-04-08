package service

import (
	"encoding/json"
	"net/http"
)

type UploadService struct {
	qiniu *QiniuService
}

func NewUploadService(qiniu *QiniuService) *UploadService {
	return &UploadService{qiniu: qiniu}
}

func (s *UploadService) GetUploadToken(w http.ResponseWriter, r *http.Request) {
	token := s.qiniu.GetUploadToken()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"token": token})
}
