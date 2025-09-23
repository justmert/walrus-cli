package backend

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

type S3Client struct {
	client *s3.Client
	region string
}

type S3Credentials struct {
	AccessKeyID     string
	SecretAccessKey string
	SessionToken    string
	Region          string
}

type S3Object struct {
	Key          string
	Size         int64
	LastModified time.Time
	ETag         string
	StorageClass types.StorageClass
}

type S3TransferFilter struct {
	Prefix       string
	Include      []string
	Exclude      []string
	MinSize      int64
	MaxSize      int64
	ModifiedAfter  *time.Time
	ModifiedBefore *time.Time
}

func NewS3Client(creds S3Credentials) (*S3Client, error) {
	region := creds.Region
	if region == "" {
		region = "us-east-1"
	}

	cfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(region),
		config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(
			creds.AccessKeyID,
			creds.SecretAccessKey,
			creds.SessionToken,
		)),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &S3Client{
		client: s3.NewFromConfig(cfg),
		region: region,
	}, nil
}

func (c *S3Client) ListBuckets(ctx context.Context) ([]string, error) {
	result, err := c.client.ListBuckets(ctx, &s3.ListBucketsInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to list buckets: %w", err)
	}

	buckets := make([]string, 0, len(result.Buckets))
	for _, bucket := range result.Buckets {
		if bucket.Name != nil {
			buckets = append(buckets, *bucket.Name)
		}
	}

	return buckets, nil
}

func (c *S3Client) ListObjects(ctx context.Context, bucket string, filter *S3TransferFilter) ([]S3Object, error) {
	objects := []S3Object{}
	paginator := s3.NewListObjectsV2Paginator(c.client, &s3.ListObjectsV2Input{
		Bucket: aws.String(bucket),
		Prefix: aws.String(filter.Prefix),
	})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to list objects: %w", err)
		}

		for _, obj := range page.Contents {
			if obj.Key == nil {
				continue
			}

			var size int64
			if obj.Size != nil {
				size = *obj.Size
			}

			s3Obj := S3Object{
				Key:          *obj.Key,
				Size:         size,
				LastModified: *obj.LastModified,
				StorageClass: types.StorageClass(obj.StorageClass),
			}

			if obj.ETag != nil {
				s3Obj.ETag = *obj.ETag
			}

			if c.shouldIncludeObject(s3Obj, filter) {
				objects = append(objects, s3Obj)
			}
		}
	}

	return objects, nil
}

func (c *S3Client) shouldIncludeObject(obj S3Object, filter *S3TransferFilter) bool {
	if filter == nil {
		return true
	}

	if filter.MinSize > 0 && obj.Size < filter.MinSize {
		return false
	}

	if filter.MaxSize > 0 && obj.Size > filter.MaxSize {
		return false
	}

	if filter.ModifiedAfter != nil && obj.LastModified.Before(*filter.ModifiedAfter) {
		return false
	}

	if filter.ModifiedBefore != nil && obj.LastModified.After(*filter.ModifiedBefore) {
		return false
	}

	for _, exclude := range filter.Exclude {
		if matched := matchPattern(obj.Key, exclude); matched {
			return false
		}
	}

	if len(filter.Include) > 0 {
		included := false
		for _, include := range filter.Include {
			if matched := matchPattern(obj.Key, include); matched {
				included = true
				break
			}
		}
		if !included {
			return false
		}
	}

	return true
}

func matchPattern(text, pattern string) bool {
	if strings.Contains(pattern, "*") {
		parts := strings.Split(pattern, "*")
		if len(parts) == 1 {
			return text == pattern
		}

		if len(parts) == 2 {
			prefix := parts[0]
			suffix := parts[1]

			if prefix != "" && !strings.HasPrefix(text, prefix) {
				return false
			}
			if suffix != "" && !strings.HasSuffix(text, suffix) {
				return false
			}
			return true
		}

		currentIndex := 0
		for i, part := range parts {
			if part == "" {
				continue
			}

			index := strings.Index(text[currentIndex:], part)
			if index == -1 {
				return false
			}

			if i == 0 && part != "" && index != 0 {
				return false
			}

			currentIndex += index + len(part)
		}

		if lastPart := parts[len(parts)-1]; lastPart != "" {
			if !strings.HasSuffix(text, lastPart) {
				return false
			}
		}

		return true
	}

	return text == pattern
}

func (c *S3Client) DownloadObject(ctx context.Context, bucket, key string) (io.ReadCloser, int64, error) {
	headResult, err := c.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get object metadata: %w", err)
	}

	result, err := c.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, 0, fmt.Errorf("failed to download object: %w", err)
	}

	var contentLength int64
	if headResult.ContentLength != nil {
		contentLength = *headResult.ContentLength
	}
	return result.Body, contentLength, nil
}

func (c *S3Client) DownloadObjectToWriter(ctx context.Context, bucket, key string, w io.Writer) error {
	reader, _, err := c.DownloadObject(ctx, bucket, key)
	if err != nil {
		return err
	}
	defer reader.Close()

	_, err = io.Copy(w, reader)
	if err != nil {
		return fmt.Errorf("failed to write object data: %w", err)
	}

	return nil
}

func (c *S3Client) GetObjectMetadata(ctx context.Context, bucket, key string) (*S3Object, error) {
	result, err := c.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get object metadata: %w", err)
	}

	var contentLength int64
	if result.ContentLength != nil {
		contentLength = *result.ContentLength
	}

	obj := &S3Object{
		Key:          key,
		Size:         contentLength,
		LastModified: *result.LastModified,
		StorageClass: types.StorageClass(result.StorageClass),
	}

	if result.ETag != nil {
		obj.ETag = *result.ETag
	}

	return obj, nil
}

func (c *S3Client) EstimateTransferSize(ctx context.Context, bucket string, filter *S3TransferFilter) (int64, int, error) {
	objects, err := c.ListObjects(ctx, bucket, filter)
	if err != nil {
		return 0, 0, err
	}

	var totalSize int64
	for _, obj := range objects {
		totalSize += obj.Size
	}

	return totalSize, len(objects), nil
}