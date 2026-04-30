package storage

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/minio/minio-go/v7"
)

type fakeAdmin struct {
	exists    bool
	existsErr error
	makeErr   error
	made      bool
}

func (f *fakeAdmin) BucketExists(context.Context, string) (bool, error) {
	return f.exists, f.existsErr
}

func (f *fakeAdmin) MakeBucket(context.Context, string, minio.MakeBucketOptions) error {
	f.made = true
	return f.makeErr
}

type fakePresign struct {
	result *url.URL
	err    error
}

func (f *fakePresign) PresignHeader(context.Context, string, string, string, time.Duration, url.Values, http.Header) (*url.URL, error) {
	return f.result, f.err
}

func TestParseMinIOEndpoint(t *testing.T) {
	t.Parallel()
	tests := []struct {
		raw      string
		wantHost string
		wantTLS  bool
		wantErr  bool
	}{
		{raw: "http://localhost:9000", wantHost: "localhost:9000", wantTLS: false},
		{raw: "https://minio.example.com", wantHost: "minio.example.com", wantTLS: true},
		{raw: "minio:9000", wantHost: "minio:9000", wantTLS: false},
		{raw: "", wantErr: true},
		{raw: "ftp://x", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			t.Parallel()
			host, tls, err := ParseMinIOEndpoint(tt.raw)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if host != tt.wantHost || tls != tt.wantTLS {
				t.Fatalf("got %q tls=%v, want %q tls=%v", host, tls, tt.wantHost, tt.wantTLS)
			}
		})
	}
}

func TestMinioPhotoStore_EnsureBucket_createsWhenMissing(t *testing.T) {
	t.Parallel()
	adm := &fakeAdmin{exists: false}
	st := NewMinioPhotoStoreWithDeps(adm, &fakePresign{}, "b", time.Minute)
	if err := st.EnsureBucket(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !adm.made {
		t.Fatal("expected MakeBucket")
	}
}

func TestMinioPhotoStore_EnsureBucket_idempotent(t *testing.T) {
	t.Parallel()
	adm := &fakeAdmin{exists: true}
	st := NewMinioPhotoStoreWithDeps(adm, &fakePresign{}, "b", time.Minute)
	if err := st.EnsureBucket(context.Background()); err != nil {
		t.Fatal(err)
	}
	if adm.made {
		t.Fatal("unexpected MakeBucket")
	}
}

func TestMinioPhotoStore_PresignedPut_usesPresignerHost(t *testing.T) {
	t.Parallel()
	u, err := url.Parse("http://localhost:9000")
	if err != nil {
		t.Fatal(err)
	}
	st := NewMinioPhotoStoreWithDeps(&fakeAdmin{exists: true}, &fakePresign{result: u}, "mybucket", time.Minute)
	gotURL, headers, expMs, err := st.PresignedPut(context.Background(), "01JTEST123", "image/jpeg")
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := url.Parse(gotURL)
	if err != nil {
		t.Fatal(err)
	}
	if parsed.Scheme != "http" || parsed.Host != "localhost:9000" {
		t.Fatalf("host/scheme: %s", gotURL)
	}
	if headers["Content-Type"] != "image/jpeg" {
		t.Fatalf("headers: %#v", headers)
	}
	if expMs <= time.Now().Add(time.Second).UnixMilli() {
		t.Fatalf("expiresAt too soon: %d", expMs)
	}
	if expMs <= 1e12 {
		t.Fatalf("expiresAt should be epoch-ms > 1e12, got %d", expMs)
	}
}

func TestMinioPhotoStore_PresignedPut_objectKeyContainsPhotoID(t *testing.T) {
	t.Parallel()
	var sawKey string
	p := &trackingPresign{fn: func(bucket, object string) {
		sawKey = object
	}}
	st := NewMinioPhotoStoreWithDeps(&fakeAdmin{exists: true}, p, "buck", time.Minute)
	u, _ := url.Parse("http://internal:9000/x")
	p.result = u
	_, _, _, err := st.PresignedPut(context.Background(), "pid-xyz", "image/png")
	if err != nil {
		t.Fatal(err)
	}
	if sawKey != "photos/pid-xyz" {
		t.Fatalf("object key %q", sawKey)
	}
}

type trackingPresign struct {
	result *url.URL
	fn     func(bucket, object string)
}

func (tr *trackingPresign) PresignHeader(_ context.Context, _ string, bucketName, objectName string, _ time.Duration, _ url.Values, _ http.Header) (*url.URL, error) {
	tr.fn(bucketName, objectName)
	return tr.result, nil
}

func TestNewMinioPhotoStore_invalidEndpoint(t *testing.T) {
	t.Parallel()
	if _, err := NewMinioPhotoStore("://bad", "", "k", "s", "b"); err == nil {
		t.Fatal("expected error")
	}
}

func TestMinioPhotoStore_EnsureBucket_propagatesAdminErr(t *testing.T) {
	t.Parallel()
	want := errors.New("boom")
	st := NewMinioPhotoStoreWithDeps(&fakeAdmin{existsErr: want}, &fakePresign{}, "b", time.Minute)
	err := st.EnsureBucket(context.Background())
	if err == nil || !errors.Is(err, want) {
		t.Fatalf("err %v", err)
	}
}
