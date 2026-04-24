package data

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PhotoInfo struct {
	URL      string    `bson:"url" json:"url"`
	FileSize int64     `bson:"file_size" json:"file_size"`
	Width    int       `bson:"width" json:"width"`
	Height   int       `bson:"height" json:"height"`
	Deleted  bool      `bson:"deleted" json:"deleted"`
}

type AIEvaluation struct {
	Status      string            `bson:"status" json:"status"`
	Score       int               `bson:"score" json:"score"`
	Issues      []string          `bson:"issues" json:"issues"`
	Suggestions []string          `bson:"suggestions" json:"suggestions"`
	Breakdown   map[string]int    `bson:"breakdown" json:"breakdown"`
	EvaluatedAt time.Time         `bson:"evaluated_at,omitempty" json:"evaluated_at,omitempty"`
}

type UserInfo struct {
	NickName  string `bson:"nick_name" json:"nick_name"`
	AvatarURL string `bson:"avatar_url" json:"avatar_url"`
}

type Submission struct {
	ID           primitive.ObjectID     `bson:"_id,omitempty" json:"id"`
	TaskID       primitive.ObjectID     `bson:"task_id" json:"task_id"`
	UserID       string                 `bson:"user_id" json:"user_id"`
	VerificationCode string             `bson:"-" json:"verification_code,omitempty"`
	UserInfo     UserInfo               `bson:"user_info" json:"user_info"`
	CustomData   map[string]interface{} `bson:"custom_data" json:"custom_data"`
	Photo        PhotoInfo              `bson:"photo" json:"photo"`
	AIEvaluation AIEvaluation           `bson:"ai_evaluation" json:"ai_evaluation"`
	Status       string                 `bson:"status" json:"status"`
	CreatedAt    time.Time              `bson:"created_at" json:"created_at"`
	UpdatedAt    time.Time              `bson:"updated_at" json:"updated_at"`
}
