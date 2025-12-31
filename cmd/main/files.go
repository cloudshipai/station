package main

import (
	"github.com/spf13/cobra"
	"station/cmd/main/handlers"
)

var (
	filesCmd = &cobra.Command{
		Use:   "files",
		Short: "Manage files in NATS Object Store",
		Long: `Manage files for staging data to/from sandbox containers.

Files are stored in NATS JetStream Object Store and can be:
  - Uploaded from local filesystem
  - Downloaded to local filesystem
  - Staged into sandbox containers by agents
  - Published from sandbox containers by agents

File key conventions:
  - files/{file_id}          - User-uploaded files
  - runs/{run_id}/output/*   - Workflow run outputs
  - sessions/{session_id}/*  - Session artifacts`,
	}

	filesUploadCmd = &cobra.Command{
		Use:   "upload <path>",
		Short: "Upload a file to the object store",
		Long: `Upload a file from local filesystem to NATS Object Store.

The file will be stored with an auto-generated key (files/f_{ulid}) unless
a custom key is specified with --key flag.

Examples:
  stn files upload data.csv
  stn files upload --key mydata data.csv
  stn files upload --ttl 24h report.pdf
  stn files upload --station http://localhost:8585 data.csv`,
		Args: cobra.ExactArgs(1),
		RunE: runFilesUpload,
	}

	filesDownloadCmd = &cobra.Command{
		Use:   "download <file_key>",
		Short: "Download a file from the object store",
		Long: `Download a file from NATS Object Store to local filesystem.

The file key can be the full key (files/f_abc123) or just the file ID (f_abc123).

Examples:
  stn files download f_abc123 -o output.csv
  stn files download files/f_abc123
  stn files download runs/run123/output/result.csv -o result.csv
  stn files download --station http://localhost:8585 f_abc123 -o data.csv`,
		Args: cobra.ExactArgs(1),
		RunE: runFilesDownload,
	}

	filesListCmd = &cobra.Command{
		Use:   "list",
		Short: "List files in the object store",
		Long: `List files stored in NATS Object Store.

Use --prefix to filter files by key prefix.

Examples:
  stn files list
  stn files list --prefix files/
  stn files list --prefix runs/
  stn files list --json
  stn files list --station http://localhost:8585`,
		RunE: runFilesList,
	}

	filesDeleteCmd = &cobra.Command{
		Use:   "delete <file_key>",
		Short: "Delete a file from the object store",
		Long: `Delete a file from NATS Object Store.

Use --force to skip confirmation prompt.

Examples:
  stn files delete f_abc123
  stn files delete files/f_abc123 --force
  stn files delete --station http://localhost:8585 f_abc123`,
		Args: cobra.ExactArgs(1),
		RunE: runFilesDelete,
	}

	filesInfoCmd = &cobra.Command{
		Use:   "info <file_key>",
		Short: "Show file metadata",
		Long: `Show metadata for a file in NATS Object Store.

Examples:
  stn files info f_abc123
  stn files info files/f_abc123
  stn files info --station http://localhost:8585 f_abc123`,
		Args: cobra.ExactArgs(1),
		RunE: runFilesInfo,
	}
)

func runFilesUpload(cmd *cobra.Command, args []string) error {
	filesHandler := handlers.NewFilesHandler(themeManager)
	return filesHandler.Upload(cmd, args)
}

func runFilesDownload(cmd *cobra.Command, args []string) error {
	filesHandler := handlers.NewFilesHandler(themeManager)
	return filesHandler.Download(cmd, args)
}

func runFilesList(cmd *cobra.Command, args []string) error {
	filesHandler := handlers.NewFilesHandler(themeManager)
	return filesHandler.List(cmd, args)
}

func runFilesDelete(cmd *cobra.Command, args []string) error {
	filesHandler := handlers.NewFilesHandler(themeManager)
	return filesHandler.Delete(cmd, args)
}

func runFilesInfo(cmd *cobra.Command, args []string) error {
	filesHandler := handlers.NewFilesHandler(themeManager)
	return filesHandler.Info(cmd, args)
}
