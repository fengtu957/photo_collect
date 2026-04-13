package main

import (
	"log"
	"net/http"
	"os"
	"photo-backend/internal/biz"
	"photo-backend/internal/data"
	"photo-backend/internal/service"
	"photo-backend/pkg"

	"github.com/gorilla/mux"
)

func main() {
	mongoURI := os.Getenv("MONGODB_URI")
	d, err := data.NewData(mongoURI)
	if err != nil {
		log.Fatal(err)
	}

	taskRepo := data.NewTaskRepo(d)
	subRepo := data.NewSubmissionRepo(d)
	taskUC := biz.NewTaskUsecase(taskRepo, subRepo)
	taskSvc := service.NewTaskService(taskUC)

	qiniuSvc := service.NewQiniuService()
	qwenClient := pkg.NewQwenClient()
	evalUC := biz.NewEvaluationUsecase(qwenClient)
	exportSvc := service.NewExportService(taskRepo, subRepo, qiniuSvc)

	subUC := biz.NewSubmissionUsecase(subRepo, taskRepo)
	subSvc := service.NewSubmissionService(subUC, taskUC, evalUC, qiniuSvc)

	uploadSvc := service.NewUploadService(qiniuSvc)

	authSvc := service.NewAuthService()

	r := mux.NewRouter()

	// 公开接口
	r.HandleFunc("/api/v1/auth/login", authSvc.Login).Methods("POST")

	// 需要认证的接口
	api := r.PathPrefix("/api/v1").Subrouter()
	api.Use(service.AuthMiddleware)
	api.HandleFunc("/tasks", taskSvc.CreateTask).Methods("POST")
	api.HandleFunc("/tasks", taskSvc.ListTasks).Methods("GET")
	api.HandleFunc("/tasks/{id}", taskSvc.GetTask).Methods("GET")
	api.HandleFunc("/tasks/{id}", taskSvc.UpdateTask).Methods("PUT")
	api.HandleFunc("/tasks/{id}/export", exportSvc.ExportTask).Methods("POST")
	api.HandleFunc("/tasks/{id}/export/status", exportSvc.SyncExportStatus).Methods("POST")
	api.HandleFunc("/tasks/{id}/export/authorize", exportSvc.AuthorizeExportLink).Methods("POST")
	api.HandleFunc("/tasks/{id}", taskSvc.DeleteTask).Methods("DELETE")
	api.HandleFunc("/submissions", subSvc.CreateSubmission).Methods("POST")
	api.HandleFunc("/submissions/analyze-preview", subSvc.AnalyzePreview).Methods("POST")
	api.HandleFunc("/submissions/{id}", subSvc.GetSubmission).Methods("GET")
	api.HandleFunc("/submissions/{id}", subSvc.UpdateSubmission).Methods("PUT")
	api.HandleFunc("/submissions/{id}", subSvc.DeleteSubmission).Methods("DELETE")
	api.HandleFunc("/tasks/{taskId}/submissions", subSvc.ListSubmissions).Methods("GET")
	api.HandleFunc("/upload/token", uploadSvc.GetUploadToken).Methods("GET")

	log.Println("Server starting on :8000")
	log.Fatal(http.ListenAndServe(":8000", r))
}
