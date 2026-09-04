package data

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PhotoSpec struct {
	Name            string `bson:"name" json:"name"`
	Width           int    `bson:"width" json:"width"`
	Height          int    `bson:"height" json:"height"`
	DPI             int    `bson:"dpi,omitempty" json:"dpi,omitempty"`
	MaxSizeKB       int    `bson:"max_size_kb,omitempty" json:"max_size_kb,omitempty"`
	BackgroundColor string `bson:"background_color,omitempty" json:"background_color,omitempty"`
}

type CustomField struct {
	ID          string   `bson:"id" json:"id"`
	Type        string   `bson:"type" json:"type"`
	Label       string   `bson:"label" json:"label"`
	Required    bool     `bson:"required" json:"required"`
	Unique      bool     `bson:"unique,omitempty" json:"unique,omitempty"`
	Options     []string `bson:"options,omitempty" json:"options,omitempty"`
	Placeholder string   `bson:"placeholder,omitempty" json:"placeholder,omitempty"`
}

type TaskStats struct {
	TotalSubmissions int       `bson:"total_submissions" json:"total_submissions"`
	LastSubmitTime   time.Time `bson:"last_submit_time,omitempty" json:"last_submit_time,omitempty"`
}

type TaskExportInfo struct {
	Status           string    `bson:"status,omitempty" json:"status,omitempty"`
	PersistentID     string    `bson:"persistent_id,omitempty" json:"persistent_id,omitempty"`
	FilenameTemplate string    `bson:"filename_template,omitempty" json:"filename_template,omitempty"`
	ExportKey        string    `bson:"export_key,omitempty" json:"export_key,omitempty"`
	ManifestKey      string    `bson:"manifest_key,omitempty" json:"manifest_key,omitempty"`
	StatusKey        string    `bson:"status_key,omitempty" json:"status_key,omitempty"`
	FileName         string    `bson:"file_name,omitempty" json:"file_name,omitempty"`
	Count            int       `bson:"count,omitempty" json:"count,omitempty"`
	ExportedAt       time.Time `bson:"exported_at,omitempty" json:"exported_at,omitempty"`
	AvailableUntil   time.Time `bson:"available_until,omitempty" json:"available_until,omitempty"`
	ErrorMessage     string    `bson:"error_message,omitempty" json:"error_message,omitempty"`
}

type TaskInvitation struct {
	Token         string     `bson:"token" json:"-"`
	Role          string     `bson:"role" json:"role"`
	InviterUserID string     `bson:"inviter_user_id" json:"-"`
	CreatedAt     time.Time  `bson:"created_at" json:"created_at"`
	UsedBy        string     `bson:"used_by,omitempty" json:"-"`
	UsedAt        *time.Time `bson:"used_at,omitempty" json:"-"`
}

type Task struct {
	ID                           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID                       string             `bson:"user_id" json:"user_id"`
	AdminUserIDs                 []string           `bson:"admin_user_ids,omitempty" json:"admin_user_ids,omitempty"`
	CollaboratorUserIDs          []string           `bson:"collaborator_user_ids,omitempty" json:"collaborator_user_ids,omitempty"`
	CanSubmitMultiple            bool               `bson:"-" json:"can_submit_multiple,omitempty"`
	Expired                      bool               `bson:"-" json:"expired,omitempty"`
	TaskCode                     string             `bson:"task_code,omitempty" json:"task_code,omitempty"`
	Title                        string             `bson:"title" json:"title"`
	Description                  string             `bson:"description" json:"description"`
	PhotoSpec                    PhotoSpec          `bson:"photo_spec" json:"photo_spec"`
	AIAnalysisEnabled            *bool              `bson:"ai_analysis_enabled,omitempty" json:"ai_analysis_enabled,omitempty"`
	BackgroundReplacementEnabled *bool              `bson:"background_replacement_enabled,omitempty" json:"background_replacement_enabled,omitempty"`
	DisallowAlbumPhotos          bool               `bson:"disallow_album_photos,omitempty" json:"disallow_album_photos,omitempty"`
	VerificationCodeEnabled      bool               `bson:"verification_code_enabled,omitempty" json:"verification_code_enabled,omitempty"`
	VerificationCode             string             `bson:"verification_code,omitempty" json:"verification_code,omitempty"`
	MaxSubmissions               int                `bson:"max_submissions" json:"max_submissions"`
	StartTime                    time.Time          `bson:"start_time" json:"start_time"`
	EndTime                      time.Time          `bson:"end_time" json:"end_time"`
	Enabled                      bool               `bson:"enabled" json:"enabled"`
	CustomFields                 []CustomField      `bson:"custom_fields" json:"custom_fields"`
	Stats                        TaskStats          `bson:"stats" json:"stats"`
	ExportInfo                   TaskExportInfo     `bson:"export_info,omitempty" json:"export_info,omitempty"`
	Invitations                  []TaskInvitation   `bson:"invitations,omitempty" json:"-"`
	CreatedAt                    time.Time          `bson:"created_at" json:"created_at"`
	UpdatedAt                    time.Time          `bson:"updated_at" json:"updated_at"`
}

// CanManage reports whether the user is the task creator or a configured task administrator.
func (t *Task) CanManage(userID string) bool {
	if t == nil || userID == "" {
		return false
	}
	if t.UserID == userID {
		return true
	}
	for _, adminUserID := range t.AdminUserIDs {
		if adminUserID == userID {
			return true
		}
	}
	return false
}

func (t *Task) IsCollaborator(userID string) bool {
	if t == nil || userID == "" {
		return false
	}
	for _, collaboratorUserID := range t.CollaboratorUserIDs {
		if collaboratorUserID == userID {
			return true
		}
	}
	return false
}

func (t *Task) AllowsMultipleSubmissions(userID string) bool {
	return t.CanManage(userID) || t.IsCollaborator(userID)
}

func (t *Task) IsAIAnalysisEnabled() bool {
	if t == nil || t.AIAnalysisEnabled == nil {
		return true
	}

	return *t.AIAnalysisEnabled
}

func (t *Task) IsBackgroundReplacementEnabled() bool {
	return t != nil && t.BackgroundReplacementEnabled != nil && *t.BackgroundReplacementEnabled
}
