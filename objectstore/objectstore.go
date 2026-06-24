package objectstore

import (
	"context"
	"io"
	"net/url"
	"time"
)

type ObjectInfo struct {
	Bucket       string
	Key          string
	Size         int64
	ETag         string
	ContentType  string
	LastModified time.Time
}

type PutOptions struct {
	ContentType  string
	Metadata     map[string]string
	CacheControl string
	SSE          any
}

type GetOptions struct {
	Offset int64
	Length int64
}

type ListOptions struct {
	Prefix    string
	Recursive bool
	Limit     int
}

type Client interface {
	Bucket() string
	BucketExists(ctx context.Context) (bool, error)
	EnsureBucket(ctx context.Context) error
	PutObject(ctx context.Context, key string, body io.Reader, size int64, opts PutOptions) (ObjectInfo, error)
	GetObject(ctx context.Context, key string, opts GetOptions) (io.ReadCloser, ObjectInfo, error)
	DeleteObject(ctx context.Context, key string) error
	StatObject(ctx context.Context, key string) (ObjectInfo, error)
	ListObjects(ctx context.Context, opts ListOptions) ([]ObjectInfo, error)
	CopyObject(ctx context.Context, srcKey, dstKey string, opts PutOptions) (ObjectInfo, error)
	PresignPut(ctx context.Context, key string, ttl time.Duration) (*url.URL, error)
	PresignGet(ctx context.Context, key string, ttl time.Duration) (*url.URL, error)
}
