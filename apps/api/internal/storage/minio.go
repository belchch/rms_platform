package storage

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

const defaultPresignedTTL = 15 * time.Minute

type bucketAdmin interface {
	BucketExists(ctx context.Context, bucketName string) (bool, error)
	MakeBucket(ctx context.Context, bucketName string, opts minio.MakeBucketOptions) error
}

type objectPresigner interface {
	PresignHeader(ctx context.Context, method, bucketName, objectName string, expires time.Duration, reqParams url.Values, extraHeaders http.Header) (*url.URL, error)
}

type MinioPhotoStore struct {
	admin      bucketAdmin
	presign    objectPresigner
	bucket     string
	presignTTL time.Duration
}

func ParseMinIOEndpoint(raw string) (host string, secure bool, err error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", false, fmt.Errorf("empty endpoint")
	}
	if !strings.Contains(raw, "://") {
		return raw, false, nil
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", false, fmt.Errorf("parse endpoint: %w", err)
	}
	if u.Host == "" {
		return "", false, fmt.Errorf("endpoint missing host")
	}
	pathOnly := strings.TrimSuffix(u.Path, "/")
	if pathOnly != "" {
		return "", false, fmt.Errorf("endpoint URL must not include a path (got %q)", u.Path)
	}
	switch strings.ToLower(u.Scheme) {
	case "https":
		return u.Host, true, nil
	case "http":
		return u.Host, false, nil
	default:
		return "", false, fmt.Errorf("unsupported scheme %q", u.Scheme)
	}
}

func NewMinioPhotoStore(endpoint, publicEndpoint, accessKey, secretKey, bucket string) (*MinioPhotoStore, error) {
	adminHost, adminSecure, err := ParseMinIOEndpoint(endpoint)
	if err != nil {
		return nil, fmt.Errorf("S3_ENDPOINT: %w", err)
	}

	pub := strings.TrimSpace(publicEndpoint)
	if pub == "" {
		pub = endpoint
	}
	presignHost, presignSecure, err := ParseMinIOEndpoint(pub)
	if err != nil {
		return nil, fmt.Errorf("S3_PUBLIC_ENDPOINT: %w", err)
	}

	cred := credentials.NewStaticV4(accessKey, secretKey, "")
	adminClient, err := minio.New(adminHost, &minio.Options{Creds: cred, Secure: adminSecure})
	if err != nil {
		return nil, fmt.Errorf("minio admin client: %w", err)
	}
	presignClient, err := minio.New(presignHost, &minio.Options{Creds: cred, Secure: presignSecure})
	if err != nil {
		return nil, fmt.Errorf("minio presign client: %w", err)
	}

	return &MinioPhotoStore{
		admin:      adminClient,
		presign:    presignClient,
		bucket:     bucket,
		presignTTL: defaultPresignedTTL,
	}, nil
}

func NewMinioPhotoStoreWithPresignTTL(endpoint, publicEndpoint, accessKey, secretKey, bucket string, presignTTL time.Duration) (*MinioPhotoStore, error) {
	store, err := NewMinioPhotoStore(endpoint, publicEndpoint, accessKey, secretKey, bucket)
	if err != nil {
		return nil, err
	}
	if presignTTL > 0 {
		store.presignTTL = presignTTL
	}
	return store, nil
}

func NewMinioPhotoStoreWithDeps(admin bucketAdmin, presign objectPresigner, bucket string, ttl time.Duration) *MinioPhotoStore {
	if ttl <= 0 {
		ttl = defaultPresignedTTL
	}
	return &MinioPhotoStore{
		admin:      admin,
		presign:    presign,
		bucket:     bucket,
		presignTTL: ttl,
	}
}

func (s *MinioPhotoStore) EnsureBucket(ctx context.Context) error {
	ok, err := s.admin.BucketExists(ctx, s.bucket)
	if err != nil {
		return fmt.Errorf("bucket exists: %w", err)
	}
	if ok {
		return nil
	}
	if err := s.admin.MakeBucket(ctx, s.bucket, minio.MakeBucketOptions{}); err != nil {
		e := minio.ToErrorResponse(err)
		if e.Code == "BucketAlreadyOwnedByYou" || e.Code == "BucketAlreadyExists" {
			return nil
		}
		return fmt.Errorf("make bucket: %w", err)
	}
	return nil
}

func (s *MinioPhotoStore) PresignedPut(ctx context.Context, photoID, contentType string) (string, map[string]string, int64, error) {
	objectKey := "photos/" + photoID
	h := http.Header{}
	h.Set("Content-Type", contentType)

	expiresAtMs := time.Now().Add(s.presignTTL).UnixMilli()
	u, err := s.presign.PresignHeader(ctx, http.MethodPut, s.bucket, objectKey, s.presignTTL, nil, h)
	if err != nil {
		return "", nil, 0, fmt.Errorf("presign put %s/%s: %w", s.bucket, objectKey, err)
	}

	headers := map[string]string{"Content-Type": contentType}
	return u.String(), headers, expiresAtMs, nil
}
