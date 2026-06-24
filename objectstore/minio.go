package objectstore

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"time"

	"github.com/actionlab-ai/aisphere-kit/metrics"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type MinIOConfig struct {
	Provider         string `json:"provider" yaml:"provider"`
	Endpoint         string `json:"endpoint" yaml:"endpoint"`
	AccessKey        string `json:"access_key" yaml:"access_key"`
	SecretKey        string `json:"secret_key" yaml:"secret_key"`
	SessionToken     string `json:"session_token" yaml:"session_token"`
	Bucket           string `json:"bucket" yaml:"bucket"`
	Region           string `json:"region" yaml:"region"`
	UseSSL           bool   `json:"use_ssl" yaml:"use_ssl"`
	AutoCreateBucket bool   `json:"auto_create_bucket" yaml:"auto_create_bucket"`
}

type MinIOClient struct {
	cfg     MinIOConfig
	client  *minio.Client
	logger  *slog.Logger
	metrics *metrics.Metrics
}

type Option func(*options)
type options struct {
	logger  *slog.Logger
	metrics *metrics.Metrics
}

func WithLogger(l *slog.Logger) Option      { return func(o *options) { o.logger = l } }
func WithMetrics(m *metrics.Metrics) Option { return func(o *options) { o.metrics = m } }

func NewMinIO(ctx context.Context, cfg MinIOConfig, opts ...Option) (*MinIOClient, error) {
	var opt options
	for _, fn := range opts {
		if fn != nil {
			fn(&opt)
		}
	}
	l := opt.logger
	if l == nil {
		l = slog.Default()
	}
	l = l.With("component", "objectstore", "provider", firstNonEmpty(cfg.Provider, "minio"), "bucket", cfg.Bucket)
	started := time.Now()
	l.Info("minio init started", "endpoint", cfg.Endpoint, "use_ssl", cfg.UseSSL)
	if cfg.Provider != "" && cfg.Provider != "minio" {
		err := fmt.Errorf("unsupported objectstore provider %q", cfg.Provider)
		l.Error("objectstore config invalid", "error", err)
		return nil, err
	}
	if cfg.Endpoint == "" || cfg.AccessKey == "" || cfg.SecretKey == "" || cfg.Bucket == "" {
		err := fmt.Errorf("endpoint/access_key/secret_key/bucket are required")
		l.Error("objectstore config invalid", "error", err)
		return nil, err
	}
	cli, err := minio.New(cfg.Endpoint, &minio.Options{Creds: credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, cfg.SessionToken), Secure: cfg.UseSSL, Region: cfg.Region})
	if err != nil {
		if opt.metrics != nil {
			opt.metrics.ObserveDependency("minio", "new", started, err)
		}
		l.Error("minio client create failed", "error", err)
		return nil, fmt.Errorf("new minio: %w", err)
	}
	m := &MinIOClient{cfg: cfg, client: cli, logger: l, metrics: opt.metrics}
	exists, err := m.BucketExists(ctx)
	if err != nil {
		if opt.metrics != nil {
			opt.metrics.ObserveDependency("minio", "bucket_exists", started, err)
		}
		l.Error("minio bucket check failed", "error", err)
		return nil, err
	}
	if !exists && cfg.AutoCreateBucket {
		l.Warn("minio bucket missing; creating because auto_create_bucket=true", "bucket", cfg.Bucket)
		if err := m.EnsureBucket(ctx); err != nil {
			l.Error("minio bucket create failed", "error", err)
			return nil, err
		}
	} else if !exists {
		l.Warn("minio bucket does not exist", "bucket", cfg.Bucket)
	}
	if opt.metrics != nil {
		opt.metrics.ObserveDependency("minio", "init", started, nil)
	}
	l.Info("minio init completed", "elapsed", time.Since(started).String())
	return m, nil
}

func (m *MinIOClient) Raw() *minio.Client { return m.client }
func (m *MinIOClient) Bucket() string     { return m.cfg.Bucket }

func (m *MinIOClient) BucketExists(ctx context.Context) (bool, error) {
	started := time.Now()
	exists, err := m.client.BucketExists(ctx, m.cfg.Bucket)
	if m.metrics != nil {
		m.metrics.ObserveDependency("minio", "bucket_exists", started, err)
	}
	if err != nil {
		return false, fmt.Errorf("check bucket %s: %w", m.cfg.Bucket, err)
	}
	if m.logger != nil {
		m.logger.Debug("minio bucket exists checked", "exists", exists, "elapsed", time.Since(started).String())
	}
	return exists, nil
}

func (m *MinIOClient) EnsureBucket(ctx context.Context) error {
	exists, err := m.BucketExists(ctx)
	if err != nil {
		return err
	}
	if exists {
		return nil
	}
	started := time.Now()
	if err := m.client.MakeBucket(ctx, m.cfg.Bucket, minio.MakeBucketOptions{Region: m.cfg.Region}); err != nil {
		if m.metrics != nil {
			m.metrics.ObserveDependency("minio", "make_bucket", started, err)
		}
		return fmt.Errorf("make bucket %s: %w", m.cfg.Bucket, err)
	}
	if m.metrics != nil {
		m.metrics.ObserveDependency("minio", "make_bucket", started, nil)
	}
	if m.logger != nil {
		m.logger.Info("minio bucket created", "elapsed", time.Since(started).String())
	}
	return nil
}

func (m *MinIOClient) PutObject(ctx context.Context, key string, body io.Reader, size int64, opts PutOptions) (ObjectInfo, error) {
	started := time.Now()
	l := m.logger.With("operation", "put_object", "key", key, "size", size)
	l.Debug("minio put object started")
	po := minio.PutObjectOptions{ContentType: opts.ContentType, UserMetadata: opts.Metadata, CacheControl: opts.CacheControl}
	mi, err := m.client.PutObject(ctx, m.cfg.Bucket, key, body, size, po)
	if m.metrics != nil {
		m.metrics.ObserveDependency("minio", "put_object", started, err)
	}
	if err != nil {
		l.Warn("minio put object failed", "error", err, "elapsed", time.Since(started).String())
		return ObjectInfo{}, err
	}
	l.Info("minio put object completed", "etag", mi.ETag, "elapsed", time.Since(started).String())
	return ObjectInfo{Bucket: mi.Bucket, Key: mi.Key, Size: mi.Size, ETag: mi.ETag, ContentType: opts.ContentType}, nil
}

func (m *MinIOClient) GetObject(ctx context.Context, key string, opts GetOptions) (io.ReadCloser, ObjectInfo, error) {
	started := time.Now()
	l := m.logger.With("operation", "get_object", "key", key)
	l.Debug("minio get object started", "offset", opts.Offset, "length", opts.Length)
	goOpts := minio.GetObjectOptions{}
	if opts.Length > 0 {
		if err := goOpts.SetRange(opts.Offset, opts.Offset+opts.Length-1); err != nil {
			return nil, ObjectInfo{}, err
		}
	} else if opts.Offset > 0 {
		if err := goOpts.SetRange(opts.Offset, 0); err != nil {
			return nil, ObjectInfo{}, err
		}
	}
	obj, err := m.client.GetObject(ctx, m.cfg.Bucket, key, goOpts)
	if err != nil {
		if m.metrics != nil {
			m.metrics.ObserveDependency("minio", "get_object", started, err)
		}
		l.Warn("minio get object failed", "error", err)
		return nil, ObjectInfo{}, err
	}
	st, err := obj.Stat()
	if m.metrics != nil {
		m.metrics.ObserveDependency("minio", "get_object", started, err)
	}
	if err != nil {
		_ = obj.Close()
		l.Warn("minio get object stat failed", "error", err)
		return nil, ObjectInfo{}, err
	}
	l.Debug("minio get object completed", "size", st.Size, "elapsed", time.Since(started).String())
	return obj, ObjectInfo{Bucket: m.cfg.Bucket, Key: st.Key, Size: st.Size, ETag: st.ETag, ContentType: st.ContentType, LastModified: st.LastModified}, nil
}

func (m *MinIOClient) DeleteObject(ctx context.Context, key string) error {
	started := time.Now()
	err := m.client.RemoveObject(ctx, m.cfg.Bucket, key, minio.RemoveObjectOptions{})
	if m.metrics != nil {
		m.metrics.ObserveDependency("minio", "delete_object", started, err)
	}
	if err != nil {
		m.logger.Warn("minio delete object failed", "key", key, "error", err)
	} else {
		m.logger.Info("minio delete object completed", "key", key, "elapsed", time.Since(started).String())
	}
	return err
}
func (m *MinIOClient) StatObject(ctx context.Context, key string) (ObjectInfo, error) {
	started := time.Now()
	st, err := m.client.StatObject(ctx, m.cfg.Bucket, key, minio.StatObjectOptions{})
	if m.metrics != nil {
		m.metrics.ObserveDependency("minio", "stat_object", started, err)
	}
	if err != nil {
		return ObjectInfo{}, err
	}
	return ObjectInfo{Bucket: m.cfg.Bucket, Key: st.Key, Size: st.Size, ETag: st.ETag, ContentType: st.ContentType, LastModified: st.LastModified}, nil
}
func (m *MinIOClient) ListObjects(ctx context.Context, opts ListOptions) ([]ObjectInfo, error) {
	started := time.Now()
	ch := m.client.ListObjects(ctx, m.cfg.Bucket, minio.ListObjectsOptions{Prefix: opts.Prefix, Recursive: opts.Recursive})
	items := make([]ObjectInfo, 0)
	var err error
	for obj := range ch {
		if obj.Err != nil {
			err = obj.Err
			break
		}
		items = append(items, ObjectInfo{Bucket: m.cfg.Bucket, Key: obj.Key, Size: obj.Size, ETag: obj.ETag, LastModified: obj.LastModified, ContentType: obj.ContentType})
		if opts.Limit > 0 && len(items) >= opts.Limit {
			break
		}
	}
	if m.metrics != nil {
		m.metrics.ObserveDependency("minio", "list_objects", started, err)
	}
	if err != nil {
		m.logger.Warn("minio list objects failed", "prefix", opts.Prefix, "error", err)
		return nil, err
	}
	m.logger.Debug("minio list objects completed", "prefix", opts.Prefix, "count", len(items))
	return items, nil
}
func (m *MinIOClient) CopyObject(ctx context.Context, srcKey, dstKey string, opts PutOptions) (ObjectInfo, error) {
	started := time.Now()
	dst := minio.CopyDestOptions{Bucket: m.cfg.Bucket, Object: dstKey, UserMetadata: opts.Metadata}
	src := minio.CopySrcOptions{Bucket: m.cfg.Bucket, Object: srcKey}
	info, err := m.client.CopyObject(ctx, dst, src)
	if m.metrics != nil {
		m.metrics.ObserveDependency("minio", "copy_object", started, err)
	}
	if err != nil {
		m.logger.Warn("minio copy object failed", "src", srcKey, "dst", dstKey, "error", err)
		return ObjectInfo{}, err
	}
	m.logger.Info("minio copy object completed", "src", srcKey, "dst", dstKey, "elapsed", time.Since(started).String())
	return ObjectInfo{Bucket: m.cfg.Bucket, Key: info.Key, Size: info.Size, ETag: info.ETag}, nil
}
func (m *MinIOClient) PresignPut(ctx context.Context, key string, ttl time.Duration) (*url.URL, error) {
	started := time.Now()
	u, err := m.client.PresignedPutObject(ctx, m.cfg.Bucket, key, ttl)
	if m.metrics != nil {
		m.metrics.ObserveDependency("minio", "presign_put", started, err)
	}
	if err != nil {
		m.logger.Warn("minio presign put failed", "key", key, "error", err)
	} else {
		m.logger.Debug("minio presign put completed", "key", key, "ttl", ttl.String())
	}
	return u, err
}
func (m *MinIOClient) PresignGet(ctx context.Context, key string, ttl time.Duration) (*url.URL, error) {
	started := time.Now()
	u, err := m.client.PresignedGetObject(ctx, m.cfg.Bucket, key, ttl, nil)
	if m.metrics != nil {
		m.metrics.ObserveDependency("minio", "presign_get", started, err)
	}
	if err != nil {
		m.logger.Warn("minio presign get failed", "key", key, "error", err)
	} else {
		m.logger.Debug("minio presign get completed", "key", key, "ttl", ttl.String())
	}
	return u, err
}
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
