package service

import (
	"context"
	"encoding/json"
	"net/http"
	"photo-backend/internal/biz"
	"photo-backend/internal/data"

	"github.com/gorilla/mux"
)

type TaskService struct {
	uc *biz.TaskUsecase
}

func sanitizeTaskForViewer(task *data.Task, viewerID string) *data.Task {
	if task == nil {
		return nil
	}

	safeTask := *task
	if safeTask.UserID != viewerID {
		safeTask.VerificationCode = ""
	}

	return &safeTask
}

func NewTaskService(uc *biz.TaskUsecase) *TaskService {
	return &TaskService{uc: uc}
}

func (s *TaskService) CreateTask(w http.ResponseWriter, r *http.Request) {
	var task data.Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		Error(w, 1002, err.Error())
		return
	}

	// 从 context 中获取用户 ID（由 JWT 中间件注入）
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		Error(w, 1002, "unauthorized")
		return
	}
	task.UserID = userID

	if err := s.uc.CreateTask(context.Background(), &task); err != nil {
		Error(w, 1003, err.Error())
		return
	}

	Success(w, map[string]interface{}{"id": task.ID.Hex()})
}

func (s *TaskService) GetTask(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		Error(w, 1004, "unauthorized")
		return
	}

	task, err := s.uc.GetTask(context.Background(), id)
	if err != nil {
		Error(w, 1004, err.Error())
		return
	}

	Success(w, sanitizeTaskForViewer(task, userID))
}

func (s *TaskService) GetTaskByCode(w http.ResponseWriter, r *http.Request) {
	taskCode := mux.Vars(r)["taskCode"]
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		Error(w, 1010, "unauthorized")
		return
	}

	task, err := s.uc.GetTaskByCode(context.Background(), taskCode)
	if err != nil {
		Error(w, 1010, err.Error())
		return
	}

	Success(w, sanitizeTaskForViewer(task, userID))
}

func (s *TaskService) UpdateTask(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	var task data.Task
	if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
		Error(w, 1008, err.Error())
		return
	}

	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		Error(w, 1008, "unauthorized")
		return
	}

	if err := s.uc.UpdateTask(context.Background(), id, userID, &task); err != nil {
		Error(w, 1009, err.Error())
		return
	}

	Success(w, map[string]interface{}{"id": id})
}

func (s *TaskService) DeleteTask(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		Error(w, 1006, "unauthorized")
		return
	}

	if err := s.uc.DeleteTask(context.Background(), id, userID); err != nil {
		Error(w, 1007, err.Error())
		return
	}

	Success(w, nil)
}

func (s *TaskService) ListTasks(w http.ResponseWriter, r *http.Request) {
	// 从 context 中获取用户 ID（由 JWT 中间件注入）
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		Error(w, 1005, "unauthorized")
		return
	}

	tasks, err := s.uc.ListTasks(context.Background(), userID)
	if err != nil {
		Error(w, 1005, err.Error())
		return
	}

	result := make([]*data.Task, 0, len(tasks))
	for _, task := range tasks {
		result = append(result, sanitizeTaskForViewer(task, userID))
	}

	Success(w, result)
}
