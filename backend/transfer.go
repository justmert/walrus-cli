package backend

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"path"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fatih/color"
	"github.com/schollz/progressbar/v3"
)

// EstimateWalrusCost estimates the cost in WAL for storing data
func EstimateWalrusCost(sizeBytes int64, epochs int) float64 {
	// Encoding overhead: 5x original + 64MB metadata
	encodedSizeBytes := sizeBytes*5 + 64*1024*1024
	// Convert to MB (round up)
	encodedSizeMB := (encodedSizeBytes + 1048575) / 1048576
	// Cost per MB per epoch in FROST (with 80% subsidy)
	costPerMBPerEpoch := int64(55000 / 5) // 11,000 FROST after subsidy
	// Total cost in FROST
	totalFROST := encodedSizeMB * costPerMBPerEpoch * int64(epochs)
	// Convert to WAL (1 WAL = 1,000,000,000 FROST)
	return float64(totalFROST) / 1_000_000_000.0
}

type TransferManager struct {
	s3Client      *S3Client
	walrusClient  *WalrusClient
	simpleFS      *SimpleFs
	concurrency   int
	dryRun        bool
	enableEncrypt bool
}

type TransferJob struct {
	Bucket       string
	Key          string
	Size         int64
	TargetName   string
	Epochs       int
	EncryptionConfig *EncryptionSettings
}

type EncryptionSettings struct {
	Enabled   bool
	Threshold int
	PolicyID  string
}

type TransferResult struct {
	SourceKey     string
	TargetName    string
	BlobID        string
	Size          int64
	Success       bool
	Error         error
	UploadTime    time.Time
	EstimatedCost float64
	ExpiryEpoch   *int64
	RegisteredEpoch *int64
	SuiObjectID   string
}

type TransferProgress struct {
	TotalFiles      int
	ProcessedFiles  int32
	TotalBytes      int64
	ProcessedBytes  int64
	FailedFiles     int32
	StartTime       time.Time
	Results         []TransferResult
	mu              sync.Mutex
}

func NewTransferManager(s3Client *S3Client, walrusClient *WalrusClient, simpleFS *SimpleFs, concurrency int) *TransferManager {
	if concurrency <= 0 {
		concurrency = 1
	}
	if concurrency > 10 {
		concurrency = 10
	}

	return &TransferManager{
		s3Client:     s3Client,
		walrusClient: walrusClient,
		simpleFS:     simpleFS,
		concurrency:  concurrency,
	}
}

func (tm *TransferManager) SetDryRun(dryRun bool) {
	tm.dryRun = dryRun
}

func (tm *TransferManager) SetEncryption(enable bool) {
	tm.enableEncrypt = enable
}

func (tm *TransferManager) EstimateTransferCost(ctx context.Context, bucket string, filter *S3TransferFilter, epochs int) (float64, int, error) {
	objects, err := tm.s3Client.ListObjects(ctx, bucket, filter)
	if err != nil {
		return 0, 0, err
	}

	var totalCost float64
	for _, obj := range objects {
		cost := EstimateWalrusCost(obj.Size, epochs)
		totalCost += cost
	}

	return totalCost, len(objects), nil
}

func (tm *TransferManager) TransferBatch(ctx context.Context, bucket string, filter *S3TransferFilter, epochs int, encryptionConfig *EncryptionSettings) (*TransferProgress, error) {
	objects, err := tm.s3Client.ListObjects(ctx, bucket, filter)
	if err != nil {
		return nil, fmt.Errorf("failed to list objects: %w", err)
	}

	if len(objects) == 0 {
		return &TransferProgress{
			TotalFiles: 0,
			StartTime:  time.Now(),
		}, nil
	}

	var totalSize int64
	jobs := make([]TransferJob, 0, len(objects))
	for _, obj := range objects {
		totalSize += obj.Size

		targetName := path.Base(obj.Key)
		if targetName == "" {
			targetName = obj.Key
		}

		jobs = append(jobs, TransferJob{
			Bucket:           bucket,
			Key:              obj.Key,
			Size:             obj.Size,
			TargetName:       targetName,
			Epochs:           epochs,
			EncryptionConfig: encryptionConfig,
		})
	}

	if tm.dryRun {
		fmt.Println(color.YellowString("\n=== DRY RUN MODE ==="))
		fmt.Printf("Would transfer %d files (%.2f MB total)\n", len(jobs), float64(totalSize)/(1024*1024))

		var totalCost float64
		for _, job := range jobs {
			cost := EstimateWalrusCost(job.Size, epochs)
			totalCost += cost
			fmt.Printf("  • %s (%.2f MB) → %.6f WAL\n",
				job.Key,
				float64(job.Size)/(1024*1024),
				cost)
		}

		fmt.Printf("\nTotal estimated cost: %.6f WAL\n", totalCost)
		fmt.Println(color.YellowString("=== DRY RUN COMPLETE ===\n"))

		return &TransferProgress{
			TotalFiles:     len(jobs),
			TotalBytes:     totalSize,
			ProcessedFiles: int32(len(jobs)),
			ProcessedBytes: totalSize,
			StartTime:      time.Now(),
		}, nil
	}

	progress := &TransferProgress{
		TotalFiles: len(jobs),
		TotalBytes: totalSize,
		StartTime:  time.Now(),
		Results:    make([]TransferResult, 0, len(jobs)),
	}

	bar := progressbar.NewOptions64(
		totalSize,
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(50),
		progressbar.OptionSetDescription("[cyan]Transferring files[reset]"),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
		progressbar.OptionOnCompletion(func() {
			fmt.Println()
		}),
	)

	jobChan := make(chan TransferJob, len(jobs))
	for _, job := range jobs {
		jobChan <- job
	}
	close(jobChan)

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, tm.concurrency)

	for i := 0; i < tm.concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for job := range jobChan {
				select {
				case <-ctx.Done():
					return
				case semaphore <- struct{}{}:
					result := tm.transferSingleFile(ctx, job, bar)

					atomic.AddInt32(&progress.ProcessedFiles, 1)
					if result.Success {
						atomic.AddInt64(&progress.ProcessedBytes, job.Size)
					} else {
						atomic.AddInt32(&progress.FailedFiles, 1)
					}

					progress.mu.Lock()
					progress.Results = append(progress.Results, result)
					progress.mu.Unlock()

					<-semaphore
				}
			}
		}()
	}

	wg.Wait()
	bar.Finish()

	return progress, nil
}

func (tm *TransferManager) transferSingleFile(ctx context.Context, job TransferJob, bar *progressbar.ProgressBar) TransferResult {
	result := TransferResult{
		SourceKey:  job.Key,
		TargetName: job.TargetName,
		Size:       job.Size,
		Success:    false,
		UploadTime: time.Now(),
		EstimatedCost: EstimateWalrusCost(job.Size, job.Epochs),
	}

	reader, size, err := tm.s3Client.DownloadObject(ctx, job.Bucket, job.Key)
	if err != nil {
		result.Error = fmt.Errorf("failed to download from S3: %w", err)
		return result
	}
	defer reader.Close()

	var dataReader io.Reader = reader
	var buffer bytes.Buffer

	if job.Size < 100*1024*1024 {
		if _, err := io.Copy(&buffer, reader); err != nil {
			result.Error = fmt.Errorf("failed to buffer S3 object: %w", err)
			return result
		}
		dataReader = &buffer
	}

	var encryptedData []byte
	if job.EncryptionConfig != nil && job.EncryptionConfig.Enabled {
		data, err := io.ReadAll(dataReader)
		if err != nil {
			result.Error = fmt.Errorf("failed to read data for encryption: %w", err)
			return result
		}

		encryptedData = data
		dataReader = bytes.NewReader(encryptedData)
		job.TargetName = job.TargetName + ".sealed"
	}

	data, err := io.ReadAll(dataReader)
	if err != nil {
		result.Error = fmt.Errorf("failed to read data: %w", err)
		return result
	}

	uploadResp, err := tm.walrusClient.StoreBlob(data, job.Epochs)
	if err != nil {
		result.Error = fmt.Errorf("failed to upload to Walrus: %w", err)
		return result
	}

	result.BlobID = uploadResp.BlobID
	result.Success = true
	result.ExpiryEpoch = uploadResp.EndEpoch
	result.RegisteredEpoch = uploadResp.RegisteredEpoch
	result.SuiObjectID = uploadResp.SuiObjectID

	if tm.simpleFS != nil {
		tm.simpleFS.indexMu.Lock()
		expiryEpoch := 0
		if uploadResp.EndEpoch != nil {
			expiryEpoch = int(*uploadResp.EndEpoch)
		}
		tm.simpleFS.index.Files[job.TargetName] = &SimpleFileEntry{
			BlobID:      uploadResp.BlobID,
			Size:        size,
			ModTime:     time.Now(),
			ExpiryEpoch: expiryEpoch,
		}
		tm.simpleFS.indexMu.Unlock()
		tm.simpleFS.SaveIndex()
	}

	bar.Add64(job.Size)

	return result
}

func (tm *TransferManager) TransferSingle(ctx context.Context, bucket, key string, epochs int) (*TransferResult, error) {
	obj, err := tm.s3Client.GetObjectMetadata(ctx, bucket, key)
	if err != nil {
		return nil, err
	}

	job := TransferJob{
		Bucket:     bucket,
		Key:        key,
		Size:       obj.Size,
		TargetName: path.Base(key),
		Epochs:     epochs,
	}

	if tm.dryRun {
		cost := EstimateWalrusCost(obj.Size, epochs)
		fmt.Printf("DRY RUN: Would transfer %s (%.2f MB) → %.6f WAL\n",
			key, float64(obj.Size)/(1024*1024), cost)
		return &TransferResult{
			SourceKey:     key,
			TargetName:    job.TargetName,
			Size:          obj.Size,
			Success:       true,
			EstimatedCost: cost,
		}, nil
	}

	result := tm.transferSingleFile(ctx, job, progressbar.DefaultBytes(obj.Size))
	return &result, nil
}

func (p *TransferProgress) GetSummary() string {
	duration := time.Since(p.StartTime)
	successCount := p.ProcessedFiles - p.FailedFiles

	return fmt.Sprintf(
		"Transfer Summary:\n"+
		"  Total Files: %d\n"+
		"  Successful: %d\n"+
		"  Failed: %d\n"+
		"  Total Size: %.2f MB\n"+
		"  Duration: %s\n"+
		"  Average Speed: %.2f MB/s",
		p.TotalFiles,
		successCount,
		p.FailedFiles,
		float64(p.ProcessedBytes)/(1024*1024),
		duration.Round(time.Second),
		float64(p.ProcessedBytes)/(1024*1024)/duration.Seconds(),
	)
}