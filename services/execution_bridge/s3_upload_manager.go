package executionbridge

import (
	"compress/gzip"
	"context"
	"fmt"
	"io"
	"os"
	// "path/filepath" // removed unused import
	"time"

	"agent/logger"
	"agent/models/uploadvideo"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

/*
nkk: NEW IMPLEMENTATION
Notes by nkk:
- Implements S3UploadManager for streaming video uploads directly to S3.
- Compresses video on the fly using gzip to reduce storage and bandwidth.
- Uses AWS SDK's multipart uploader for efficient large file handling.
- Aligns with architecture plan for optimized session recording pipeline.
*/
type S3UploadManager struct {
	uploader *s3manager.Uploader
	bucket   string
}

func NewS3UploadManager() *S3UploadManager {
	sess := session.Must(session.NewSession(&aws.Config{
		Region: aws.String("us-east-1"),
	}))
	return &S3UploadManager{
		uploader: s3manager.NewUploader(sess),
		bucket:   "agent-session-recordings",
	}
}

func (m *S3UploadManager) StreamToS3(ctx context.Context, filePath string, metadata uploadvideo.UploadVideo) error {
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	pr, pw := io.Pipe()
	gzipWriter := gzip.NewWriter(pw)

	go func() {
		defer pw.Close()
		defer gzipWriter.Close()
		io.Copy(gzipWriter, file)
	}()

	key := fmt.Sprintf("recordings/%s/%s/%s.webm.gz",
		metadata.OrgId,
		time.Now().Format("2006-01-02"),
		metadata.ExecutionId)

	_, err = m.uploader.UploadWithContext(ctx, &s3manager.UploadInput{
		Bucket:          aws.String(m.bucket),
		Key:             aws.String(key),
		Body:            pr,
		ContentType:     aws.String("video/webm"),
		ContentEncoding: aws.String("gzip"),
		Metadata: map[string]*string{
			"execution-id": aws.String(metadata.ExecutionId),
			"testcase-id":  aws.String(metadata.TestcaseId),
		},
	})
	if err != nil {
		logger.Error("S3 upload failed", err)
		return err
	}

	logger.Info("S3 upload successful", nil)
	return nil
}
