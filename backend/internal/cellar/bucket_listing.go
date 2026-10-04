package cellar

import (
	"context"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/benbjohnson/litestream"
	"github.com/google/uuid"
)

// listBucket lists the databases of the bucket, by id, with the time of the
// newest object of each. The bucket holds a folder dbs/{id}/ per database.
func (databases *Databases) listBucket(ctx context.Context) (map[string]time.Time, error) {
	if databases.bucket.Scheme == "file" {
		return listBucketDirectory(filepath.Join(databases.bucket.Path, "dbs"))
	}
	return listBucketS3(ctx, databases.bucket)
}

func listBucketDirectory(root string) (map[string]time.Time, error) {
	newest := map[string]time.Time{}
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return newest, nil
	}
	if err != nil {
		return nil, err
	}
	for _, entry := range entries {
		if _, err := uuid.Parse(entry.Name()); err != nil || !entry.IsDir() {
			continue
		}
		var latest time.Time
		err := filepath.WalkDir(filepath.Join(root, entry.Name()), func(_ string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			info, err := d.Info()
			if err == nil && info.ModTime().After(latest) {
				latest = info.ModTime()
			}
			return err
		})
		if err != nil {
			return nil, err
		}
		newest[entry.Name()] = latest
	}
	return newest, nil
}

// listBucketS3 pages through every object under dbs/. One listing for all the
// databases: a request per database would grow with the number of cold ones.
func listBucketS3(ctx context.Context, location url.URL) (map[string]time.Time, error) {
	options := []func(*config.LoadOptions) error{config.WithRegion(location.Query().Get("region"))}
	if bucketAccessKeyID != "" {
		options = append(options, config.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(bucketAccessKeyID, bucketSecretAccessKey, "")))
	}
	loaded, err := config.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return nil, err
	}
	endpoint, _ := litestream.EnsureEndpointScheme(location.Query().Get("endpoint"))
	client := s3.NewFromConfig(loaded, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
			o.UsePathStyle = true
		}
	})

	prefix := strings.Trim(location.Path, "/")
	if prefix != "" {
		prefix += "/"
	}
	prefix += "dbs/"
	newest := map[string]time.Time{}
	pages := s3.NewListObjectsV2Paginator(client, &s3.ListObjectsV2Input{Bucket: aws.String(location.Host), Prefix: aws.String(prefix)})
	for pages.HasMorePages() {
		page, err := pages.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("list bucket: %w", err)
		}
		for _, object := range page.Contents {
			id, _, _ := strings.Cut(strings.TrimPrefix(aws.ToString(object.Key), prefix), "/")
			if _, err := uuid.Parse(id); err != nil {
				continue
			}
			if modified := aws.ToTime(object.LastModified); modified.After(newest[id]) {
				newest[id] = modified
			}
		}
	}
	return newest, nil
}
