package test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	fileapi "myclouddrive-go/internal/file/api"
	filemodel "myclouddrive-go/internal/file/model"
	filesvc "myclouddrive-go/internal/file/service"
	storagemodel "myclouddrive-go/internal/storage/model"
)

type mockStorage struct {
	objects map[string][]byte
}

func newMockStorage() *mockStorage {
	return &mockStorage{objects: make(map[string][]byte)}
}

func (m *mockStorage) Put(_ context.Context, in storagemodel.ObjectPutInput) (storagemodel.ObjectInfo, error) {
	raw, err := io.ReadAll(in.Reader)
	if err != nil {
		return storagemodel.ObjectInfo{}, err
	}
	m.objects[in.Key] = raw
	size := int64(len(raw))
	return storagemodel.ObjectInfo{Key: in.Key, Size: size, ContentType: in.ContentType}, nil
}

func (m *mockStorage) PresignDownloadURL(_ context.Context, key string, _ time.Duration) (string, error) {
	return "/mock-download/" + key, nil
}

func (m *mockStorage) Get(_ context.Context, key string) (io.ReadCloser, storagemodel.ObjectInfo, error) {
	raw := m.objects[key]
	return io.NopCloser(bytes.NewReader(raw)), storagemodel.ObjectInfo{Key: key, Size: int64(len(raw)), ContentType: "application/octet-stream"}, nil
}

func (m *mockStorage) Delete(_ context.Context, key string) error {
	delete(m.objects, key)
	return nil
}

func TestFileUploadFlow(t *testing.T) {
	t.Skip("待迁移到新的 storage service 构造方式后再启用")
	mux := http.NewServeMux()
	svc := filesvc.NewFileService(nil, nil, nil)
	fileapi.RegisterRoutes(mux, svc)
	server := httptest.NewServer(mux)
	defer server.Close()

	precheckReq := map[string]any{
		"fileName":    "hello.txt",
		"fileSize":    11,
		"fileHash":    "abc123",
		"totalParts":  1,
		"contentType": "text/plain",
		"parentId":    "root",
	}
	precheckResp := postJSON(t, server.URL+"/apis/transfer/check", precheckReq)
	taskID := readStringField(t, precheckResp, "data.taskId")
	if taskID == "" {
		t.Fatalf("empty taskId: %#v", precheckResp)
	}

	uploadChunk(t, server.URL+"/apis/transfer/chunk?taskId="+taskID+"&chunkIndex=1", "file", "hello.txt", []byte("hello world"))
	_ = postJSON(t, server.URL+"/apis/transfer/merge/"+taskID, map[string]any{"taskId": taskID})

	resp, err := http.Get(server.URL + "/apis/file/list?parentId=root")
	if err != nil {
		t.Fatalf("get list failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("list status=%d", resp.StatusCode)
	}
}

func postJSON(t *testing.T, url string, payload any) map[string]any {
	t.Helper()
	raw, _ := json.Marshal(payload)
	req, _ := http.NewRequest(http.MethodPost, url, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("post %s failed: %v", url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, _ := io.ReadAll(resp.Body)
	var out map[string]any
	_ = json.Unmarshal(body, &out)
	if resp.StatusCode >= 300 {
		t.Fatalf("post %s status=%d body=%s", url, resp.StatusCode, string(body))
	}
	return out
}

func uploadChunk(t *testing.T, url, fieldName, fileName string, content []byte) {
	t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile(fieldName, fileName)
	if err != nil {
		t.Fatalf("create form file failed: %v", err)
	}
	if _, err = part.Write(content); err != nil {
		t.Fatalf("write form file failed: %v", err)
	}
	_ = writer.Close()

	req, _ := http.NewRequest(http.MethodPost, url, &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("upload chunk failed: %v", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("upload chunk status=%d body=%s", resp.StatusCode, string(body))
	}
}

func readStringField(t *testing.T, payload map[string]any, path string) string {
	t.Helper()
	data, _ := payload["data"].(map[string]any)
	task, _ := data["taskId"].(string)
	return task
}

var _ filesvc.StorageGateway = (*mockStorage)(nil)
var _ = filemodel.TransferTask{}
