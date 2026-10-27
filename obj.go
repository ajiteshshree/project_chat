package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

type ObjectStore struct {
	client *minio.Client
	bucket string
}

func connectMinIO(ctx context.Context, cfg minioCfg) (*ObjectStore, error) {
	client, err := minio.New(cfg.Endpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.AccessKey, cfg.SecretKey, ""),
		Secure: cfg.UseSSL,
	})
	if err != nil {
		return nil, err
	}
	os := &ObjectStore{client: client, bucket: cfg.Bucket}
	for i := 0; i < 10; i++ {
		exists, err := client.BucketExists(ctx, cfg.Bucket)
		if err == nil {
			if !exists {
				if err := client.MakeBucket(ctx, cfg.Bucket, minio.MakeBucketOptions{}); err != nil {
					return nil, err
				}
			}
			return os, nil
		}
		fmt.Println("waiting for minio:", err)
		time.Sleep(3 * time.Second)
	}
	return nil, fmt.Errorf("minio not reachable")
}

type ObjectInfo struct {
	Key         string
	Size        int64
	ContentType string
}

func (o *ObjectStore) Put(ctx context.Context, key string, r io.Reader, size int64, contentType string) error {
	_, err := o.client.PutObject(ctx, o.bucket, key, r, size, minio.PutObjectOptions{
		ContentType: contentType,
	})
	return err
}

func (o *ObjectStore) Stat(ctx context.Context, key string) (ObjectInfo, error) {
	info, err := o.client.StatObject(ctx, o.bucket, key, minio.StatObjectOptions{})
	if err != nil {
		return ObjectInfo{}, err
	}
	return ObjectInfo{Key: key, Size: info.Size, ContentType: info.ContentType}, nil
}

func (o *ObjectStore) OpenRange(ctx context.Context, key string, start, end int64) (io.ReadCloser, error) {
	opts := minio.GetObjectOptions{}
	if start >= 0 && end >= start {
		if err := opts.SetRange(start, end); err != nil {
			return nil, err
		}
	}
	return o.client.GetObject(ctx, o.bucket, key, opts)
}