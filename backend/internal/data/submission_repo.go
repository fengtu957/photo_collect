package data

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SubmissionRepo struct {
	data *Data
}

func NewSubmissionRepo(data *Data) *SubmissionRepo {
	return &SubmissionRepo{data: data}
}

func (r *SubmissionRepo) Create(ctx context.Context, sub *Submission) error {
	sub.ID = primitive.NewObjectID()
	sub.CreatedAt = time.Now()
	sub.UpdatedAt = time.Now()
	_, err := r.data.DB().Collection("submissions").InsertOne(ctx, sub)
	return err
}

func (r *SubmissionRepo) FindByTaskID(ctx context.Context, taskID string) ([]*Submission, error) {
	objID, _ := primitive.ObjectIDFromHex(taskID)
	cursor, err := r.data.DB().Collection("submissions").Find(ctx, bson.M{"task_id": objID})
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var subs []*Submission
	if err := cursor.All(ctx, &subs); err != nil {
		return nil, err
	}
	return subs, nil
}
