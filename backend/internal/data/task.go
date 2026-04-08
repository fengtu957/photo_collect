package data

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PhotoSpec struct {
	Name   string `bson:"name" json:"name"`
	Width  int    `bson:"width" json:"width"`
	Height int    `bson:"height" json:"height"`
	DPI    int    `bson:"dpi,omitempty" json:"dpi,omitempty"`
}

type CustomField struct {
	ID          string   `bson:"id" json:"id"`
	Type        string   `bson:"type" json:"type"`
	Label       string   `bson:"label" json:"label"`
	Required    bool     `bson:"required" json:"required"`
	Options     []string `bson:"options,omitempty" json:"options,omitempty"`
	Placeholder string   `bson:"placeholder,omitempty" json:"placeholder,omitempty"`
}

type TaskStats struct {
	TotalSubmissions int       `bson:"total_submissions" json:"total_submissions"`
	LastSubmitTime   time.Time `bson:"last_submit_time,omitempty" json:"last_submit_time,omitempty"`
}

type Task struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID       string             `bson:"user_id" json:"user_id"`
	Title        string             `bson:"title" json:"title"`
	Description  string             `bson:"description" json:"description"`
	PhotoSpec    PhotoSpec          `bson:"photo_spec" json:"photo_spec"`
	StartTime    time.Time          `bson:"start_time" json:"start_time"`
	EndTime      time.Time          `bson:"end_time" json:"end_time"`
	Enabled      bool               `bson:"enabled" json:"enabled"`
	CustomFields []CustomField      `bson:"custom_fields" json:"custom_fields"`
	Stats        TaskStats          `bson:"stats" json:"stats"`
	CreatedAt    time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt    time.Time          `bson:"updated_at" json:"updated_at"`
}
