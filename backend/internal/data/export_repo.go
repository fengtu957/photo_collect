package data

import (
	"context"
	"errors"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type ExportRepo struct {
	data *Data
}

func NewExportRepo(data *Data) *ExportRepo {
	return &ExportRepo{data: data}
}

func (r *ExportRepo) EnsureIndexes(ctx context.Context) error {
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "export_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys: bson.D{{Key: "task_id", Value: 1}, {Key: "created_at", Value: -1}},
		},
	}
	_, err := r.data.DB().Collection("export_records").Indexes().CreateMany(ctx, indexes)
	return err
}

func (r *ExportRepo) Create(ctx context.Context, record *ExportRecord) error {
	if record == nil {
		return errors.New("导出记录不能为空")
	}
	now := time.Now()
	record.ID = primitive.NewObjectID()
	record.CreatedAt = now
	record.UpdatedAt = now
	_, err := r.data.DB().Collection("export_records").InsertOne(ctx, record)
	return err
}

func (r *ExportRepo) FindByTaskID(ctx context.Context, taskID string, page int, limit int) (*ExportListResult, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 || limit > 100 {
		limit = 20
	}
	filter := bson.M{"task_id": taskID}
	total, err := r.data.DB().Collection("export_records").CountDocuments(ctx, filter)
	if err != nil {
		return nil, err
	}
	opts := options.Find().
		SetSort(bson.D{{Key: "created_at", Value: -1}}).
		SetSkip(int64((page - 1) * limit)).
		SetLimit(int64(limit))
	cursor, err := r.data.DB().Collection("export_records").Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var records []*ExportRecord
	if err := cursor.All(ctx, &records); err != nil {
		return nil, err
	}
	return &ExportListResult{
		List:    records,
		Total:   total,
		Page:    page,
		HasMore: int64(page*limit) < total,
	}, nil
}

func (r *ExportRepo) FindByExportID(ctx context.Context, exportID string) (*ExportRecord, error) {
	var record ExportRecord
	err := r.data.DB().Collection("export_records").FindOne(ctx, bson.M{"export_id": exportID}).Decode(&record)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &record, nil
}

func (r *ExportRepo) UpdateStatus(ctx context.Context, record *ExportRecord) (bool, error) {
	if record == nil {
		return false, errors.New("导出记录不能为空")
	}
	record.UpdatedAt = time.Now()
	filter := bson.M{"export_id": record.ExportID}
	if record.Status == "processing" || record.Status == "pending" {
		filter["status"] = bson.M{"$nin": []string{"success", "failed"}}
	} else if record.Status == "failed" {
		filter["status"] = bson.M{"$ne": "success"}
	}
	result, err := r.data.DB().Collection("export_records").UpdateOne(
		ctx,
		filter,
		bson.M{"$set": bson.M{
			"status":          record.Status,
			"error_message":   record.ErrorMessage,
			"exported_at":     record.ExportedAt,
			"available_until": record.AvailableUntil,
			"updated_at":      record.UpdatedAt,
		}},
	)
	if err != nil {
		return false, err
	}
	return result.MatchedCount > 0, nil
}
