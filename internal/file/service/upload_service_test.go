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
	taskID := svc.initTransferTask(filemodel.UploadInitInput{
		FileName:   "report.txt",
		FileHash:   "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		FileSize:   10,
		ParentID:   "root",
		TotalParts: 2,
	}, "local-a")

	if err := svc.UploadChunk(taskID, 1, []byte("hello"), sha256Hex([]byte("hello"))); err != nil {
		t.Fatalf("UploadChunk part 1 returned error: %v", err)
	}
	if err := svc.UploadChunk(taskID, 2, []byte("world"), sha256Hex([]byte("world"))); err != nil {
		t.Fatalf("UploadChunk part 2 returned error: %v", err)
	}

	if _, err := svc.MergeUpload(context.Background(), taskID); err == nil {
		t.Fatalf("expected merge hash mismatch error")
	}
}
