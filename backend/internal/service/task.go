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
	task, err := s.uc.GetTask(context.Background(), id)
	if err != nil {
		Error(w, 1004, err.Error())
		return
	}

	Success(w, task)
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

	Success(w, tasks)
}
