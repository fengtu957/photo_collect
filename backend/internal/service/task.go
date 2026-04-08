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

	userID := r.Header.Get("X-User-ID")
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

func (s *TaskService) ListTasks(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	tasks, err := s.uc.ListTasks(context.Background(), userID)
	if err != nil {
		Error(w, 1005, err.Error())
		return
	}

	Success(w, tasks)
}
