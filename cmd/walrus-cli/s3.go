package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/fatih/color"
	"github.com/spf13/cobra"
	"github.com/justmert/walrus-cli/backend"
)

var s3Cmd = &cobra.Command{
	Use:   "s3",
	Short: "Transfer files from AWS S3 to Walrus",
	Long:  `Commands for transferring files from AWS S3 buckets to Walrus decentralized storage`,
}

var s3ConfigureCmd = &cobra.Command{
	Use:   "configure",
	Short: "Configure AWS S3 credentials",
	RunE:  runS3Configure,
}

var s3ListBucketsCmd = &cobra.Command{
	Use:   "list-buckets",
	Short: "List all S3 buckets",
	RunE:  runS3ListBuckets,
}

var s3ListObjectsCmd = &cobra.Command{
	Use:   "list-objects",
	Short: "List objects in an S3 bucket",
	RunE:  runS3ListObjects,
}

var s3TransferCmd = &cobra.Command{
	Use:   "transfer",
	Short: "Transfer files from S3 to Walrus",
	Long: `Transfer files from an S3 bucket to Walrus storage with filtering options.

Examples:
  # Transfer all files from a bucket
  walrus-cli s3 transfer --bucket my-bucket

  # Transfer with prefix filter
  walrus-cli s3 transfer --bucket my-bucket --prefix data/2024/

  # Transfer with include/exclude patterns
  walrus-cli s3 transfer --bucket my-bucket --include "*.pdf" --exclude "temp/*"

  # Dry run to preview transfer
  walrus-cli s3 transfer --bucket my-bucket --dry-run

  # Transfer with parallel uploads
  walrus-cli s3 transfer --bucket my-bucket --parallel 5`,
	RunE: runS3Transfer,
}

var (
	s3Bucket      string
	s3Prefix      string
	s3Include     []string
	s3Exclude     []string
	s3MinSize     int64
	s3MaxSize     int64
	s3Parallel    int
	s3DryRun      bool
	s3Encrypt     bool
	s3Epochs      int
	s3AccessKey   string
	s3SecretKey   string
	s3SessionToken string
	s3Region      string
)

func init() {
	s3Cmd.AddCommand(s3ConfigureCmd)
	s3Cmd.AddCommand(s3ListBucketsCmd)
	s3Cmd.AddCommand(s3ListObjectsCmd)
	s3Cmd.AddCommand(s3TransferCmd)

	s3ListObjectsCmd.Flags().StringVar(&s3Bucket, "bucket", "", "S3 bucket name")
	s3ListObjectsCmd.Flags().StringVar(&s3Prefix, "prefix", "", "Object key prefix filter")
	s3ListObjectsCmd.MarkFlagRequired("bucket")

	s3TransferCmd.Flags().StringVar(&s3Bucket, "bucket", "", "S3 bucket name")
	s3TransferCmd.Flags().StringVar(&s3Prefix, "prefix", "", "Object key prefix filter")
	s3TransferCmd.Flags().StringSliceVar(&s3Include, "include", nil, "Include patterns (e.g., *.pdf)")
	s3TransferCmd.Flags().StringSliceVar(&s3Exclude, "exclude", nil, "Exclude patterns (e.g., temp/*)")
	s3TransferCmd.Flags().Int64Var(&s3MinSize, "min-size", 0, "Minimum file size in bytes")
	s3TransferCmd.Flags().Int64Var(&s3MaxSize, "max-size", 0, "Maximum file size in bytes")
	s3TransferCmd.Flags().IntVar(&s3Parallel, "parallel", 3, "Number of parallel transfers (1-10)")
	s3TransferCmd.Flags().BoolVar(&s3DryRun, "dry-run", false, "Preview transfer without uploading")
	s3TransferCmd.Flags().BoolVar(&s3Encrypt, "encrypt", false, "Enable Seal encryption for transferred files")
	s3TransferCmd.Flags().IntVar(&s3Epochs, "epochs", 5, "Storage duration in epochs")
	s3TransferCmd.MarkFlagRequired("bucket")

	s3Cmd.PersistentFlags().StringVar(&s3AccessKey, "access-key", "", "AWS Access Key ID")
	s3Cmd.PersistentFlags().StringVar(&s3SecretKey, "secret-key", "", "AWS Secret Access Key")
	s3Cmd.PersistentFlags().StringVar(&s3SessionToken, "session-token", "", "AWS Session Token (optional)")
	s3Cmd.PersistentFlags().StringVar(&s3Region, "region", "us-east-1", "AWS Region")
}

func getS3Credentials() (backend.S3Credentials, error) {
	creds := backend.S3Credentials{
		AccessKeyID:     s3AccessKey,
		SecretAccessKey: s3SecretKey,
		SessionToken:    s3SessionToken,
		Region:          s3Region,
	}

	if creds.AccessKeyID == "" {
		creds.AccessKeyID = os.Getenv("AWS_ACCESS_KEY_ID")
	}
	if creds.SecretAccessKey == "" {
		creds.SecretAccessKey = os.Getenv("AWS_SECRET_ACCESS_KEY")
	}
	if creds.SessionToken == "" {
		creds.SessionToken = os.Getenv("AWS_SESSION_TOKEN")
	}
	if creds.Region == "" || creds.Region == "us-east-1" {
		if region := os.Getenv("AWS_REGION"); region != "" {
			creds.Region = region
		}
	}

	if creds.AccessKeyID == "" || creds.SecretAccessKey == "" {
		return creds, fmt.Errorf("AWS credentials not found. Please set --access-key and --secret-key flags or AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY environment variables")
	}

	return creds, nil
}

func runS3Configure(cmd *cobra.Command, args []string) error {
	fmt.Println(color.CyanString("🔧 Configure AWS S3 Credentials"))
	fmt.Println(strings.Repeat("=", 40))

	var accessKey, secretKey, sessionToken, region string

	prompt := &survey.Input{
		Message: "AWS Access Key ID:",
		Default: os.Getenv("AWS_ACCESS_KEY_ID"),
	}
	survey.AskOne(prompt, &accessKey)

	prompt = &survey.Input{
		Message: "AWS Secret Access Key:",
		Default: os.Getenv("AWS_SECRET_ACCESS_KEY"),
	}
	survey.AskOne(prompt, &secretKey)

	prompt = &survey.Input{
		Message: "AWS Session Token (optional):",
		Default: os.Getenv("AWS_SESSION_TOKEN"),
	}
	survey.AskOne(prompt, &sessionToken)

	prompt = &survey.Input{
		Message: "AWS Region:",
		Default: "us-east-1",
	}
	survey.AskOne(prompt, &region)

	fmt.Println(color.GreenString("\n✅ S3 credentials configured for this session"))
	fmt.Println("\nTo persist credentials, you can:")
	fmt.Println("1. Set environment variables: AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY")
	fmt.Println("2. Use AWS CLI: aws configure")
	fmt.Println("3. Pass credentials as flags: --access-key, --secret-key")

	s3AccessKey = accessKey
	s3SecretKey = secretKey
	s3SessionToken = sessionToken
	s3Region = region

	return nil
}

func runS3ListBuckets(cmd *cobra.Command, args []string) error {
	creds, err := getS3Credentials()
	if err != nil {
		return err
	}

	s3Client, err := backend.NewS3Client(creds)
	if err != nil {
		return fmt.Errorf("failed to create S3 client: %w", err)
	}

	ctx := context.Background()
	buckets, err := s3Client.ListBuckets(ctx)
	if err != nil {
		return fmt.Errorf("failed to list buckets: %w", err)
	}

	if len(buckets) == 0 {
		fmt.Println(color.YellowString("No buckets found"))
		return nil
	}

	fmt.Println(color.CyanString("\n📦 S3 Buckets:"))
	fmt.Println(strings.Repeat("-", 40))
	for _, bucket := range buckets {
		fmt.Printf("  • %s\n", bucket)
	}
	fmt.Printf("\nTotal: %d buckets\n", len(buckets))

	return nil
}

func runS3ListObjects(cmd *cobra.Command, args []string) error {
	creds, err := getS3Credentials()
	if err != nil {
		return err
	}

	s3Client, err := backend.NewS3Client(creds)
	if err != nil {
		return fmt.Errorf("failed to create S3 client: %w", err)
	}

	filter := &backend.S3TransferFilter{
		Prefix: s3Prefix,
	}

	ctx := context.Background()
	objects, err := s3Client.ListObjects(ctx, s3Bucket, filter)
	if err != nil {
		return fmt.Errorf("failed to list objects: %w", err)
	}

	if len(objects) == 0 {
		fmt.Println(color.YellowString("No objects found"))
		return nil
	}

	fmt.Printf(color.CyanString("\n📄 Objects in bucket '%s':\n"), s3Bucket)
	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("%-50s %15s %20s\n", "Key", "Size", "Last Modified")
	fmt.Println(strings.Repeat("-", 80))

	var totalSize int64
	for _, obj := range objects {
		fmt.Printf("%-50s %15s %20s\n",
			truncateString(obj.Key, 50),
			formatS3Bytes(obj.Size),
			obj.LastModified.Format("2006-01-02 15:04:05"),
		)
		totalSize += obj.Size
	}

	fmt.Println(strings.Repeat("-", 80))
	fmt.Printf("Total: %d objects, %s\n", len(objects), formatS3Bytes(totalSize))

	return nil
}

func runS3Transfer(cmd *cobra.Command, args []string) error {
	creds, err := getS3Credentials()
	if err != nil {
		return err
	}

	s3Client, err := backend.NewS3Client(creds)
	if err != nil {
		return fmt.Errorf("failed to create S3 client: %w", err)
	}

	config, err := backend.LoadConfig("")
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	walrusClient := backend.NewWalrusClient(config.Walrus.AggregatorURL, config.Walrus.PublisherURL)
	simpleFS := backend.NewSimpleFs(config.Walrus.AggregatorURL, config.Walrus.PublisherURL)

	transferManager := backend.NewTransferManager(s3Client, walrusClient, simpleFS, s3Parallel)
	transferManager.SetDryRun(s3DryRun)
	transferManager.SetEncryption(s3Encrypt)

	filter := &backend.S3TransferFilter{
		Prefix:  s3Prefix,
		Include: s3Include,
		Exclude: s3Exclude,
		MinSize: s3MinSize,
		MaxSize: s3MaxSize,
	}

	ctx := context.Background()

	fmt.Println(color.CyanString("\n🚀 S3 to Walrus Transfer"))
	fmt.Println(strings.Repeat("=", 50))
	fmt.Printf("Bucket: %s\n", s3Bucket)
	if s3Prefix != "" {
		fmt.Printf("Prefix: %s\n", s3Prefix)
	}
	if len(s3Include) > 0 {
		fmt.Printf("Include: %s\n", strings.Join(s3Include, ", "))
	}
	if len(s3Exclude) > 0 {
		fmt.Printf("Exclude: %s\n", strings.Join(s3Exclude, ", "))
	}
	fmt.Printf("Parallel transfers: %d\n", s3Parallel)
	fmt.Printf("Storage duration: %d epochs\n", s3Epochs)
	if s3Encrypt {
		fmt.Println(color.YellowString("Encryption: Enabled (Seal)"))
	}
	if s3DryRun {
		fmt.Println(color.YellowString("Mode: DRY RUN (preview only)"))
	}
	fmt.Println(strings.Repeat("=", 50))

	totalSize, fileCount, err := s3Client.EstimateTransferSize(ctx, s3Bucket, filter)
	if err != nil {
		return fmt.Errorf("failed to estimate transfer size: %w", err)
	}

	if fileCount == 0 {
		fmt.Println(color.YellowString("\nNo files match the specified criteria"))
		return nil
	}

	fmt.Printf("\nFound %d files to transfer (%s total)\n", fileCount, formatS3Bytes(totalSize))

	totalCost, _, err := transferManager.EstimateTransferCost(ctx, s3Bucket, filter, s3Epochs)
	if err != nil {
		return fmt.Errorf("failed to estimate cost: %w", err)
	}

	fmt.Printf("Estimated cost: %.6f WAL\n", totalCost)

	if !s3DryRun {
		var confirm bool
		prompt := &survey.Confirm{
			Message: fmt.Sprintf("Proceed with transfer of %d files?", fileCount),
			Default: true,
		}
		survey.AskOne(prompt, &confirm)

		if !confirm {
			fmt.Println(color.YellowString("Transfer cancelled"))
			return nil
		}
	}

	fmt.Println()

	var encryptionConfig *backend.EncryptionSettings
	if s3Encrypt {
		encryptionConfig = &backend.EncryptionSettings{
			Enabled:   true,
			Threshold: 2,
		}
	}

	progress, err := transferManager.TransferBatch(ctx, s3Bucket, filter, s3Epochs, encryptionConfig)
	if err != nil {
		return fmt.Errorf("transfer failed: %w", err)
	}

	fmt.Println(color.GreenString("\n✅ Transfer Complete"))
	fmt.Println(progress.GetSummary())

	if progress.FailedFiles > 0 {
		fmt.Println(color.RedString("\n❌ Failed Transfers:"))
		for _, result := range progress.Results {
			if !result.Success && result.Error != nil {
				fmt.Printf("  • %s: %v\n", result.SourceKey, result.Error)
			}
		}
	}

	return nil
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func formatS3Bytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}