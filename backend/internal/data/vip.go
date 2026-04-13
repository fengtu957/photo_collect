package data

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type VIPMembership struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID    string             `bson:"user_id" json:"user_id"`
	PlanCode  string             `bson:"plan_code" json:"plan_code"`
	Status    string             `bson:"status" json:"status"`
	StartAt   time.Time          `bson:"start_at" json:"start_at"`
	ExpireAt  time.Time          `bson:"expire_at" json:"expire_at"`
	Source    string             `bson:"source,omitempty" json:"source,omitempty"`
	Remark    string             `bson:"remark,omitempty" json:"remark,omitempty"`
	CreatedAt time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time          `bson:"updated_at" json:"updated_at"`
}

type VIPActivationCode struct {
	ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Code        string             `bson:"code" json:"code"`
	PlanCode    string             `bson:"plan_code" json:"plan_code"`
	DurationDay int                `bson:"duration_day" json:"duration_day"`
	Status      string             `bson:"status" json:"status"`
	UsedBy      string             `bson:"used_by,omitempty" json:"used_by,omitempty"`
	UsedAt      time.Time          `bson:"used_at,omitempty" json:"used_at,omitempty"`
	ExpireAt    time.Time          `bson:"expire_at,omitempty" json:"expire_at,omitempty"`
	Remark      string             `bson:"remark,omitempty" json:"remark,omitempty"`
	CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt   time.Time          `bson:"updated_at" json:"updated_at"`
}
