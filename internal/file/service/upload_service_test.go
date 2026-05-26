package service

import (
	"context"
	"testing"
	"time"

	filemodel "myclouddrive-go/internal/file/model"
)

func TestPrecheckUpload_StrongMatchCreatesInstantFile(t *testing.T) {
	svc := NewFileService(nil)
	now := time.Now()
	svc.items["src"] = &filemodel.FileItem{
		ID:               "src",
		ParentID:         "root",
		StorageSettingID: "local-a",
		Name:             "report.pdf",
		Size:             128,
		FileHash:         "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ObjectKey:        "objects/report.pdf",
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	result, err := svc.PrecheckUpload(context.Background(), filemodel.UploadInitInput{
		FileName:   "report.pdf",
		FileSize:   128,
		FileHash:   "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		ParentID:   "root",
		TotalParts: 1,
	}, "local-a")
	if err != nil {
		t.Fatalf("PrecheckUpload returned error: %v", err)
	}
	if !result.SkipUpload {
		t.Fatalf("expected skip upload")
	}
	if !result.StrongMatch {
		t.Fatalf("expected strong match")
	}
	if result.WeakMatchCount != 1 {
		t.Fatalf("expected weak match count 1, got %d", result.WeakMatchCount)
	}
	if result.InstantFile == nil {
		t.Fatalf("expected instant file metadata")
	}
	if result.InstantFile.ObjectKey != "objects/report.pdf" {
		t.Fatalf("unexpected object key: %s", result.InstantFile.ObjectKey)
	}
	if result.InstantFile.ID == "src" {
		t.Fatalf("expected cloned file metadata, got original id")
	}
}

func TestPrecheckUpload_WeakMatchFallsBackToChunkUpload(t *testing.T) {
	svc := NewFileService(nil)
	now := time.Now()
	svc.items["src"] = &filemodel.FileItem{
		ID:               "src",
		ParentID:         "root",
		StorageSettingID: "local-a",
		Name:             "report.pdf",
		Size:             128,
		FileHash:         "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		ObjectKey:        "objects/report.pdf",
		CreatedAt:        now,
		UpdatedAt:        now,
	}

	result, err := svc.PrecheckUpload(context.Background(), filemodel.UploadInitInput{
		FileName:   "report.pdf",
		FileSize:   128,
		FileHash:   "cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
		ParentID:   "root",
		TotalParts: 2,
	}, "local-a")
	if err != nil {
		t.Fatalf("PrecheckUpload returned error: %v", err)
	}
	if result.SkipUpload {
		t.Fatalf("expected chunk upload path")
	}
	if result.WeakMatchCount != 1 {
		t.Fatalf("expected weak match count 1, got %d", result.WeakMatchCount)
	}
	if result.TaskID == "" {
		t.Fatalf("expected upload task id")
	}
}

func TestMergeUpload_VerifiesFinalFileHash(t *testing.T) {
	svc := NewFileService(nil)
	taskID := svc.initTransferTask(context.Background(), filemodel.UploadInitInput{
		FileName:   "report.txt",
		FileHash:   "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		FileSize:   10,
		ParentID:   "root",
		TotalParts: 2,
	}, "local-a")

	if _, err := svc.UploadChunk(context.Background(), taskID, 1, []byte("hello"), sha256Hex([]byte("hello"))); err != nil {
		t.Fatalf("UploadChunk part 1 returned error: %v", err)
	}
	if _, err := svc.UploadChunk(context.Background(), taskID, 2, []byte("world"), sha256Hex([]byte("world"))); err != nil {
		t.Fatalf("UploadChunk part 2 returned error: %v", err)
	}

	if _, err := svc.MergeUpload(context.Background(), taskID); err == nil {
		t.Fatalf("expected merge hash mismatch error")
	}
}

func TestUploadChunk_PauseBlocksNewChunkButKeepsUploadedProgress(t *testing.T) {
	svc := NewFileService(nil)
	taskID := svc.initTransferTask(context.Background(), filemodel.UploadInitInput{
		FileName:   "paused.bin",
		FileSize:   10,
		ParentID:   "root",
		TotalParts: 2,
	}, "local-a")

	progress, err := svc.UploadChunk(context.Background(), taskID, 1, []byte("hello"), sha256Hex([]byte("hello")))
	if err != nil {
		t.Fatalf("UploadChunk part 1 returned error: %v", err)
	}
	if got := progress["uploadedParts"]; got != 1 {
		t.Fatalf("expected uploadedParts 1 before pause, got %v", got)
	}

	if err := svc.PauseTransfer(context.Background(), taskID); err != nil {
		t.Fatalf("PauseTransfer returned error: %v", err)
	}

	if _, err := svc.UploadChunk(context.Background(), taskID, 2, []byte("world"), sha256Hex([]byte("world"))); err == nil {
		t.Fatalf("expected paused task to reject new chunk")
	}

	task := svc.getTransferTask(taskID)
	if task == nil {
		t.Fatalf("expected task to exist")
	}
	if task.Status != filemodel.TransferTaskPaused {
		t.Fatalf("expected paused status, got %s", task.Status)
	}
	if task.UploadedPart != 1 {
		t.Fatalf("expected uploadedParts to remain 1, got %d", task.UploadedPart)
	}
}

func TestCancelTransfer_RegistersCleanupJobAndBlocksMerge(t *testing.T) {
	svc := NewFileService(nil)
	taskID := svc.initTransferTask(context.Background(), filemodel.UploadInitInput{
		FileName:   "cancel.bin",
		FileSize:   10,
		ParentID:   "root",
		TotalParts: 2,
	}, "local-a")

	if _, err := svc.UploadChunk(context.Background(), taskID, 1, []byte("hello"), sha256Hex([]byte("hello"))); err != nil {
		t.Fatalf("UploadChunk part 1 returned error: %v", err)
	}

	if err := svc.CancelTransfer(context.Background(), taskID); err != nil {
		t.Fatalf("CancelTransfer returned error: %v", err)
	}

	if _, err := svc.MergeUpload(context.Background(), taskID); err == nil {
		t.Fatalf("expected canceled task to reject merge")
	}

	foundCleanup := false
	for _, job := range svc.cleanupJobs {
		if job.TaskID == taskID && job.JobType == "cancel_cleanup" {
			foundCleanup = true
			break
		}
	}
	if !foundCleanup {
		t.Fatalf("expected cancel cleanup job to be registered")
	}
}

func TestMergeUpload_CompletesAndRegistersCleanupJob(t *testing.T) {
	svc := NewFileService(nil)
	finalBytes := []byte("helloworld")
	taskID := svc.initTransferTask(context.Background(), filemodel.UploadInitInput{
		FileName:   "done.txt",
		FileHash:   sha256Hex(finalBytes),
		FileSize:   int64(len(finalBytes)),
		ParentID:   "root",
		TotalParts: 2,
	}, "local-a")

	if _, err := svc.UploadChunk(context.Background(), taskID, 1, []byte("hello"), sha256Hex([]byte("hello"))); err != nil {
		t.Fatalf("UploadChunk part 1 returned error: %v", err)
	}
	if _, err := svc.UploadChunk(context.Background(), taskID, 2, []byte("world"), sha256Hex([]byte("world"))); err != nil {
		t.Fatalf("UploadChunk part 2 returned error: %v", err)
	}

	item, err := svc.MergeUpload(context.Background(), taskID)
	if err != nil {
		t.Fatalf("MergeUpload returned error: %v", err)
	}
	if item == nil {
		t.Fatalf("expected merged file item")
	}

	task := svc.getTransferTask(taskID)
	if task == nil {
		t.Fatalf("expected task to exist")
	}
	if task.Status != filemodel.TransferTaskCompleted {
		t.Fatalf("expected completed status, got %s", task.Status)
	}

	foundCleanup := false
	for _, job := range svc.cleanupJobs {
		if job.TaskID == taskID && job.JobType == "merge_cleanup" {
			foundCleanup = true
			break
		}
	}
	if !foundCleanup {
		t.Fatalf("expected merge cleanup job to be registered")
	}
}

func TestPauseTransfer_RejectsMergingTask(t *testing.T) {
	svc := NewFileService(nil)
	taskID := svc.initTransferTask(context.Background(), filemodel.UploadInitInput{
		FileName:   "merging.bin",
		FileSize:   10,
		ParentID:   "root",
		TotalParts: 2,
	}, "local-a")

	task := svc.getTransferTask(taskID)
	if task == nil {
		t.Fatalf("expected task to exist")
	}
	task.Status = filemodel.TransferTaskMerging

	if err := svc.PauseTransfer(context.Background(), taskID); err == nil {
		t.Fatalf("expected merging task to reject pause")
	}
}
