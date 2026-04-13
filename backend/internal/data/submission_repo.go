package data

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo/options"
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

func (r *SubmissionRepo) FindByTaskID(ctx context.Context, taskID string, page, limit int) ([]*Submission, error) {
	objID, _ := primitive.ObjectIDFromHex(taskID)
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetSkip(int64((page - 1) * limit)).
		SetLimit(int64(limit))
	cursor, err := r.data.DB().Collection("submissions").Find(ctx, bson.M{"task_id": objID}, opts)
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

func (r *SubmissionRepo) FindAllByTaskID(ctx context.Context, taskID string) ([]*Submission, error) {
	objID, _ := primitive.ObjectIDFromHex(taskID)
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: 1}})
	cursor, err := r.data.DB().Collection("submissions").Find(ctx, bson.M{"task_id": objID}, opts)
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

func (r *SubmissionRepo) FindByTaskIDAndUserID(ctx context.Context, taskID string, userID string, page, limit int) ([]*Submission, error) {
	objID, _ := primitive.ObjectIDFromHex(taskID)
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetSkip(int64((page - 1) * limit)).
		SetLimit(int64(limit))
	cursor, err := r.data.DB().Collection("submissions").Find(ctx, bson.M{
		"task_id": objID,
		"user_id": userID,
	}, opts)
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

func (r *SubmissionRepo) FindOneByTaskIDAndUserID(ctx context.Context, taskID string, userID string) (*Submission, error) {
	objID, _ := primitive.ObjectIDFromHex(taskID)
	var sub Submission
	err := r.data.DB().Collection("submissions").FindOne(ctx, bson.M{
		"task_id": objID,
		"user_id": userID,
	}).Decode(&sub)
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (r *SubmissionRepo) Update(ctx context.Context, id string, sub *Submission) error {
	objID, _ := primitive.ObjectIDFromHex(id)
	sub.UpdatedAt = time.Now()
	_, err := r.data.DB().Collection("submissions").UpdateOne(
		ctx,
		bson.M{"_id": objID},
		bson.M{"$set": sub},
	)
	return err
}

func (r *SubmissionRepo) FindByID(ctx context.Context, id string) (*Submission, error) {
	objID, _ := primitive.ObjectIDFromHex(id)
	var sub Submission
	err := r.data.DB().Collection("submissions").FindOne(ctx, bson.M{"_id": objID}).Decode(&sub)
	if err != nil {
		return nil, err
	}
	return &sub, nil
}

func (r *SubmissionRepo) Delete(ctx context.Context, id string) error {
	objID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	_, err = r.data.DB().Collection("submissions").DeleteOne(ctx, bson.M{"_id": objID})
	return err
}

func (r *SubmissionRepo) CountByTaskID(ctx context.Context, taskID string) (int64, error) {
	objID, _ := primitive.ObjectIDFromHex(taskID)
	count, err := r.data.DB().Collection("submissions").CountDocuments(ctx, bson.M{"task_id": objID})
	return count, err
}

func (r *SubmissionRepo) CountByTaskIDAndUserID(ctx context.Context, taskID string, userID string) (int64, error) {
	objID, _ := primitive.ObjectIDFromHex(taskID)
	count, err := r.data.DB().Collection("submissions").CountDocuments(ctx, bson.M{
		"task_id": objID,
		"user_id": userID,
	})
	return count, err
}

// FindDistinctTaskIDsByUserID 查询用户参与过（有提交记录）的所有任务ID（去重）
func (r *SubmissionRepo) FindDistinctTaskIDsByUserID(ctx context.Context, userID string) ([]primitive.ObjectID, error) {
	results, err := r.data.DB().Collection("submissions").Distinct(ctx, "task_id", bson.M{"user_id": userID})
	if err != nil {
		return nil, err
	}
	var ids []primitive.ObjectID
	for _, v := range results {
		if oid, ok := v.(primitive.ObjectID); ok {
			ids = append(ids, oid)
		}
	}
	return ids, nil
}

func (r *SubmissionRepo) DeleteByTaskID(ctx context.Context, taskID string) error {
	objID, err := primitive.ObjectIDFromHex(taskID)
	if err != nil {
		return err
	}

	_, err = r.data.DB().Collection("submissions").DeleteMany(ctx, bson.M{"task_id": objID})
	return err
}
