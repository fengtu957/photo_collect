package data

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type TaskRepo struct {
	data *Data
}

func NewTaskRepo(data *Data) *TaskRepo {
	return &TaskRepo{data: data}
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
			"title":               task.Title,
			"description":         task.Description,
			"photo_spec":          task.PhotoSpec,
			"ai_analysis_enabled": task.AIAnalysisEnabled,
			"start_time":          task.StartTime,
			"end_time":            task.EndTime,
			"custom_fields":       task.CustomFields,
			"updated_at":          task.UpdatedAt,
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
