package oss

import (
	"context"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"XWFrame/internal/frame/consts"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/gogf/gf/v2/errors/gcode"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/google/uuid"
)

const (
	defaultRegion        = "ap-southeast-1"
	defaultBucket        = "token-ai"
	defaultKeyPrefix     = "resource/"
	defaultResourceURL   = "https://token-ai.s3.ap-southeast-1.amazonaws.com/resource/"
	defaultExpiresSecond = 600
	maxImageSize         = 10 * 1024 * 1024
	maxVideoSize         = 200 * 1024 * 1024
)

var ossErrMsgs = map[string]map[string]string{
	"invalid_filename": {
		"zh-CN": "文件名不合法", "zh-TW": "檔案名稱不合法", "en-US": "Invalid filename", "ja-JP": "ファイル名が正しくありません", "ko-KR": "파일명이 유효하지 않습니다", "vi-VN": "Tên tệp không hợp lệ", "th-TH": "ชื่อไฟล์ไม่ถูกต้อง",
	},
	"unsupported_file_type": {
		"zh-CN": "不支持的文件类型", "zh-TW": "不支援的檔案類型", "en-US": "Unsupported file type", "ja-JP": "サポートされていないファイル形式です", "ko-KR": "지원하지 않는 파일 형식입니다", "vi-VN": "Loại tệp không được hỗ trợ", "th-TH": "ไม่รองรับประเภทไฟล์นี้",
	},
	"invalid_file_size": {
		"zh-CN": "文件大小不合法", "zh-TW": "檔案大小不合法", "en-US": "Invalid file size", "ja-JP": "ファイルサイズが正しくありません", "ko-KR": "파일 크기가 유효하지 않습니다", "vi-VN": "Kích thước tệp không hợp lệ", "th-TH": "ขนาดไฟล์ไม่ถูกต้อง",
	},
	"file_size_exceeds_limit": {
		"zh-CN": "文件大小超过限制", "zh-TW": "檔案大小超過限制", "en-US": "File size exceeds the limit", "ja-JP": "ファイルサイズが上限を超えています", "ko-KR": "파일 크기가 제한을 초과했습니다", "vi-VN": "Kích thước tệp vượt quá giới hạn", "th-TH": "ขนาดไฟล์เกินขีดจำกัด",
	},
	"oss_config_missing": {
		"zh-CN": "对象存储配置缺失", "zh-TW": "物件儲存配置缺失", "en-US": "Object storage config is missing", "ja-JP": "オブジェクトストレージ設定が不足しています", "ko-KR": "객체 스토리지 설정이 누락되었습니다", "vi-VN": "Thiếu cấu hình lưu trữ đối tượng", "th-TH": "การตั้งค่าที่เก็บวัตถุหายไป",
	},
	"invalid_object_key": {
		"zh-CN": "对象路径不合法", "zh-TW": "物件路徑不合法", "en-US": "Invalid object key", "ja-JP": "オブジェクトキーが正しくありません", "ko-KR": "객체 경로가 유효하지 않습니다", "vi-VN": "Đường dẫn đối tượng không hợp lệ", "th-TH": "เส้นทางวัตถุไม่ถูกต้อง",
	},
}

type Service interface {
	PresignUpload(ctx context.Context, req *PresignUploadReq) (*PresignUploadRes, error)
	PresignGet(ctx context.Context, req *PresignGetReq) (*PresignGetRes, error)
}

type service struct{}

func NewService() Service {
	return &service{}
}

type PresignUploadReq struct {
	UserId      int64
	Filename    string
	ContentType string
	Size        int64
}

type PresignUploadRes struct {
	UploadURL string            `json:"upload_url"`
	Method    string            `json:"method"`
	Headers   map[string]string `json:"headers"`
	Key       string            `json:"key"`
	FileURL   string            `json:"file_url"`
	ExpiresIn int               `json:"expires_in"`
}

type PresignGetReq struct {
	Key string
}

type PresignGetRes struct {
	ViewURL   string `json:"view_url"`
	Key       string `json:"key"`
	ExpiresIn int    `json:"expires_in"`
}

type ossConfig struct {
	Region          string
	Bucket          string
	KeyPrefix       string
	ResourceURL     string
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	ExpiresIn       int
}

func (s *service) PresignUpload(ctx context.Context, req *PresignUploadReq) (*PresignUploadRes, error) {
	ext, normalizedContentType, err := validateUpload(ctx, req)
	if err != nil {
		return nil, err
	}
	cfg := loadOSSConfig(ctx)
	if cfg.Region == "" || cfg.Bucket == "" {
		return nil, ossErr(ctx, "oss_config_missing")
	}

	key := buildObjectKey(cfg.KeyPrefix, req.UserId, ext)
	awsCfg, err := loadAWSConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(awsCfg)
	presignClient := s3.NewPresignClient(client)
	out, err := presignClient.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket:      aws.String(cfg.Bucket),
		Key:         aws.String(key),
		ContentType: aws.String(normalizedContentType),
	}, s3.WithPresignExpires(time.Duration(cfg.ExpiresIn)*time.Second))
	if err != nil {
		return nil, err
	}
	return &PresignUploadRes{
		UploadURL: out.URL,
		Method:    http.MethodPut,
		Headers:   map[string]string{"Content-Type": normalizedContentType},
		Key:       key,
		FileURL:   buildFileURL(cfg, key),
		ExpiresIn: cfg.ExpiresIn,
	}, nil
}

func (s *service) PresignGet(ctx context.Context, req *PresignGetReq) (*PresignGetRes, error) {
	cfg := loadOSSConfig(ctx)
	if cfg.Region == "" || cfg.Bucket == "" {
		return nil, ossErr(ctx, "oss_config_missing")
	}
	key, err := normalizeObjectKey(ctx, req.Key, cfg)
	if err != nil {
		return nil, err
	}
	awsCfg, err := loadAWSConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	client := s3.NewFromConfig(awsCfg)
	presignClient := s3.NewPresignClient(client)
	out, err := presignClient.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(cfg.Bucket),
		Key:    aws.String(key),
	}, s3.WithPresignExpires(time.Duration(cfg.ExpiresIn)*time.Second))
	if err != nil {
		return nil, err
	}
	return &PresignGetRes{ViewURL: out.URL, Key: key, ExpiresIn: cfg.ExpiresIn}, nil
}

func validateUpload(ctx context.Context, req *PresignUploadReq) (string, string, error) {
	filename := strings.TrimSpace(req.Filename)
	if filename == "" || strings.Contains(filename, "/") || strings.Contains(filename, "\\") {
		return "", "", ossErr(ctx, "invalid_filename")
	}
	if req.Size <= 0 {
		return "", "", ossErr(ctx, "invalid_file_size")
	}
	ext := strings.ToLower(filepath.Ext(filename))
	contentType := strings.ToLower(strings.TrimSpace(req.ContentType))
	normalizedExt, normalizedContentType, maxSize, ok := allowedUploadType(ext, contentType)
	if !ok {
		return "", "", ossErr(ctx, "unsupported_file_type")
	}
	if req.Size > maxSize {
		return "", "", ossErr(ctx, "file_size_exceeds_limit")
	}
	return normalizedExt, normalizedContentType, nil
}

func allowedUploadType(ext, contentType string) (string, string, int64, bool) {
	types := map[string]struct {
		ext     string
		maxSize int64
	}{
		"image/jpeg":       {".jpg", maxImageSize},
		"image/png":        {".png", maxImageSize},
		"image/webp":       {".webp", maxImageSize},
		"image/gif":        {".gif", maxImageSize},
		"video/mp4":        {".mp4", maxVideoSize},
		"video/webm":       {".webm", maxVideoSize},
		"video/quicktime":  {".mov", maxVideoSize},
		"video/x-msvideo":  {".avi", maxVideoSize},
		"video/x-matroska": {".mkv", maxVideoSize},
		"video/x-m4v":      {".m4v", maxVideoSize},
		"video/3gpp":       {".3gp", maxVideoSize},
	}
	allowedExt := map[string]string{
		".jpg": "image/jpeg", ".jpeg": "image/jpeg", ".png": "image/png", ".webp": "image/webp", ".gif": "image/gif",
		".mp4": "video/mp4", ".webm": "video/webm", ".mov": "video/quicktime",
		".avi": "video/x-msvideo", ".mkv": "video/x-matroska", ".m4v": "video/x-m4v", ".3gp": "video/3gpp",
	}
	expectedContentType, extOK := allowedExt[ext]
	info, typeOK := types[contentType]
	if !extOK || !typeOK || expectedContentType != contentType {
		return "", "", 0, false
	}
	return info.ext, contentType, info.maxSize, true
}

func loadOSSConfig(ctx context.Context) ossConfig {
	cfg := ossConfig{
		Region:      g.Cfg().MustGet(ctx, "oss.region", defaultRegion).String(),
		Bucket:      g.Cfg().MustGet(ctx, "oss.bucket", defaultBucket).String(),
		KeyPrefix:   g.Cfg().MustGet(ctx, "oss.prefix", defaultKeyPrefix).String(),
		ResourceURL: g.Cfg().MustGet(ctx, "oss.resourceUrl", defaultResourceURL).String(),
		ExpiresIn:   g.Cfg().MustGet(ctx, "oss.presignExpires", defaultExpiresSecond).Int(),
	}
	if cfg.ExpiresIn <= 0 || cfg.ExpiresIn > 3600 {
		cfg.ExpiresIn = defaultExpiresSecond
	}
	cfg.AccessKeyID = g.Cfg().MustGet(ctx, "oss.accessKeyId", "").String()
	cfg.SecretAccessKey = g.Cfg().MustGet(ctx, "oss.secretAccessKey", "").String()
	cfg.SessionToken = g.Cfg().MustGet(ctx, "oss.sessionToken", "").String()
	return cfg
}

func loadAWSConfig(ctx context.Context, cfg ossConfig) (aws.Config, error) {
	options := []func(*config.LoadOptions) error{config.WithRegion(cfg.Region)}
	if cfg.AccessKeyID != "" && cfg.SecretAccessKey != "" {
		options = append(options, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(cfg.AccessKeyID, cfg.SecretAccessKey, cfg.SessionToken)))
	}
	return config.LoadDefaultConfig(ctx, options...)
}

func buildObjectKey(prefix string, userID int64, ext string) string {
	cleanPrefix := strings.Trim(strings.TrimSpace(prefix), "/")
	day := time.Now().Format("20060102")
	key := "uploads/" + strconvFormatInt(userID) + "/" + day + "/" + strings.ReplaceAll(uuid.NewString(), "-", "") + ext
	if cleanPrefix == "" {
		return key
	}
	return cleanPrefix + "/" + key
}

func buildFileURL(cfg ossConfig, key string) string {
	resourceURL := strings.TrimSpace(cfg.ResourceURL)
	if resourceURL != "" {
		prefix := strings.Trim(strings.TrimSpace(cfg.KeyPrefix), "/")
		path := key
		if prefix != "" {
			path = strings.TrimPrefix(key, prefix+"/")
		}
		return strings.TrimRight(resourceURL, "/") + "/" + path
	}
	return "https://" + cfg.Bucket + ".s3." + cfg.Region + ".amazonaws.com/" + key
}

func normalizeObjectKey(ctx context.Context, raw string, cfg ossConfig) (string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return "", ossErr(ctx, "invalid_object_key")
	}
	if strings.HasPrefix(value, "http://") || strings.HasPrefix(value, "https://") {
		parsed, err := url.Parse(value)
		if err != nil {
			return "", ossErr(ctx, "invalid_object_key")
		}
		value = strings.TrimPrefix(parsed.Path, "/")
	}
	value = strings.TrimPrefix(value, "/")
	prefix := strings.Trim(strings.TrimSpace(cfg.KeyPrefix), "/")
	if prefix != "" && !strings.HasPrefix(value, prefix+"/") {
		value = prefix + "/" + strings.TrimPrefix(value, prefix+"/")
	}
	if strings.Contains(value, "..") || strings.Contains(value, "\\") || value == prefix || !strings.HasPrefix(value, prefix+"/") {
		return "", ossErr(ctx, "invalid_object_key")
	}
	return value, nil
}

func ossErr(ctx context.Context, key string) error {
	msg := consts.LocalizedText(ossErrMsgs[key], consts.LocaleFromCtx(ctx))
	if msg == "" {
		msg = key
	}
	return gerror.NewCode(gcode.New(400, "", nil), msg)
}

func strconvFormatInt(v int64) string {
	if v == 0 {
		return "0"
	}
	buf := [20]byte{}
	i := len(buf)
	for v > 0 {
		i--
		buf[i] = byte('0' + v%10)
		v /= 10
	}
	return string(buf[i:])
}
