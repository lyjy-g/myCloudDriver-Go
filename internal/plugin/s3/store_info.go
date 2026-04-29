package s3

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"myclouddrive-go/internal/framework/code"
	"myclouddrive-go/internal/plugin"
	"path"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	awss3 "github.com/aws/aws-sdk-go-v2/service/s3"
	awss3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/aws/smithy-go"
)

// Config 是 S3 兼容存储配置。
type Config struct {
	// Endpoint 示例：127.0.0.1:9000 或 s3.amazonaws.com。
	Endpoint string `json:"endpoint"`
	// Region 示例：us-east-1。
	Region string `json:"region"`
	// Bucket 为对象桶名称。
	Bucket string `json:"bucket"`
	// AccessKeyID 为访问密钥。
	AccessKeyID string `json:"access_key_id"`
	// SecretAccessKey 为访问密钥 Secret。
	SecretAccessKey string `json:"secret_access_key"`
	// SessionToken 为临时凭证 token，可选。
	SessionToken string `json:"session_token,omitempty"`
	// UseSSL 表示是否使用 HTTPS。
	UseSSL bool `json:"use_ssl"`
	// PathStyle 表示是否强制 path-style 访问（MinIO 推荐 true）。
	PathStyle bool `json:"path_style"`
	// Prefix 为对象前缀，可选。
	Prefix string `json:"prefix,omitempty"`
}

// StoreInfo 是 S3 兼容存储平台信息实现。
type StoreInfo struct{}

// NewStoreInfo 创建 S3 存储平台信息实例。
func NewStoreInfo() *StoreInfo {
	return &StoreInfo{}
}

// PlatformIdentifier 返回平台标识。
func (s *StoreInfo) PlatformIdentifier() plugin.PlatformIdentifier {
	return plugin.PlatformIdentifierS3
}

// Capabilities 返回能力集合。
func (s *StoreInfo) Capabilities() []plugin.Capability {
	return []plugin.Capability{plugin.CapabilityBasic, plugin.CapabilitySignedURL}
}

// ValidateConfig 校验配置是否合法。
func (s *StoreInfo) ValidateConfig(_ context.Context, raw []byte) error {
	cfg, err := parseConfig(raw)
	if err != nil {
		return err
	}
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return fmt.Errorf("s3 config.endpoint is required")
	}
	if strings.TrimSpace(cfg.Bucket) == "" {
		return fmt.Errorf("s3 config.bucket is required")
	}
	if strings.TrimSpace(cfg.AccessKeyID) == "" || strings.TrimSpace(cfg.SecretAccessKey) == "" {
		return fmt.Errorf("s3 access key is required")
	}
	return nil
}

// Build 基于配置构建 Store。
func (s *StoreInfo) Build(ctx context.Context, cfg plugin.ResolvedStorageConfig) (plugin.StorePower, error) {
	parsed, err := parseConfig(cfg.ConfigData)
	if err != nil {
		return nil, err
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx,
		awsconfig.WithRegion(parsed.Region),
		awsconfig.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(parsed.AccessKeyID, parsed.SecretAccessKey, parsed.SessionToken)),
	)
	if err != nil {
		return nil, fmt.Errorf("load aws config failed: %w", err)
	}

	client := awss3.NewFromConfig(awsCfg, func(options *awss3.Options) {
		options.UsePathStyle = parsed.PathStyle
		options.BaseEndpoint = aws.String(normalizeEndpoint(parsed.Endpoint, parsed.UseSSL))
	})

	bucket := parsed.Bucket
	_, err = client.HeadBucket(ctx, &awss3.HeadBucketInput{Bucket: aws.String(bucket)})
	if err != nil {
		if isNotFoundError(err) {
			if _, mkErr := client.CreateBucket(ctx, &awss3.CreateBucketInput{Bucket: aws.String(bucket)}); mkErr != nil {
				return nil, fmt.Errorf("create bucket failed: %w", mkErr)
			}
		} else {
			return nil, fmt.Errorf("check bucket failed: %w", err)
		}
	}

	return &store{
		client:  client,
		presign: awss3.NewPresignClient(client),
		bucket:  bucket,
		prefix:  strings.Trim(parsed.Prefix, "/"),
	}, nil
}

// store 是 S3 兼容对象存储实现。
type store struct {
	client  *awss3.Client
	presign *awss3.PresignClient
	bucket  string
	prefix  string
}

func (s *store) Put(ctx context.Context, key plugin.Key, r io.Reader, opts plugin.PutOptions) (plugin.ObjectInfo, error) {
	objectName, normalized, err := s.objectName(key)
	if err != nil {
		return plugin.ObjectInfo{}, err
	}

	input := &awss3.PutObjectInput{
		Bucket:   aws.String(s.bucket),
		Key:      aws.String(objectName),
		Body:     r,
		Metadata: opts.Metadata,
	}
	if opts.ContentType != "" {
		input.ContentType = aws.String(opts.ContentType)
	}
	if opts.ContentLength != nil {
		input.ContentLength = opts.ContentLength
	}

	out, err := s.client.PutObject(ctx, input)
	if err != nil {
		return plugin.ObjectInfo{}, fmt.Errorf("put s3 object failed: %w", err)
	}

	etag := ""
	if out.ETag != nil {
		etag = strings.Trim(*out.ETag, `"`)
	}

	return plugin.ObjectInfo{
		Key:         plugin.Key(normalized),
		Size:        valueOr(opts.ContentLength, 0),
		ContentType: opts.ContentType,
		ETag:        etag,
	}, nil
}

func (s *store) Get(ctx context.Context, key plugin.Key) (io.ReadCloser, plugin.ObjectInfo, error) {
	objectName, normalized, err := s.objectName(key)
	if err != nil {
		return nil, plugin.ObjectInfo{}, err
	}

	out, err := s.client.GetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectName),
	})
	if err != nil {
		return nil, plugin.ObjectInfo{}, s.wrapNotFound(err, normalized)
	}

	info := plugin.ObjectInfo{
		Key:         plugin.Key(normalized),
		Size:        aws.ToInt64(out.ContentLength),
		ContentType: aws.ToString(out.ContentType),
		ETag:        strings.Trim(aws.ToString(out.ETag), `"`),
	}
	if out.LastModified != nil {
		t := *out.LastModified
		info.LastModified = &t
	}

	return out.Body, info, nil
}

func (s *store) Delete(ctx context.Context, key plugin.Key) error {
	objectName, normalized, err := s.objectName(key)
	if err != nil {
		return err
	}

	if _, err = s.Stat(ctx, key); err != nil {
		return err
	}

	_, err = s.client.DeleteObject(ctx, &awss3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectName),
	})
	if err != nil {
		return s.wrapNotFound(err, normalized)
	}
	return nil
}

func (s *store) Stat(ctx context.Context, key plugin.Key) (plugin.ObjectInfo, error) {
	objectName, normalized, err := s.objectName(key)
	if err != nil {
		return plugin.ObjectInfo{}, err
	}

	out, err := s.client.HeadObject(ctx, &awss3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectName),
	})
	if err != nil {
		return plugin.ObjectInfo{}, s.wrapNotFound(err, normalized)
	}

	info := plugin.ObjectInfo{
		Key:         plugin.Key(normalized),
		Size:        aws.ToInt64(out.ContentLength),
		ContentType: aws.ToString(out.ContentType),
		ETag:        strings.Trim(aws.ToString(out.ETag), `"`),
	}
	if out.LastModified != nil {
		t := *out.LastModified
		info.LastModified = &t
	}
	return info, nil
}

// PresignGet 生成下载预签名 URL。
func (s *store) PresignGet(ctx context.Context, key plugin.Key, expire time.Duration) (string, error) {
	objectName, _, err := s.objectName(key)
	if err != nil {
		return "", err
	}

	out, err := s.presign.PresignGetObject(ctx, &awss3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectName),
	}, func(options *awss3.PresignOptions) {
		options.Expires = expire
	})
	if err != nil {
		return "", fmt.Errorf("presign get object failed: %w", err)
	}
	return out.URL, nil
}

// PresignPut 生成上传预签名 URL。
func (s *store) PresignPut(ctx context.Context, key plugin.Key, expire time.Duration) (string, error) {
	objectName, _, err := s.objectName(key)
	if err != nil {
		return "", err
	}

	out, err := s.presign.PresignPutObject(ctx, &awss3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(objectName),
	}, func(options *awss3.PresignOptions) {
		options.Expires = expire
	})
	if err != nil {
		return "", fmt.Errorf("presign put object failed: %w", err)
	}
	return out.URL, nil
}

func (s *store) objectName(key plugin.Key) (string, string, error) {
	raw := strings.TrimSpace(string(key))
	if raw == "" {
		return "", "", code.ErrInvalidKey
	}
	normalized := strings.Trim(path.Clean("/"+raw), "/")
	if normalized == "" || normalized == "." {
		return "", "", code.ErrInvalidKey
	}
	if s.prefix == "" {
		return normalized, normalized, nil
	}
	return s.prefix + "/" + normalized, normalized, nil
}

func (s *store) wrapNotFound(err error, key string) error {
	if err == nil {
		return nil
	}
	if isNotFoundError(err) {
		return fmt.Errorf("%w: %s", code.ErrObjectNotFound, key)
	}
	return err
}

func isNotFoundError(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey", "NoSuchBucket", "404":
			return true
		}
	}

	var notFound *awss3types.NotFound
	return errors.As(err, &notFound)
}

func parseConfig(raw []byte) (Config, error) {
	var cfg Config
	if len(raw) == 0 {
		return cfg, fmt.Errorf("s3 storage config is empty")
	}
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return cfg, fmt.Errorf("parse s3 config failed: %w", err)
	}
	cfg.Endpoint = strings.TrimSpace(cfg.Endpoint)
	cfg.Bucket = strings.TrimSpace(cfg.Bucket)
	cfg.Region = strings.TrimSpace(cfg.Region)
	if cfg.Region == "" {
		cfg.Region = "us-east-1"
	}
	return cfg, nil
}

func normalizeEndpoint(endpoint string, useSSL bool) string {
	endpoint = strings.TrimSpace(endpoint)
	if strings.HasPrefix(endpoint, "http://") || strings.HasPrefix(endpoint, "https://") {
		return endpoint
	}
	if useSSL {
		return "https://" + endpoint
	}
	return "http://" + endpoint
}

func valueOr[T any](v *T, def T) T {
	if v == nil {
		return def
	}
	return *v
}
