package data

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ExportRecord struct {
	ID                  primitive.ObjectID `bson:"_id,omitempty" json:"-"`
	ExportID            string             `bson:"export_id" json:"id"`
	TaskID              string             `bson:"task_id" json:"task_id"`
	UserID              string             `bson:"user_id" json:"-"`
	Status              string             `bson:"status" json:"status"`
	FilenameTemplate    string             `bson:"filename_template" json:"filename_template"`
	ExportKey           string             `bson:"export_key" json:"-"`
	ManifestKey         string             `bson:"manifest_key" json:"-"`
	StatusKey           string             `bson:"status_key" json:"-"`
	CallbackToken       string             `bson:"callback_token,omitempty" json:"-"`
	AvailabilitySeconds int64              `bson:"availability_seconds" json:"-"`
	FileName            string             `bson:"file_name" json:"file_name"`
	Count               int                `bson:"count" json:"count"`
	ErrorMessage        string             `bson:"error_message,omitempty" json:"error_message,omitempty"`
	ExportedAt          time.Time          `bson:"exported_at,omitempty" json:"exported_at,omitempty"`
	AvailableUntil      time.Time          `bson:"available_until,omitempty" json:"available_until,omitempty"`
	CreatedAt           time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt           time.Time          `bson:"updated_at" json:"updated_at"`
}

type ExportListResult struct {
	List    []*ExportRecord `json:"list"`
	Total   int64           `json:"total"`
	Page    int             `json:"page"`
	HasMore bool            `json:"has_more"`
}
