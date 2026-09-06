package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"photo-backend/internal/data"
	"photo-backend/internal/service"
	"strings"
	"syscall"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type storedSubmission struct {
	ID     primitive.ObjectID `bson:"_id"`
	TaskID primitive.ObjectID `bson:"task_id"`
	UserID string             `bson:"user_id"`
	Photo  struct {
		URL     string `bson:"url"`
		Deleted bool   `bson:"deleted"`
	} `bson:"photo"`
}

type migrationSummary struct {
	scanned          int
	planned          int
	migrated         int
	alreadyMigrated  int
	skippedArchives  int
	skippedNonImages int
	skippedInvalid   int
	failed           int
}

type qiniuSource struct {
	accessKey string
	secretKey string
	domain    *url.URL
	client    *http.Client
}

var imageExtensions = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".webp": true,
	".gif": true, ".bmp": true, ".tif": true, ".tiff": true,
	".heic": true, ".heif": true,
}

var archiveExtensions = map[string]bool{
	".zip": true, ".rar": true, ".7z": true, ".tar": true,
	".gz": true, ".tgz": true, ".bz2": true, ".xz": true,
}

func main() {
	execute := flag.Bool("execute", false, "执行上传和数据库更新；不传时仅预演")
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, *execute); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context, execute bool) error {
	mongoURI := strings.TrimSpace(os.Getenv("MONGODB_URI"))
	if mongoURI == "" {
		return errors.New("MONGODB_URI 未配置")
	}

	source, err := newQiniuSource()
	if err != nil {
		return err
	}
	d, err := data.NewData(mongoURI)
	if err != nil {
		return err
	}
	defer func() {
		disconnectCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = d.DB().Client().Disconnect(disconnectCtx)
	}()

	oss := service.NewAliyunOSSService()
	collection := d.DB().Collection("submissions")
	findOptions := options.Find().
		SetProjection(bson.M{"task_id": 1, "user_id": 1, "photo.url": 1, "photo.deleted": 1}).
		SetSort(bson.D{{Key: "_id", Value: 1}})
	cursor, err := collection.Find(ctx, bson.M{
		"photo.url":     bson.M{"$type": "string", "$ne": ""},
		"photo.deleted": bson.M{"$ne": true},
	}, findOptions)
	if err != nil {
		return err
	}
	defer cursor.Close(ctx)

	summary := migrationSummary{}
	for cursor.Next(ctx) {
		var submission storedSubmission
		if err := cursor.Decode(&submission); err != nil {
			summary.failed++
			log.Printf("decode failed: %v", err)
			continue
		}
		summary.scanned++
		migrateSubmission(ctx, collection, source, oss, &submission, execute, &summary)
	}
	if err := cursor.Err(); err != nil {
		return err
	}

	mode := "预演"
	if execute {
		mode = "执行"
	}
	log.Printf(
		"%s完成: scanned=%d planned=%d migrated=%d already_migrated=%d skipped_archives=%d skipped_non_images=%d skipped_invalid=%d failed=%d",
		mode,
		summary.scanned,
		summary.planned,
		summary.migrated,
		summary.alreadyMigrated,
		summary.skippedArchives,
		summary.skippedNonImages,
		summary.skippedInvalid,
		summary.failed,
	)
	if summary.failed > 0 {
		return fmt.Errorf("%d 张图片迁移失败", summary.failed)
	}
	return nil
}

func migrateSubmission(
	ctx context.Context,
	collection *mongo.Collection,
	source *qiniuSource,
	oss *service.AliyunOSSService,
	submission *storedSubmission,
	execute bool,
	summary *migrationSummary,
) {
	sourceValue := strings.TrimSpace(submission.Photo.URL)
	taskID := submission.TaskID.Hex()
	userID := strings.TrimSpace(submission.UserID)
	if sourceValue == "" || submission.TaskID.IsZero() || userID == "" {
		summary.skippedInvalid++
		return
	}
	if oss.IsOwnedFinalKey(userID, taskID, sourceValue) {
		summary.alreadyMigrated++
		return
	}

	sourceKey, err := normalizeSourceKey(sourceValue)
	if err != nil {
		summary.skippedInvalid++
		log.Printf("submission=%s invalid source: %v", submission.ID.Hex(), err)
		return
	}
	extension := strings.ToLower(path.Ext(sourceKey))
	if archiveExtensions[extension] {
		summary.skippedArchives++
		return
	}
	if !imageExtensions[extension] {
		summary.skippedNonImages++
		return
	}

	summary.planned++
	if !execute {
		log.Printf("[dry-run] submission=%s source=%s", submission.ID.Hex(), sourceKey)
		return
	}

	destinationKey := oss.NewFinalPhotoKey(userID, taskID)
	if extension != ".jpg" {
		destinationKey = strings.TrimSuffix(destinationKey, path.Ext(destinationKey)) + extension
	}

	response, err := source.open(ctx, sourceKey)
	if err != nil {
		recordFailure(summary, submission.ID, err)
		return
	}
	defer response.Body.Close()
	if response.ContentLength <= 0 {
		recordFailure(summary, submission.ID, errors.New("七牛未返回有效文件大小"))
		return
	}
	if err := oss.PutObjectStream(destinationKey, response.Body, response.ContentLength); err != nil {
		recordFailure(summary, submission.ID, err)
		return
	}
	objectInfo, err := oss.ProbeObject(destinationKey)
	if err != nil {
		recordFailure(summary, submission.ID, err)
		return
	}
	if objectInfo.Size != response.ContentLength {
		recordFailure(summary, submission.ID, fmt.Errorf("迁移后大小不一致: source=%d destination=%d", response.ContentLength, objectInfo.Size))
		return
	}

	result, err := collection.UpdateOne(
		ctx,
		bson.M{"_id": submission.ID, "photo.url": sourceValue},
		bson.M{"$set": bson.M{
			"photo.url":       destinationKey,
			"photo.file_size": objectInfo.Size,
			"updated_at":      time.Now(),
		}},
	)
	if err != nil {
		recordFailure(summary, submission.ID, err)
		return
	}
	if result.ModifiedCount != 1 {
		recordFailure(summary, submission.ID, errors.New("数据库记录已发生变化，未更新"))
		return
	}

	summary.migrated++
	log.Printf("submission=%s migrated source=%s destination=%s", submission.ID.Hex(), sourceKey, destinationKey)
}

func recordFailure(summary *migrationSummary, submissionID primitive.ObjectID, err error) {
	summary.failed++
	log.Printf("submission=%s failed: %v", submissionID.Hex(), err)
}

func newQiniuSource() (*qiniuSource, error) {
	accessKey := strings.TrimSpace(os.Getenv("QINIU_ACCESS_KEY"))
	secretKey := strings.TrimSpace(os.Getenv("QINIU_SECRET_KEY"))
	domainText := strings.TrimSpace(os.Getenv("QINIU_DOMAIN"))
	if accessKey == "" || secretKey == "" || domainText == "" {
		return nil, errors.New("七牛配置不完整，需要 QINIU_ACCESS_KEY、QINIU_SECRET_KEY、QINIU_DOMAIN")
	}
	domain, err := url.Parse(domainText)
	if err != nil || domain.Scheme == "" || domain.Host == "" {
		return nil, errors.New("QINIU_DOMAIN 无效")
	}
	return &qiniuSource{
		accessKey: accessKey,
		secretKey: secretKey,
		domain:    domain,
		client:    &http.Client{Timeout: 2 * time.Minute},
	}, nil
}

func (s *qiniuSource) open(ctx context.Context, key string) (*http.Response, error) {
	fileURL := s.privateURL(key, time.Hour)
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
	if err != nil {
		return nil, err
	}
	response, err := s.client.Do(request)
	if err != nil {
		return nil, err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		response.Body.Close()
		return nil, fmt.Errorf("七牛文件下载失败: %s", response.Status)
	}
	return response, nil
}

func (s *qiniuSource) privateURL(key string, ttl time.Duration) string {
	u := *s.domain
	u.Path = strings.TrimRight(u.Path, "/") + "/" + strings.TrimLeft(key, "/")
	query := u.Query()
	query.Set("e", fmt.Sprintf("%d", time.Now().Add(ttl).Unix()))
	u.RawQuery = query.Encode()
	unsignedURL := u.String()
	h := hmac.New(sha1.New, []byte(s.secretKey))
	_, _ = h.Write([]byte(unsignedURL))
	token := s.accessKey + ":" + base64.URLEncoding.EncodeToString(h.Sum(nil))
	return unsignedURL + "&token=" + token
}

func normalizeSourceKey(value string) (string, error) {
	normalized := strings.TrimSpace(value)
	if parsed, err := url.Parse(normalized); err == nil && parsed.Scheme != "" {
		normalized = parsed.Path
	} else if queryIndex := strings.Index(normalized, "?"); queryIndex >= 0 {
		normalized = normalized[:queryIndex]
	}
	normalized = strings.Trim(normalized, "/")
	if normalized == "" || path.Clean(normalized) != normalized || strings.Contains(normalized, "://") {
		return "", errors.New("七牛文件 key 无效")
	}
	return normalized, nil
}
