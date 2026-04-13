package data

import (
	"context"
	"errors"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type VIPRepo struct {
	data *Data
}

func NewVIPRepo(data *Data) *VIPRepo {
	return &VIPRepo{data: data}
}

func (r *VIPRepo) EnsureIndexes(ctx context.Context) error {
	membershipIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "user_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	}
	if _, err := r.data.DB().Collection("vip_memberships").Indexes().CreateMany(ctx, membershipIndexes); err != nil {
		return err
	}

	codeIndexes := []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "code", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	}
	_, err := r.data.DB().Collection("vip_activation_codes").Indexes().CreateMany(ctx, codeIndexes)
	return err
}

func (r *VIPRepo) FindMembershipByUserID(ctx context.Context, userID string) (*VIPMembership, error) {
	var membership VIPMembership
	err := r.data.DB().Collection("vip_memberships").FindOne(ctx, bson.M{"user_id": userID}).Decode(&membership)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &membership, nil
}

func (r *VIPRepo) SaveMembership(ctx context.Context, membership *VIPMembership) error {
	if membership == nil {
		return errors.New("membership 不能为空")
	}
	now := time.Now()
	if membership.CreatedAt.IsZero() {
		membership.CreatedAt = now
	}
	membership.UpdatedAt = now
	_, err := r.data.DB().Collection("vip_memberships").UpdateOne(
		ctx,
		bson.M{"user_id": membership.UserID},
		bson.M{"$set": membership},
		options.Update().SetUpsert(true),
	)
	return err
}

func (r *VIPRepo) CreateActivationCodes(ctx context.Context, codes []*VIPActivationCode) error {
	if len(codes) == 0 {
		return nil
	}
	docs := make([]interface{}, 0, len(codes))
	now := time.Now()
	for _, code := range codes {
		if code == nil {
			continue
		}
		code.Code = strings.ToUpper(strings.TrimSpace(code.Code))
		code.CreatedAt = now
		code.UpdatedAt = now
		docs = append(docs, code)
	}
	if len(docs) == 0 {
		return nil
	}
	_, err := r.data.DB().Collection("vip_activation_codes").InsertMany(ctx, docs)
	return err
}

func (r *VIPRepo) FindCodeByCode(ctx context.Context, code string) (*VIPActivationCode, error) {
	var activationCode VIPActivationCode
	err := r.data.DB().Collection("vip_activation_codes").FindOne(ctx, bson.M{
		"code": strings.ToUpper(strings.TrimSpace(code)),
	}).Decode(&activationCode)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &activationCode, nil
}

func (r *VIPRepo) ConsumeCode(ctx context.Context, code string, userID string) (*VIPActivationCode, error) {
	normalizedCode := strings.ToUpper(strings.TrimSpace(code))
	now := time.Now()
	filter := bson.M{
		"code":   normalizedCode,
		"status": "unused",
		"$or": []bson.M{
			{"expire_at": bson.M{"$exists": false}},
			{"expire_at": time.Time{}},
			{"expire_at": bson.M{"$gt": now}},
		},
	}
	update := bson.M{
		"$set": bson.M{
			"status":     "used",
			"used_by":    userID,
			"used_at":    now,
			"updated_at": now,
		},
	}

	var activationCode VIPActivationCode
	err := r.data.DB().Collection("vip_activation_codes").FindOneAndUpdate(
		ctx,
		filter,
		update,
		options.FindOneAndUpdate().SetReturnDocument(options.Before),
	).Decode(&activationCode)
	if err == mongo.ErrNoDocuments {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &activationCode, nil
}

func (r *VIPRepo) DisableCode(ctx context.Context, code string) error {
	now := time.Now()
	_, err := r.data.DB().Collection("vip_activation_codes").UpdateOne(
		ctx,
		bson.M{"code": strings.ToUpper(strings.TrimSpace(code))},
		bson.M{"$set": bson.M{"status": "disabled", "updated_at": now}},
	)
	return err
}
