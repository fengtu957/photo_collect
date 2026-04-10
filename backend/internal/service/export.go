package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"photo-backend/internal/data"
	"regexp"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

const exportLinkTTL = 2 * time.Hour
const exportSourceURLTTL = 24 * time.Hour

var invalidFileNameChars = regexp.MustCompile(`[\\/:*?"<>|\r\n\t]+`)
var exportTemplatePattern = regexp.MustCompile(`\{([^{}]+)\}`)
var duplicateUnderscorePattern = regexp.MustCompile(`_+`)
var windowsReservedNamePattern = regexp.MustCompile(`(?i)^(con|prn|aux|nul|com[1-9]|lpt[1-9])$`)

type ExportService struct {
	taskRepo *data.TaskRepo
	subRepo  *data.SubmissionRepo
	qiniuSvc *QiniuService
}

type ExportTaskRequest struct {
	FilenameTemplate string `json:"filename_template"`
}

type ExportTaskResponse struct {
	Status       string `json:"status"`
	FileName     string `json:"file_name"`
	DownloadURL  string `json:"download_url"`
	ExpiresAt    string `json:"expires_at"`
	Count        int    `json:"count"`
	ErrorMessage string `json:"error_message,omitempty"`
}

type preparedExport struct {
	filenameTemplate string
	exportKey        string
	exportFileName   string
	indexKey         string
	indexFilePath    string
	count            int
}

func NewExportService(taskRepo *data.TaskRepo, subRepo *data.SubmissionRepo, qiniuSvc *QiniuService) *ExportService {
	return &ExportService{
		taskRepo: taskRepo,
		subRepo:  subRepo,
		qiniuSvc: qiniuSvc,
	}
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

	prepared, err := s.prepareMkzipExport(task, submissions, req.FilenameTemplate)
	if err != nil {
		Error(w, 1012, err.Error())
		return
	}
	defer os.RemoveAll(filepath.Dir(prepared.indexFilePath))

	if err := s.qiniuSvc.UploadFile(prepared.indexFilePath, prepared.indexKey); err != nil {
		Error(w, 1013, err.Error())
		return
	}

	persistentID, err := s.qiniuSvc.StartMkzipJob(prepared.indexKey, prepared.exportKey)
	if err != nil {
		Error(w, 1013, err.Error())
		return
	}

	exportInfo := data.TaskExportInfo{
		Status:           "processing",
		PersistentID:     persistentID,
		FilenameTemplate: prepared.filenameTemplate,
		ExportKey:        prepared.exportKey,
		FileName:         prepared.exportFileName,
		Count:            prepared.count,
		ErrorMessage:     "",
	}
	if err := s.taskRepo.UpdateExportInfo(context.Background(), task.ID.Hex(), exportInfo); err != nil {
		Error(w, 1013, err.Error())
		return
	}

	Success(w, buildExportTaskResponse(exportInfo))
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
	if task.ExportInfo.FileName == "" {
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

	expiresAt := time.Now().Add(exportLinkTTL)
	Success(w, &ExportTaskResponse{
		Status:      exportInfo.Status,
		FileName:    exportInfo.FileName,
		DownloadURL: s.qiniuSvc.GetFileURLWithTTL(exportInfo.ExportKey, exportLinkTTL),
		ExpiresAt:   expiresAt.Format(time.RFC3339),
		Count:       exportInfo.Count,
	})
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

func (s *ExportService) prepareMkzipExport(task *data.Task, submissions []*data.Submission, template string) (*preparedExport, error) {
	filenameTemplate := strings.TrimSpace(template)
	if filenameTemplate == "" {
		filenameTemplate = defaultExportTemplate(task)
	}

	exportBaseName := sanitizeBaseName(task.Title)
	if exportBaseName == "" {
		exportBaseName = "photo_export"
	}
	exportFileName := fmt.Sprintf("%s_%s.zip", exportBaseName, time.Now().Format("20060102_150405"))
	exportKey := fmt.Sprintf("exports/%s/%s", task.ID.Hex(), exportFileName)
	indexKey := fmt.Sprintf("exports/%s/index/%s.txt", task.ID.Hex(), time.Now().Format("20060102150405"))

	usedNames := make(map[string]int)
	indexLines := make([]string, 0, len(submissions))
	count := 0

	for index, submission := range submissions {
		if submission == nil || submission.Photo.Deleted || submission.Photo.URL == "" {
			continue
		}

		exportName := buildExportFileName(task, submission, filenameTemplate, index+1, usedNames)
		sourceURL := s.qiniuSvc.GetFileURLWithTTL(submission.Photo.URL, exportSourceURLTTL)
		indexLines = append(indexLines, buildMkzipIndexLine(sourceURL, exportName))
		count++
	}

	if count == 0 {
		return nil, fmt.Errorf("当前任务暂无可导出的图片")
	}

	tmpDir, err := os.MkdirTemp("", "photo-export-index-*")
	if err != nil {
		return nil, err
	}

	indexFilePath := filepath.Join(tmpDir, "mkzip-index.txt")
	content := strings.Join(indexLines, "\n")
	if content != "" {
		content += "\n"
	}
	if err := os.WriteFile(indexFilePath, []byte(content), 0644); err != nil {
		os.RemoveAll(tmpDir)
		return nil, err
	}

	return &preparedExport{
		filenameTemplate: filenameTemplate,
		exportKey:        exportKey,
		exportFileName:   exportFileName,
		indexKey:         indexKey,
		indexFilePath:    indexFilePath,
		count:            count,
	}, nil
}

func buildMkzipIndexLine(sourceURL string, alias string) string {
	return fmt.Sprintf("/url/%s/alias/%s", encodeURLSafeBase64(sourceURL), encodeURLSafeBase64(alias))
}

func encodeURLSafeBase64(value string) string {
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString([]byte(value))
}

func (s *ExportService) syncExportInfo(ctx context.Context, task *data.Task) (data.TaskExportInfo, error) {
	exportInfo := task.ExportInfo
	if exportInfo.FileName == "" {
		return exportInfo, nil
	}
	if exportInfo.PersistentID == "" {
		return exportInfo, nil
	}

	result, err := s.qiniuSvc.QueryPfop(exportInfo.PersistentID)
	if err != nil {
		return exportInfo, err
	}

	next := exportInfo
	next.ErrorMessage = ""

	if result.Code == 0 {
		next.Status = "success"
		outputKey := findCompletedExportKey(result)
		if outputKey != "" {
			next.ExportKey = outputKey
		}
		if next.ExportKey == "" {
			next.Status = "failed"
			next.ErrorMessage = "导出完成但未找到压缩包文件"
		} else {
			if next.ExportedAt.IsZero() {
				next.ExportedAt = time.Now()
			}
		}
	} else if result.Code == 1 || result.Code == 2 {
		next.Status = "processing"
	} else {
		next.Status = "failed"
		next.ErrorMessage = pickPfopErrorMessage(result)
		if next.ErrorMessage == "" {
			next.ErrorMessage = "导出失败，请重新导出"
		}
	}

	if err := s.taskRepo.UpdateExportInfo(ctx, task.ID.Hex(), next); err != nil {
		return next, err
	}
	return next, nil
}

func buildExportTaskResponse(exportInfo data.TaskExportInfo) *ExportTaskResponse {
	response := &ExportTaskResponse{
		Status:       exportInfo.Status,
		FileName:     exportInfo.FileName,
		Count:        exportInfo.Count,
		ErrorMessage: exportInfo.ErrorMessage,
	}
	return response
}

func findCompletedExportKey(result *PfopStatusResult) string {
	if result == nil || len(result.Items) == 0 {
		return ""
	}

	for _, item := range result.Items {
		for _, subItem := range item.Items {
			if subItem.Key != "" {
				return subItem.Key
			}
		}
		if item.Key != "" {
			return item.Key
		}
	}

	return ""
}

func pickPfopErrorMessage(result *PfopStatusResult) string {
	if result == nil {
		return ""
	}
	if result.Desc != "" {
		return result.Desc
	}

	for _, item := range result.Items {
		if item.Desc != "" {
			return item.Desc
		}
		for _, subItem := range item.Items {
			if subItem.Desc != "" {
				return subItem.Desc
			}
		}
	}

	return ""
}

func buildExportFileName(task *data.Task, submission *data.Submission, template string, index int, usedNames map[string]int) string {
	fileExt := path.Ext(submission.Photo.URL)
	if fileExt == "" {
		fileExt = ".jpg"
	}

	baseName := renderExportTemplate(task, submission, template, index)
	baseName = sanitizeBaseName(baseName)
	if baseName == "" {
		baseName = fmt.Sprintf("submission_%03d", index)
	}

	fileName := baseName
	if path.Ext(baseName) == "" {
		fileName = baseName + fileExt
	}
	fileName = ensureUniqueFileName(fileName, usedNames)
	return fileName
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

	rendered := exportTemplatePattern.ReplaceAllStringFunc(trimmedTemplate, func(match string) string {
		token := strings.TrimSpace(match[1 : len(match)-1])
		return resolveExportToken(task, submission, token, index, labelMap)
	})

	return strings.TrimSpace(rendered)
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
	value = strings.TrimRight(value, ".")
	return value
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
