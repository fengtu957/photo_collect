package service

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path"
	"photo-backend/internal/biz"
	"photo-backend/internal/data"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

const exportLinkTTL = 2 * time.Hour
const freeExportAvailabilityTTL = 7 * 24 * time.Hour
const vipExportAvailabilityTTL = 30 * 24 * time.Hour
const exportRecoveryInterval = time.Minute
const exportExecutionTimeout = 15 * time.Minute
const exportRecoveryBatchSize = 100

var invalidFileNameChars = regexp.MustCompile(`[\\/:*?"<>|\r\n\t]+`)
var exportTemplatePattern = regexp.MustCompile(`\{([^{}]+)\}`)
var duplicateUnderscorePattern = regexp.MustCompile(`_+`)
var windowsReservedNamePattern = regexp.MustCompile(`(?i)^(con|prn|aux|nul|com[1-9]|lpt[1-9])$`)

type ExportService struct {
	taskRepo    *data.TaskRepo
	subRepo     *data.SubmissionRepo
	exportRepo  *data.ExportRepo
	ossSvc      *AliyunOSSService
	vipUC       *biz.VIPUsecase
	callbackURL string
}

type ExportTaskRequest struct {
	FilenameTemplate string `json:"filename_template"`
}

type ExportTaskResponse struct {
	ID               string `json:"id,omitempty"`
	Status           string `json:"status"`
	FileName         string `json:"file_name"`
	FilenameTemplate string `json:"filename_template,omitempty"`
	DownloadURL      string `json:"download_url"`
	ExpiresAt        string `json:"expires_at"`
	AvailableUntil   string `json:"available_until,omitempty"`
	Count            int    `json:"count"`
	ErrorMessage     string `json:"error_message,omitempty"`
	ExportedAt       string `json:"exported_at,omitempty"`
	CreatedAt        string `json:"created_at,omitempty"`
	UpdatedAt        string `json:"updated_at,omitempty"`
}

type ExportCallbackRequest struct {
	ExportID     string `json:"export_id"`
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message"`
	UpdatedAt    string `json:"updated_at"`
}

type exportEntry struct {
	objectKey string
	fileName  string
}

type preparedExport struct {
	filenameTemplate string
	exportID         string
	exportKey        string
	manifestKey      string
	statusKey        string
	exportFileName   string
	entries          []exportEntry
}

type exportManifest struct {
	Version       int                   `json:"version"`
	TaskID        string                `json:"task_id"`
	ExportID      string                `json:"export_id"`
	ExportKey     string                `json:"export_key"`
	StatusKey     string                `json:"status_key"`
	CallbackURL   string                `json:"callback_url,omitempty"`
	CallbackToken string                `json:"callback_token,omitempty"`
	Entries       []exportManifestEntry `json:"entries"`
}

type exportManifestEntry struct {
	ObjectKey string `json:"object_key"`
	FileName  string `json:"file_name"`
}

type exportStatusDocument struct {
	Status       string `json:"status"`
	ErrorMessage string `json:"error_message"`
	Message      string `json:"message"`
}

func NewExportService(taskRepo *data.TaskRepo, subRepo *data.SubmissionRepo, exportRepo *data.ExportRepo, ossSvc *AliyunOSSService, vipUC *biz.VIPUsecase) *ExportService {
	return &ExportService{
		taskRepo:    taskRepo,
		subRepo:     subRepo,
		exportRepo:  exportRepo,
		ossSvc:      ossSvc,
		vipUC:       vipUC,
		callbackURL: strings.TrimSpace(os.Getenv("EXPORT_CALLBACK_URL")),
	}
}

func (s *ExportService) StartRecovery(ctx context.Context) {
	if s == nil || s.exportRepo == nil {
		return
	}
	go func() {
		recoverExports := func() {
			if err := s.recoverStaleExports(ctx); err != nil {
				log.Printf("recover stale exports failed: %v", err)
			}
		}
		recoverExports()
		ticker := time.NewTicker(exportRecoveryInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				recoverExports()
			}
		}
	}()
}

func (s *ExportService) recoverStaleExports(ctx context.Context) error {
	records, err := s.exportRepo.FindActiveCreatedBefore(ctx, time.Now().Add(-exportExecutionTimeout), exportRecoveryBatchSize)
	if err != nil {
		return err
	}
	for _, record := range records {
		if record == nil {
			continue
		}
		refreshed, syncErr := s.syncExportRecord(ctx, record)
		if syncErr != nil {
			s.markExportFailed(ctx, record, fmt.Errorf("导出状态检查失败: %w", syncErr))
			continue
		}
		if refreshed != nil && (refreshed.Status == "pending" || refreshed.Status == "processing") {
			s.markExportFailed(ctx, refreshed, fmt.Errorf("函数计算执行超时或未回调"))
		}
	}
	return nil
}

func (s *ExportService) ExportTask(w http.ResponseWriter, r *http.Request) {
	task, userID, ok := s.requireCreatorTask(w, r, 1010, 1011)
	if !ok {
		return
	}
	if task.UserID != userID {
		Error(w, 1011, "无权限导出此任务")
		return
	}

	var req ExportTaskRequest
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&req)
	}
	submissions, err := s.subRepo.FindAllByTaskID(context.Background(), task.ID.Hex())
	if err != nil {
		Error(w, 1012, err.Error())
		return
	}
	prepared, err := s.prepareExport(task, submissions, req.FilenameTemplate)
	if err != nil {
		Error(w, 1012, err.Error())
		return
	}
	if err := s.ensureLegacyExportRecord(context.Background(), task); err != nil {
		Error(w, 1013, err.Error())
		return
	}
	if s.callbackURL == "" {
		Error(w, 1013, "导出回调地址未配置")
		return
	}
	callbackToken, err := newExportCallbackToken()
	if err != nil {
		Error(w, 1013, err.Error())
		return
	}
	availabilityTTL := s.buildExportAvailabilityTTL(context.Background(), task.UserID)
	record := &data.ExportRecord{
		ExportID:            prepared.exportID,
		TaskID:              task.ID.Hex(),
		UserID:              task.UserID,
		Status:              "pending",
		FilenameTemplate:    prepared.filenameTemplate,
		ExportKey:           prepared.exportKey,
		ManifestKey:         prepared.manifestKey,
		StatusKey:           prepared.statusKey,
		CallbackToken:       callbackToken,
		AvailabilitySeconds: int64(availabilityTTL / time.Second),
		FileName:            prepared.exportFileName,
		Count:               len(prepared.entries),
	}
	if err := s.exportRepo.Create(context.Background(), record); err != nil {
		Error(w, 1013, err.Error())
		return
	}

	exportInfo := data.TaskExportInfo{
		Status:           record.Status,
		PersistentID:     prepared.exportID,
		FilenameTemplate: prepared.filenameTemplate,
		ExportKey:        prepared.exportKey,
		ManifestKey:      prepared.manifestKey,
		StatusKey:        prepared.statusKey,
		FileName:         prepared.exportFileName,
		Count:            len(prepared.entries),
	}
	if err := s.taskRepo.UpdateExportInfo(context.Background(), task.ID.Hex(), exportInfo); err != nil {
		s.markExportFailed(context.Background(), record, err)
		Error(w, 1013, err.Error())
		return
	}

	manifest := exportManifest{
		Version:       1,
		TaskID:        task.ID.Hex(),
		ExportID:      prepared.exportID,
		ExportKey:     prepared.exportKey,
		StatusKey:     prepared.statusKey,
		CallbackURL:   s.callbackURL,
		CallbackToken: callbackToken,
		Entries:       make([]exportManifestEntry, 0, len(prepared.entries)),
	}
	for _, entry := range prepared.entries {
		manifest.Entries = append(manifest.Entries, exportManifestEntry{
			ObjectKey: entry.objectKey,
			FileName:  entry.fileName,
		})
	}
	manifestBody, err := json.Marshal(manifest)
	if err != nil {
		s.markExportFailed(context.Background(), record, err)
		Error(w, 1013, err.Error())
		return
	}
	if err := s.ossSvc.PutObject(prepared.manifestKey, manifestBody); err != nil {
		s.markExportFailed(context.Background(), record, err)
		Error(w, 1013, err.Error())
		return
	}

	Success(w, buildExportRecordResponse(record))
}

func (s *ExportService) ListExports(w http.ResponseWriter, r *http.Request) {
	task, userID, ok := s.requireCreatorTask(w, r, 1020, 1021)
	if !ok {
		return
	}
	if task.UserID != userID {
		Error(w, 1021, "无权限查看此任务的导出记录")
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	result, err := s.exportRepo.FindByTaskID(context.Background(), task.ID.Hex(), page, limit)
	if err != nil {
		Error(w, 1022, err.Error())
		return
	}

	for _, record := range result.List {
		if record != nil &&
			(record.Status == "pending" || record.Status == "processing") &&
			(record.CallbackToken == "" || time.Since(record.UpdatedAt) > 15*time.Second) {
			_, _ = s.syncExportRecord(context.Background(), record)
		}
	}
	if result.Total == 0 && result.Page == 1 && task.ExportInfo.FileName != "" {
		result.List = []*data.ExportRecord{legacyExportRecord(task)}
		result.Total = 1
		result.HasMore = false
	}
	items := make([]*ExportTaskResponse, 0, len(result.List))
	for _, record := range result.List {
		items = append(items, buildExportRecordResponse(record))
	}
	Success(w, map[string]interface{}{
		"list":     items,
		"total":    result.Total,
		"page":     result.Page,
		"has_more": result.HasMore,
	})
}

func (s *ExportService) AuthorizeExportRecordLink(w http.ResponseWriter, r *http.Request) {
	task, userID, ok := s.requireCreatorTask(w, r, 1023, 1024)
	if !ok {
		return
	}
	if task.UserID != userID {
		Error(w, 1024, "无权限操作此任务")
		return
	}
	exportID := strings.TrimSpace(mux.Vars(r)["exportId"])
	record, err := s.exportRepo.FindByExportID(context.Background(), exportID)
	if err != nil {
		Error(w, 1025, err.Error())
		return
	}
	if record == nil {
		legacyID := legacyExportRecord(task).ExportID
		if exportID != legacyID || task.ExportInfo.FileName == "" {
			Error(w, 1025, "导出记录不存在")
			return
		}
		s.authorizeLegacyExportLink(w, task)
		return
	}
	if record.TaskID != task.ID.Hex() || record.UserID != userID {
		Error(w, 1025, "导出记录不存在")
		return
	}
	if record.Status == "pending" || record.Status == "processing" {
		record, err = s.syncExportRecord(context.Background(), record)
		if err != nil {
			Error(w, 1025, err.Error())
			return
		}
	}
	if record.Status == "pending" || record.Status == "processing" {
		Error(w, 1025, "导出仍在处理中")
		return
	}
	if record.Status == "failed" {
		message := record.ErrorMessage
		if message == "" {
			message = "导出失败，请重新导出"
		}
		Error(w, 1025, message)
		return
	}
	if !record.AvailableUntil.IsZero() && time.Now().After(record.AvailableUntil) {
		Error(w, 1025, "导出下载有效期已结束")
		return
	}
	downloadURL, err := s.ossSvc.GetFileURLWithTTL(record.ExportKey, exportLinkTTL)
	if err != nil {
		Error(w, 1025, err.Error())
		return
	}
	response := buildExportRecordResponse(record)
	response.DownloadURL = downloadURL
	response.ExpiresAt = time.Now().Add(exportLinkTTL).Format(time.RFC3339)
	Success(w, response)
}

func (s *ExportService) ExportCallback(w http.ResponseWriter, r *http.Request) {
	var req ExportCallbackRequest
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64*1024)).Decode(&req); err != nil {
		Error(w, 1026, "回调数据无效")
		return
	}
	req.ExportID = strings.TrimSpace(req.ExportID)
	req.Status = strings.ToLower(strings.TrimSpace(req.Status))
	if req.ExportID == "" || (req.Status != "processing" && req.Status != "success" && req.Status != "failed") {
		Error(w, 1026, "回调状态无效")
		return
	}
	record, err := s.exportRepo.FindByExportID(context.Background(), req.ExportID)
	if err != nil {
		Error(w, 1027, err.Error())
		return
	}
	if record == nil {
		Error(w, 1027, "导出记录不存在")
		return
	}
	callbackToken := strings.TrimSpace(r.Header.Get("X-Export-Callback-Token"))
	if record.CallbackToken == "" || subtle.ConstantTimeCompare([]byte(callbackToken), []byte(record.CallbackToken)) != 1 {
		Error(w, 1027, "回调凭证无效")
		return
	}
	statusTime := time.Now()
	if parsed, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(req.UpdatedAt)); parseErr == nil {
		statusTime = parsed
	}
	if err := s.applyExportRecordStatus(context.Background(), record, req.Status, req.ErrorMessage, statusTime); err != nil {
		Error(w, 1028, err.Error())
		return
	}
	Success(w, nil)
}

func (s *ExportService) SyncExportStatus(w http.ResponseWriter, r *http.Request) {
	task, userID, ok := s.requireCreatorTask(w, r, 1017, 1018)
	if !ok {
		return
	}
	if task.UserID != userID {
		Error(w, 1018, "无权限操作此任务")
		return
	}
	if task.ExportInfo.FileName == "" {
		Error(w, 1018, "当前任务还没有导出记录")
		return
	}
	exportInfo, err := s.syncExportInfo(context.Background(), task)
	if err != nil {
		Error(w, 1019, err.Error())
		return
	}
	Success(w, buildExportTaskResponse(exportInfo))
}

func (s *ExportService) AuthorizeExportLink(w http.ResponseWriter, r *http.Request) {
	task, userID, ok := s.requireCreatorTask(w, r, 1014, 1015)
	if !ok {
		return
	}
	if task.UserID != userID {
		Error(w, 1015, "无权限操作此任务")
		return
	}
	exportInfo := task.ExportInfo
	if exportInfo.FileName == "" {
		Error(w, 1015, "当前任务还没有导出记录")
		return
	}
	exportInfo, err := s.syncExportInfo(context.Background(), task)
	if err != nil {
		Error(w, 1016, err.Error())
		return
	}
	if exportInfo.Status == "processing" || exportInfo.Status == "pending" {
		Error(w, 1016, "导出仍在处理中，请稍后刷新状态")
		return
	}
	if exportInfo.Status == "failed" {
		message := exportInfo.ErrorMessage
		if message == "" {
			message = "导出失败，请重新导出"
		}
		Error(w, 1016, message)
		return
	}
	if exportInfo.ExportKey == "" {
		Error(w, 1016, "当前任务导出文件不存在")
		return
	}
	if !exportInfo.AvailableUntil.IsZero() && time.Now().After(exportInfo.AvailableUntil) {
		Error(w, 1016, fmt.Sprintf("导出下载有效期已结束，仅可在 %s 前重新生成下载链接", exportInfo.AvailableUntil.Format("2006-01-02 15:04")))
		return
	}

	downloadURL, err := s.ossSvc.GetFileURLWithTTL(exportInfo.ExportKey, exportLinkTTL)
	if err != nil {
		Error(w, 1016, err.Error())
		return
	}
	expiresAt := time.Now().Add(exportLinkTTL)
	Success(w, &ExportTaskResponse{
		Status:         exportInfo.Status,
		FileName:       exportInfo.FileName,
		DownloadURL:    downloadURL,
		ExpiresAt:      expiresAt.Format(time.RFC3339),
		AvailableUntil: formatTimeRFC3339(exportInfo.AvailableUntil),
		Count:          exportInfo.Count,
	})
}

func (s *ExportService) authorizeLegacyExportLink(w http.ResponseWriter, task *data.Task) {
	exportInfo, err := s.syncExportInfo(context.Background(), task)
	if err != nil {
		Error(w, 1025, err.Error())
		return
	}
	if exportInfo.Status == "processing" || exportInfo.Status == "pending" {
		Error(w, 1025, "导出仍在处理中")
		return
	}
	if exportInfo.Status == "failed" {
		message := exportInfo.ErrorMessage
		if message == "" {
			message = "导出失败，请重新导出"
		}
		Error(w, 1025, message)
		return
	}
	if exportInfo.ExportKey == "" {
		Error(w, 1025, "当前任务导出文件不存在")
		return
	}
	if !exportInfo.AvailableUntil.IsZero() && time.Now().After(exportInfo.AvailableUntil) {
		Error(w, 1025, "导出下载有效期已结束")
		return
	}
	downloadURL, err := s.ossSvc.GetFileURLWithTTL(exportInfo.ExportKey, exportLinkTTL)
	if err != nil {
		Error(w, 1025, err.Error())
		return
	}
	response := buildExportTaskResponse(exportInfo)
	response.ID = legacyExportRecord(task).ExportID
	response.DownloadURL = downloadURL
	response.ExpiresAt = time.Now().Add(exportLinkTTL).Format(time.RFC3339)
	Success(w, response)
}

func (s *ExportService) markExportFailed(ctx context.Context, record *data.ExportRecord, exportErr error) {
	if record == nil || exportErr == nil {
		return
	}
	_ = s.applyExportRecordStatus(ctx, record, "failed", exportErr.Error(), time.Now())
}

func (s *ExportService) ensureLegacyExportRecord(ctx context.Context, task *data.Task) error {
	if task == nil || task.ExportInfo.FileName == "" {
		return nil
	}
	result, err := s.exportRepo.FindByTaskID(ctx, task.ID.Hex(), 1, 1)
	if err != nil {
		return err
	}
	if result.Total > 0 {
		return nil
	}
	return s.exportRepo.Create(ctx, legacyExportRecord(task))
}

func (s *ExportService) applyExportRecordStatus(ctx context.Context, record *data.ExportRecord, status string, errorMessage string, statusTime time.Time) error {
	if record == nil {
		return fmt.Errorf("导出记录不存在")
	}
	if record.Status == "success" || (record.Status == "failed" && status != "success") {
		return nil
	}
	if statusTime.IsZero() {
		statusTime = time.Now()
	}
	record.Status = status
	record.ErrorMessage = ""
	if status == "success" {
		record.ExportedAt = statusTime
		availabilitySeconds := record.AvailabilitySeconds
		if availabilitySeconds <= 0 {
			availabilitySeconds = int64(freeExportAvailabilityTTL / time.Second)
		}
		record.AvailableUntil = statusTime.Add(time.Duration(availabilitySeconds) * time.Second)
	} else if status == "failed" {
		record.ErrorMessage = strings.TrimSpace(errorMessage)
		if record.ErrorMessage == "" {
			record.ErrorMessage = "导出失败，请重新导出"
		}
	}
	updated, err := s.exportRepo.UpdateStatus(ctx, record)
	if err != nil {
		return err
	}
	if !updated {
		latest, findErr := s.exportRepo.FindByExportID(ctx, record.ExportID)
		if findErr != nil {
			return findErr
		}
		if latest != nil {
			*record = *latest
		}
		return nil
	}
	exportInfo := exportRecordToTaskExportInfo(record)
	return s.taskRepo.UpdateLatestExportStatus(ctx, record.TaskID, record.ExportID, exportInfo)
}

func (s *ExportService) syncExportRecord(ctx context.Context, record *data.ExportRecord) (*data.ExportRecord, error) {
	if record == nil || record.ExportKey == "" || s == nil || s.ossSvc == nil {
		return record, nil
	}
	objectInfo, err := s.ossSvc.ProbeObject(record.ExportKey)
	if err == nil && objectInfo != nil && objectInfo.Size > 0 {
		if err := s.applyExportRecordStatus(ctx, record, "success", "", time.Now()); err != nil {
			return record, err
		}
		return record, nil
	}
	if err != nil && !isOSSObjectNotFound(err) {
		return record, err
	}
	if record.StatusKey == "" {
		return record, nil
	}
	statusBody, statusErr := s.ossSvc.ReadSmallObject(record.StatusKey, 64*1024)
	if statusErr != nil {
		if isOSSObjectNotFound(statusErr) {
			return record, nil
		}
		return record, statusErr
	}
	var statusDoc exportStatusDocument
	if err := json.Unmarshal(statusBody, &statusDoc); err != nil {
		return record, fmt.Errorf("解析导出状态失败: %w", err)
	}
	status := strings.ToLower(strings.TrimSpace(statusDoc.Status))
	if status != "processing" && status != "failed" {
		return record, nil
	}
	message := strings.TrimSpace(statusDoc.ErrorMessage)
	if message == "" {
		message = strings.TrimSpace(statusDoc.Message)
	}
	if err := s.applyExportRecordStatus(ctx, record, status, message, time.Now()); err != nil {
		return record, err
	}
	return record, nil
}

func (s *ExportService) syncExportInfo(ctx context.Context, task *data.Task) (data.TaskExportInfo, error) {
	info := task.ExportInfo
	if info.FileName == "" || info.ExportKey == "" || s == nil || s.ossSvc == nil {
		return info, nil
	}

	objectInfo, err := s.ossSvc.ProbeObject(info.ExportKey)
	if err == nil && objectInfo != nil && objectInfo.Size > 0 {
		changed := info.Status != "success" || info.ErrorMessage != ""
		if info.ExportedAt.IsZero() {
			info.ExportedAt = time.Now()
			changed = true
		}
		if info.AvailableUntil.IsZero() {
			info.AvailableUntil = s.buildExportAvailableUntil(ctx, task.UserID, info.ExportedAt)
			changed = true
		}
		info.Status = "success"
		info.ErrorMessage = ""
		if changed {
			if err := s.taskRepo.UpdateExportInfo(ctx, task.ID.Hex(), info); err != nil {
				return info, err
			}
		}
		return info, nil
	}
	if err != nil && !isOSSObjectNotFound(err) {
		return info, err
	}

	statusKey := info.StatusKey
	if statusKey == "" && info.PersistentID != "" {
		statusKey = s.ossSvc.NewExportStatusKey(task.ID.Hex(), info.PersistentID)
	}
	if statusKey == "" {
		return info, nil
	}
	statusBody, statusErr := s.ossSvc.ReadSmallObject(statusKey, 64*1024)
	if statusErr != nil {
		if isOSSObjectNotFound(statusErr) {
			return info, nil
		}
		return info, statusErr
	}
	var statusDoc exportStatusDocument
	if err := json.Unmarshal(statusBody, &statusDoc); err != nil {
		return info, fmt.Errorf("解析导出状态失败: %w", err)
	}
	status := strings.ToLower(strings.TrimSpace(statusDoc.Status))
	if status != "failed" {
		return info, nil
	}
	message := strings.TrimSpace(statusDoc.ErrorMessage)
	if message == "" {
		message = strings.TrimSpace(statusDoc.Message)
	}
	if message == "" {
		message = "导出失败，请重新导出"
	}
	if info.Status == "failed" && info.ErrorMessage == message {
		return info, nil
	}
	info.Status = "failed"
	info.ErrorMessage = message
	if err := s.taskRepo.UpdateExportInfo(ctx, task.ID.Hex(), info); err != nil {
		return info, err
	}
	return info, nil
}

func (s *ExportService) requireCreatorTask(w http.ResponseWriter, r *http.Request, unauthorizedCode int, taskCode int) (*data.Task, string, bool) {
	taskID := mux.Vars(r)["id"]
	userID, ok := r.Context().Value(UserIDKey).(string)
	if !ok {
		Error(w, unauthorizedCode, "unauthorized")
		return nil, "", false
	}
	task, err := s.taskRepo.FindByID(context.Background(), taskID)
	if err != nil {
		Error(w, taskCode, err.Error())
		return nil, "", false
	}
	if task == nil {
		Error(w, taskCode, "任务不存在")
		return nil, "", false
	}
	return task, userID, true
}

func (s *ExportService) prepareExport(task *data.Task, submissions []*data.Submission, template string) (*preparedExport, error) {
	if s == nil || s.ossSvc == nil {
		return nil, fmt.Errorf("阿里云 OSS 未配置")
	}
	filenameTemplate := strings.TrimSpace(template)
	if filenameTemplate == "" {
		filenameTemplate = defaultExportTemplate(task)
	}
	exportBaseName := sanitizeBaseName(task.Title)
	if exportBaseName == "" {
		exportBaseName = "photo_export"
	}
	exportFileName := fmt.Sprintf("%s_%s.zip", exportBaseName, time.Now().Format("20060102_150405"))
	usedNames := make(map[string]int)
	entries := make([]exportEntry, 0, len(submissions))
	for index, submission := range submissions {
		if submission == nil || submission.Photo.Deleted || submission.Photo.URL == "" {
			continue
		}
		if strings.TrimSpace(submission.UserID) == "" || !s.ossSvc.IsOwnedFinalKey(submission.UserID, task.ID.Hex(), submission.Photo.URL) {
			return nil, fmt.Errorf("提交记录 %s 的照片存储路径无效", submission.ID.Hex())
		}
		entries = append(entries, exportEntry{
			objectKey: submission.Photo.URL,
			fileName:  buildExportFileName(task, submission, filenameTemplate, index+1, usedNames),
		})
	}
	if len(entries) == 0 {
		return nil, fmt.Errorf("当前任务暂无可导出的图片")
	}
	exportID := fmt.Sprintf("%d", time.Now().UTC().UnixNano())
	return &preparedExport{
		filenameTemplate: filenameTemplate,
		exportID:         exportID,
		exportKey:        s.ossSvc.NewExportKey(task.ID.Hex(), exportID),
		manifestKey:      s.ossSvc.NewExportManifestKey(task.ID.Hex(), exportID),
		statusKey:        s.ossSvc.NewExportStatusKey(task.ID.Hex(), exportID),
		exportFileName:   exportFileName,
		entries:          entries,
	}, nil
}

func buildExportTaskResponse(exportInfo data.TaskExportInfo) *ExportTaskResponse {
	return &ExportTaskResponse{
		ID:               exportInfo.PersistentID,
		Status:           exportInfo.Status,
		FileName:         exportInfo.FileName,
		FilenameTemplate: exportInfo.FilenameTemplate,
		AvailableUntil:   formatTimeRFC3339(exportInfo.AvailableUntil),
		Count:            exportInfo.Count,
		ErrorMessage:     exportInfo.ErrorMessage,
		ExportedAt:       formatTimeRFC3339(exportInfo.ExportedAt),
	}
}

func (s *ExportService) buildExportAvailableUntil(ctx context.Context, userID string, exportedAt time.Time) time.Time {
	return exportedAt.Add(s.buildExportAvailabilityTTL(ctx, userID))
}

func (s *ExportService) buildExportAvailabilityTTL(ctx context.Context, userID string) time.Duration {
	ttl := freeExportAvailabilityTTL
	if s.vipUC != nil {
		entitlements, err := s.vipUC.GetUserEntitlements(ctx, userID)
		if err == nil && entitlements != nil && entitlements.IsVIP {
			ttl = vipExportAvailabilityTTL
		}
	}
	return ttl
}

func buildExportRecordResponse(record *data.ExportRecord) *ExportTaskResponse {
	if record == nil {
		return &ExportTaskResponse{}
	}
	return &ExportTaskResponse{
		ID:               record.ExportID,
		Status:           record.Status,
		FileName:         record.FileName,
		FilenameTemplate: record.FilenameTemplate,
		AvailableUntil:   formatTimeRFC3339(record.AvailableUntil),
		Count:            record.Count,
		ErrorMessage:     record.ErrorMessage,
		ExportedAt:       formatTimeRFC3339(record.ExportedAt),
		CreatedAt:        formatTimeRFC3339(record.CreatedAt),
		UpdatedAt:        formatTimeRFC3339(record.UpdatedAt),
	}
}

func exportRecordToTaskExportInfo(record *data.ExportRecord) data.TaskExportInfo {
	if record == nil {
		return data.TaskExportInfo{}
	}
	return data.TaskExportInfo{
		Status:           record.Status,
		PersistentID:     record.ExportID,
		FilenameTemplate: record.FilenameTemplate,
		ExportKey:        record.ExportKey,
		ManifestKey:      record.ManifestKey,
		StatusKey:        record.StatusKey,
		FileName:         record.FileName,
		Count:            record.Count,
		ExportedAt:       record.ExportedAt,
		AvailableUntil:   record.AvailableUntil,
		ErrorMessage:     record.ErrorMessage,
	}
}

func legacyExportRecord(task *data.Task) *data.ExportRecord {
	info := task.ExportInfo
	exportID := info.PersistentID
	if exportID == "" {
		exportID = "legacy_" + task.ID.Hex()
	}
	createdAt := info.ExportedAt
	if createdAt.IsZero() {
		createdAt = task.UpdatedAt
	}
	return &data.ExportRecord{
		ExportID:         exportID,
		TaskID:           task.ID.Hex(),
		UserID:           task.UserID,
		Status:           info.Status,
		FilenameTemplate: info.FilenameTemplate,
		ExportKey:        info.ExportKey,
		ManifestKey:      info.ManifestKey,
		StatusKey:        info.StatusKey,
		FileName:         info.FileName,
		Count:            info.Count,
		ErrorMessage:     info.ErrorMessage,
		ExportedAt:       info.ExportedAt,
		AvailableUntil:   info.AvailableUntil,
		CreatedAt:        createdAt,
		UpdatedAt:        task.UpdatedAt,
	}
}

func newExportCallbackToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("生成导出回调凭证失败: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}

func formatTimeRFC3339(value time.Time) string {
	if value.IsZero() {
		return ""
	}
	return value.Format(time.RFC3339)
}

func buildExportFileName(task *data.Task, submission *data.Submission, template string, index int, usedNames map[string]int) string {
	fileExt := path.Ext(submission.Photo.URL)
	if fileExt == "" {
		fileExt = ".jpg"
	}
	baseName := sanitizeBaseName(renderExportTemplate(task, submission, template, index))
	if baseName == "" {
		baseName = fmt.Sprintf("submission_%03d", index)
	}
	fileName := baseName
	if path.Ext(baseName) == "" {
		fileName = baseName + fileExt
	}
	return ensureUniqueFileName(fileName, usedNames)
}

func renderExportTemplate(task *data.Task, submission *data.Submission, template string, index int) string {
	trimmedTemplate := strings.TrimSpace(template)
	if trimmedTemplate == "" {
		trimmedTemplate = defaultExportTemplate(task)
	}
	labelMap := make(map[string]string)
	for _, field := range task.CustomFields {
		if field.Label != "" {
			labelMap[field.Label] = field.ID
		}
	}
	return strings.TrimSpace(exportTemplatePattern.ReplaceAllStringFunc(trimmedTemplate, func(match string) string {
		token := strings.TrimSpace(match[1 : len(match)-1])
		return resolveExportToken(task, submission, token, index, labelMap)
	}))
}

func defaultExportTemplate(task *data.Task) string {
	if len(task.CustomFields) > 0 && task.CustomFields[0].Label != "" {
		return fmt.Sprintf("{index}_{field:%s}_{nick_name}", task.CustomFields[0].Label)
	}
	return "{index}_{nick_name}"
}

func resolveExportToken(task *data.Task, submission *data.Submission, token string, index int, labelMap map[string]string) string {
	switch token {
	case "index":
		return fmt.Sprintf("%03d", index)
	case "nick_name":
		return submission.UserInfo.NickName
	case "created_at":
		return submission.CreatedAt.Format("20060102_150405")
	case "task_title":
		return task.Title
	}
	if strings.HasPrefix(token, "field:") {
		fieldName := strings.TrimSpace(strings.TrimPrefix(token, "field:"))
		fieldID := fieldName
		if labelMap[fieldName] != "" {
			fieldID = labelMap[fieldName]
		}
		return stringifyExportValue(submission.CustomData[fieldID])
	}
	return ""
}

func stringifyExportValue(value interface{}) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	case int:
		return fmt.Sprintf("%d", v)
	case int32:
		return fmt.Sprintf("%d", v)
	case int64:
		return fmt.Sprintf("%d", v)
	case float32:
		return trimFloatString(fmt.Sprintf("%.2f", v))
	case float64:
		return trimFloatString(fmt.Sprintf("%.2f", v))
	case bool:
		if v {
			return "true"
		}
		return "false"
	case []string:
		return strings.Join(v, "_")
	case []interface{}:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			part := stringifyExportValue(item)
			if part != "" {
				parts = append(parts, part)
			}
		}
		return strings.Join(parts, "_")
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func trimFloatString(value string) string {
	value = strings.TrimRight(value, "0")
	return strings.TrimRight(value, ".")
}

func sanitizeBaseName(name string) string {
	trimmed := strings.TrimSpace(name)
	trimmed = invalidFileNameChars.ReplaceAllString(trimmed, "_")
	trimmed = strings.ReplaceAll(trimmed, " ", "_")
	trimmed = duplicateUnderscorePattern.ReplaceAllString(trimmed, "_")
	trimmed = strings.Trim(trimmed, " ._")
	if len(trimmed) > 120 {
		trimmed = strings.Trim(trimmed[:120], " ._")
	}
	if windowsReservedNamePattern.MatchString(trimmed) {
		trimmed = "file_" + trimmed
	}
	return trimmed
}

func ensureUniqueFileName(fileName string, usedNames map[string]int) string {
	if usedNames[fileName] == 0 {
		usedNames[fileName] = 1
		return fileName
	}
	ext := path.Ext(fileName)
	base := strings.TrimSuffix(fileName, ext)
	usedNames[fileName]++
	return fmt.Sprintf("%s_%d%s", base, usedNames[fileName], ext)
}
