package service

import (
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
	Success(w, map[string]string{"token": token})
}
