package data

import (
	"context"
	"regexp"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type TaskRepo struct {
	data *Data
}

type AdminTaskListQuery struct {
	Keyword  string
	Page     int
	PageSize int
}

type AdminTaskListResult struct {
	Items    []*Task `json:"items"`
	Total    int64   `json:"total"`
	Page     int     `json:"page"`
	PageSize int     `json:"page_size"`
}

func NewTaskRepo(data *Data) *TaskRepo {
	return &TaskRepo{data: data}
}

func (r *TaskRepo) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys: bson.D{{Key: "task_code", Value: 1}},
			Options: options.Index().
				SetUnique(true).
				SetSparse(true),
		},
	}

	_, err := r.data.DB().Collection("tasks").Indexes().CreateMany(ctx, indexes)
	return err
}

func (r *TaskRepo) Create(ctx context.Context, task *Task) error {
	task.ID = primitive.NewObjectID()
	task.CreatedAt = time.Now()
	task.UpdatedAt = time.Now()
	_, err := r.data.DB().Collection("tasks").InsertOne(ctx, task)
	return err
}

func (r *TaskRepo) FindByID(ctx context.Context, id string) (*Task, error) {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	var task Task
	err = r.data.DB().Collection("tasks").FindOne(ctx, bson.M{"_id": objID}).Decode(&task)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &task, err
}

func (r *TaskRepo) FindByTaskCode(ctx context.Context, taskCode string) (*Task, error) {
	var task Task
	err := r.data.DB().Collection("tasks").FindOne(ctx, bson.M{"task_code": strings.TrimSpace(taskCode)}).Decode(&task)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	return &task, err
}

func (r *TaskRepo) FindByUserID(ctx context.Context, userID string) ([]*Task, error) {
	// 按创建时间倒序排列
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := r.data.DB().Collection("tasks").Find(ctx, bson.M{"user_id": userID}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var tasks []*Task
	if err := cursor.All(ctx, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

func (r *TaskRepo) CountActiveByUserID(ctx context.Context, userID string) (int64, error) {
	count, err := r.data.DB().Collection("tasks").CountDocuments(ctx, bson.M{
		"user_id": userID,
		"enabled": true,
		"end_time": bson.M{
			"$gt": time.Now(),
		},
	})
	return count, err
}

func (r *TaskRepo) AdminListTasks(ctx context.Context, query AdminTaskListQuery) (*AdminTaskListResult, error) {
	page := query.Page
	if page <= 0 {
		page = 1
	}

	pageSize := query.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}

	filter := bson.M{}
	keyword := strings.TrimSpace(query.Keyword)
	if keyword != "" {
		if len(keyword) == 5 {
			isDigits := true
			for i := 0; i < len(keyword); i++ {
				if keyword[i] < '0' || keyword[i] > '9' {
					isDigits = false
					break
				}
			}
			if isDigits {
				filter["task_code"] = keyword
			}
		}

		if len(filter) == 0 {
			pattern := primitive.Regex{Pattern: regexp.QuoteMeta(keyword), Options: "i"}
			orConditions := []bson.M{
				{"title": bson.M{"$regex": pattern}},
				{"description": bson.M{"$regex": pattern}},
				{"user_id": bson.M{"$regex": pattern}},
				{"task_code": bson.M{"$regex": pattern}},
			}

			if objectID, err := primitive.ObjectIDFromHex(keyword); err == nil {
				orConditions = append(orConditions, bson.M{"_id": objectID})
			}

			filter["$or"] = orConditions
		}
	}

	total, err := r.data.DB().Collection("tasks").CountDocuments(ctx, filter)
	if err != nil {
		return nil, err
	}

	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetSkip(int64((page - 1) * pageSize)).
		SetLimit(int64(pageSize))

	cursor, err := r.data.DB().Collection("tasks").Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var tasks []*Task
	if err := cursor.All(ctx, &tasks); err != nil {
		return nil, err
	}

	return &AdminTaskListResult{
		Items:    tasks,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

// FindByIDs 按多个ID批量查询任务，按创建时间倒序
func (r *TaskRepo) FindByIDs(ctx context.Context, ids []primitive.ObjectID) ([]*Task, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	opts := options.Find().SetSort(bson.D{{Key: "created_at", Value: -1}})
	cursor, err := r.data.DB().Collection("tasks").Find(ctx, bson.M{"_id": bson.M{"$in": ids}}, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var tasks []*Task
	if err := cursor.All(ctx, &tasks); err != nil {
		return nil, err
	}
	return tasks, nil
}

func (r *TaskRepo) Delete(ctx context.Context, id string) error {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}
	_, err = r.data.DB().Collection("tasks").DeleteOne(ctx, bson.M{"_id": objID})
	return err
}

func (r *TaskRepo) Update(ctx context.Context, id string, task *Task) error {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	task.UpdatedAt = time.Now()
	_, err = r.data.DB().Collection("tasks").UpdateOne(ctx, bson.M{"_id": objID}, bson.M{
		"$set": bson.M{
			"title":                     task.Title,
			"description":               task.Description,
			"photo_spec":                task.PhotoSpec,
			"ai_analysis_enabled":       task.AIAnalysisEnabled,
			"task_code":                 task.TaskCode,
			"verification_code_enabled": task.VerificationCodeEnabled,
			"verification_code":         task.VerificationCode,
			"max_submissions":           task.MaxSubmissions,
			"start_time":                task.StartTime,
			"end_time":                  task.EndTime,
			"custom_fields":             task.CustomFields,
			"updated_at":                task.UpdatedAt,
		},
	})
	return err
}

func (r *TaskRepo) UpdateExportInfo(ctx context.Context, id string, exportInfo TaskExportInfo) error {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	updatedAt := time.Now()
	_, err = r.data.DB().Collection("tasks").UpdateOne(ctx, bson.M{"_id": objID}, bson.M{
		"$set": bson.M{
			"export_info": exportInfo,
			"updated_at":  updatedAt,
		},
	})
	return err
}
