package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	agentapi "myclouddrive-go/internal/agent/api/gen"
	agentdb "myclouddrive-go/internal/agent/model/dbmodel"
	"myclouddrive-go/internal/framework/code"
)

const (
	kfPending    = "pending"
	kfProcessing = "processing"
	kfDone       = "done"
	kfFailed     = "failed"
)

func normalizeStageStatus(v string) string {
	switch strings.TrimSpace(strings.ToLower(v)) {
	case "done", "success":
		return kfDone
	case "processing", "running":
		return kfProcessing
	case "failed", "error":
		return kfFailed
	default:
		return kfPending
	}
}

func classifyImportError(err error) string {
	if err == nil {
		return ""
	}
	switch {
	case code.Is(err, code.BadRequest):
		return "PARAM_ERROR"
	case code.Is(err, code.NoPermission):
		return "PERMISSION_ERROR"
	case code.Is(err, code.NotFound):
		return "BUSINESS_ERROR"
	default:
		return "SYSTEM_ERROR"
	}
}

// ListKnowledgeFiles 列出知识库文件及处理状态。
func (s *AgentService) ListKnowledgeFiles(ctx context.Context, workspaceID string, knowledgeID int64) ([]agentapi.KnowledgeFileDetail, error) {
	if s == nil || s.runSvc == nil || s.runSvc.db == nil {
		return nil, code.New(code.InternalError, "agent db unavailable")
	}
	if strings.TrimSpace(workspaceID) == "" {
		return nil, code.New(code.BadRequest, "workspace required")
	}
	if knowledgeID <= 0 {
		return nil, code.New(code.BadRequest, "invalid knowledge id")
	}
	if err := s.ensureKnowledgeOwnership(ctx, workspaceID, knowledgeID); err != nil {
		return nil, err
	}

	var rows []agentdb.KnowledgeFile
	if err := s.runSvc.db.WithContext(ctx).
		Where("workspace_id = ? AND knowledge_base_id = ?", strings.TrimSpace(workspaceID), knowledgeID).
		Order("id desc").
		Find(&rows).Error; err != nil {
		return nil, code.New(code.InternalError, "list knowledge files failed: "+err.Error())
	}

	result := make([]agentapi.KnowledgeFileDetail, 0, len(rows))
	for _, row := range rows {
		fileName := row.FileID
		if s.fileSvc != nil {
			if item, err := s.fileSvc.Get(ctx, row.FileID); err == nil && item != nil && strings.TrimSpace(item.Name) != "" {
				fileName = item.Name
			}
		}
		id := int(row.ID)
		fileID := row.FileID
		parse := agentapi.KnowledgeFileDetailParseStatus(normalizeStageStatus(row.ParseStatus))
		chunk := agentapi.KnowledgeFileDetailChunkStatus(normalizeStageStatus(row.ChunkStatus))
		embed := agentapi.KnowledgeFileDetailEmbedStatus(normalizeStageStatus(row.EmbedStatus))
		index := agentapi.KnowledgeFileDetailIndexStatus(normalizeStageStatus(row.IndexStatus))
		createdAt := row.CreatedAt
		result = append(result, agentapi.KnowledgeFileDetail{
			Id:          &id,
			FileId:      &fileID,
			FileName:    &fileName,
			ParseStatus: &parse,
			ChunkStatus: &chunk,
			EmbedStatus: &embed,
			IndexStatus: &index,
			CreatedAt:   &createdAt,
		})
	}
	return result, nil
}

// AddKnowledgeFile 创建异步导入任务（parse -> chunk -> embed -> index）。
func (s *AgentService) AddKnowledgeFile(ctx context.Context, workspaceID string, knowledgeID int64, fileID, storageSettingID string) (*agentapi.KnowledgeFileDetail, error) {
	if s == nil || s.runSvc == nil || s.runSvc.db == nil {
		return nil, code.New(code.InternalError, "agent db unavailable")
	}
	ws := strings.TrimSpace(workspaceID)
	fid := strings.TrimSpace(fileID)
	ssid := strings.TrimSpace(storageSettingID)
	if ws == "" {
		return nil, code.New(code.BadRequest, "workspace required")
	}
	if knowledgeID <= 0 {
		return nil, code.New(code.BadRequest, "invalid knowledge id")
	}
	if fid == "" {
		return nil, code.New(code.BadRequest, "fileId is required")
	}
	if ssid == "" {
		return nil, code.New(code.BadRequest, "storageSettingId is required")
	}
	if err := s.ensureKnowledgeOwnership(ctx, ws, knowledgeID); err != nil {
		return nil, err
	}
	if s.fileSvc == nil {
		return nil, code.New(code.InternalError, "file service unavailable")
	}

	item, err := s.fileSvc.Get(ctx, fid)
	if err != nil {
		return nil, code.New(code.BadRequest, "file not found or no permission")
	}
	if item == nil || item.IsDir {
		return nil, code.New(code.BadRequest, "only file can be imported")
	}
	if strings.TrimSpace(item.StorageSettingID) != "" && strings.TrimSpace(item.StorageSettingID) != ssid {
		return nil, code.New(code.BadRequest, "file does not belong to storageSettingId")
	}

	var record agentdb.KnowledgeFile
	err = s.runSvc.db.WithContext(ctx).
		Where("workspace_id = ? AND knowledge_base_id = ? AND file_id = ?", ws, knowledgeID, fid).
		First(&record).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, code.New(code.InternalError, "query knowledge file failed: "+err.Error())
	}

	now := time.Now()
	if errors.Is(err, gorm.ErrRecordNotFound) {
		record = agentdb.KnowledgeFile{
			KnowledgeBaseID:  knowledgeID,
			WorkspaceID:      ws,
			StorageSettingID: ssid,
			FileID:           fid,
			ParseStatus:      kfPending,
			ChunkStatus:      kfPending,
			EmbedStatus:      kfPending,
			IndexStatus:      kfPending,
			CreatedAt:        now,
			UpdatedAt:        now,
		}
		if err = s.runSvc.db.WithContext(ctx).Create(&record).Error; err != nil {
			return nil, code.New(code.InternalError, "create knowledge file failed: "+err.Error())
		}
	} else {
		// 处理中任务直接幂等返回。
		if normalizeStageStatus(record.IndexStatus) == kfProcessing ||
			normalizeStageStatus(record.EmbedStatus) == kfProcessing ||
			normalizeStageStatus(record.ChunkStatus) == kfProcessing ||
			normalizeStageStatus(record.ParseStatus) == kfProcessing {
			return s.toKnowledgeFileDetail(ctx, &record), nil
		}
		if err = s.updateKnowledgeFileStatus(ctx, record.ID, kfPending, kfPending, kfPending, kfPending); err != nil {
			return nil, err
		}
	}

	taskID := "kbt_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	task := agentdb.KnowledgeImportTask{
		ID:               taskID,
		WorkspaceID:      ws,
		KnowledgeBaseID:  knowledgeID,
		KnowledgeFileID:  record.ID,
		FileID:           fid,
		StorageSettingID: ssid,
		Status:           "pending",
		Stage:            "pending",
		Progress:         0,
		RetryCount:       0,
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err = s.runSvc.db.WithContext(ctx).Create(&task).Error; err != nil {
		return nil, code.New(code.InternalError, "create import task failed: "+err.Error())
	}
	go s.runKnowledgeImportTask(taskID, ws, knowledgeID, &record)
	return s.toKnowledgeFileDetail(ctx, &record), nil
}

func (s *AgentService) runKnowledgeImportTask(taskID, workspaceID string, knowledgeID int64, record *agentdb.KnowledgeFile) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	if record == nil {
		return
	}
	actionID := taskID
	_ = s.runSvc.createAction(ctx, actionID, "", workspaceID, "knowledge import", "rag", "running", "write")
	s.logImportStep(ctx, actionID, "knowledge.import.start", "success", "start")
	_ = s.updateImportTask(ctx, taskID, "running", "parsing", 10, "", "")
	_ = s.updateKnowledgeFileStatus(ctx, record.ID, kfProcessing, kfPending, kfPending, kfPending)

	content, err := s.parseFileContent(ctx, record.FileID)
	if err != nil {
		s.finishImportTaskFailed(ctx, taskID, record.ID, actionID, "parsing", code.New(code.InternalError, "parse failed: "+err.Error()))
		return
	}
	s.logImportStep(ctx, actionID, "knowledge.import.parsing", "success", "parse done")
	_ = s.updateImportTask(ctx, taskID, "running", "chunking", 30, "", "")
	_ = s.updateKnowledgeFileStatus(ctx, record.ID, kfDone, kfProcessing, kfPending, kfPending)

	chunks := splitChunks(content, 700)
	if len(chunks) == 0 {
		chunks = []string{fmt.Sprintf("FILE:%s", record.FileID)}
	}
	if err = s.replaceKnowledgeChunks(ctx, knowledgeID, record.FileID, record.StorageSettingID, chunks); err != nil {
		s.finishImportTaskFailed(ctx, taskID, record.ID, actionID, "chunking", err)
		return
	}
	s.logImportStep(ctx, actionID, "knowledge.import.chunking", "success", fmt.Sprintf("chunks=%d", len(chunks)))
	_ = s.updateImportTask(ctx, taskID, "running", "embedding", 60, "", "")
	_ = s.updateKnowledgeFileStatus(ctx, record.ID, kfDone, kfDone, kfProcessing, kfPending)

	if err = s.embedAndIndexKnowledge(ctx, workspaceID, knowledgeID, record.FileID); err != nil {
		s.finishImportTaskFailed(ctx, taskID, record.ID, actionID, "embedding", err)
		return
	}
	s.logImportStep(ctx, actionID, "knowledge.import.embedding", "success", "embed done")
	_ = s.updateImportTask(ctx, taskID, "running", "indexing", 85, "", "")
	_ = s.updateKnowledgeFileStatus(ctx, record.ID, kfDone, kfDone, kfDone, kfProcessing)

	s.logImportStep(ctx, actionID, "knowledge.import.indexing", "success", "index done")
	_ = s.updateKnowledgeFileStatus(ctx, record.ID, kfDone, kfDone, kfDone, kfDone)
	_ = s.updateImportTask(ctx, taskID, "success", "done", 100, "", "")
	_ = s.runSvc.updateActionStatus(ctx, actionID, "success", "")
	s.logImportStep(ctx, actionID, "knowledge.import.done", "success", "completed")
}

func (s *AgentService) finishImportTaskFailed(ctx context.Context, taskID string, knowledgeFileID int64, actionID, stage string, err error) {
	category := classifyImportError(err)
	_ = s.updateKnowledgeFileStatus(ctx, knowledgeFileID, kfFailed, kfFailed, kfFailed, kfFailed)
	_ = s.updateImportTask(ctx, taskID, "failed", stage, 100, category, err.Error())
	_ = s.runSvc.updateActionStatus(ctx, actionID, "failed", "")
	s.logImportStep(ctx, actionID, "knowledge.import."+stage, "failed", err.Error())
	log.Printf("agent_step trace=%s step=knowledge.import.failed stage=%s category=%s err=%q", taskID, stage, category, err.Error())
}

func (s *AgentService) logImportStep(ctx context.Context, traceID, step, status, content string) {
	log.Printf("agent_step trace=%s step=%s status=%s detail=%q", traceID, step, status, content)
	_, _ = s.runSvc.createStep(ctx, traceID, "observe", step+" "+content, status)
}

func (s *AgentService) updateImportTask(ctx context.Context, taskID, status, stage string, progress int32, category, msg string) error {
	updates := map[string]any{
		"status":     status,
		"stage":      stage,
		"progress":   progress,
		"updated_at": time.Now(),
	}
	if strings.TrimSpace(category) != "" {
		updates["error_category"] = category
	}
	if strings.TrimSpace(msg) != "" {
		updates["error_message"] = msg
	}
	return s.runSvc.db.WithContext(ctx).Model(&agentdb.KnowledgeImportTask{}).Where("id = ?", taskID).Updates(updates).Error
}

func (s *AgentService) RemoveKnowledgeFile(ctx context.Context, workspaceID string, knowledgeID int64, fileID string) error {
	if s == nil || s.runSvc == nil || s.runSvc.db == nil {
		return code.New(code.InternalError, "agent db unavailable")
	}
	ws := strings.TrimSpace(workspaceID)
	fid := strings.TrimSpace(fileID)
	if ws == "" {
		return code.New(code.BadRequest, "workspace required")
	}
	if knowledgeID <= 0 {
		return code.New(code.BadRequest, "invalid knowledge id")
	}
	if fid == "" {
		return code.New(code.BadRequest, "fileId is required")
	}
	if err := s.ensureKnowledgeOwnership(ctx, ws, knowledgeID); err != nil {
		return err
	}
	if err := s.runSvc.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("workspace_id = ? AND knowledge_base_id = ? AND file_id = ?", ws, knowledgeID, fid).
			Delete(&agentdb.KnowledgeFile{}).Error; err != nil {
			return code.New(code.InternalError, "delete knowledge file failed: "+err.Error())
		}
		if err := tx.Where("knowledge_base_id = ? AND file_id = ?", knowledgeID, fid).
			Delete(&agentdb.KnowledgeDocumentChunk{}).Error; err != nil {
			return code.New(code.InternalError, "delete knowledge chunks failed: "+err.Error())
		}
		if err := tx.Where("workspace_id = ? AND knowledge_base_id = ? AND file_id = ?", ws, knowledgeID, fid).
			Delete(&agentdb.KnowledgeImportTask{}).Error; err != nil {
			return code.New(code.InternalError, "delete import task failed: "+err.Error())
		}
		return nil
	}); err != nil {
		return err
	}
	if s.ragIndexer != nil {
		if err := s.rebuildNamespaceIndex(ctx, ragNamespace(ws, fmt.Sprintf("%d", knowledgeID)), fmt.Sprintf("%d", knowledgeID)); err != nil {
			return err
		}
	}
	return nil
}

func (s *AgentService) replaceKnowledgeChunks(ctx context.Context, knowledgeID int64, fileID, storageSettingID string, chunks []string) error {
	if err := s.runSvc.db.WithContext(ctx).
		Where("knowledge_base_id = ? AND file_id = ?", knowledgeID, fileID).
		Delete(&agentdb.KnowledgeDocumentChunk{}).Error; err != nil {
		return code.New(code.InternalError, "clear old chunks failed: "+err.Error())
	}
	for idx, chunk := range chunks {
		meta := map[string]any{"storageSettingId": storageSettingID, "chunkNo": idx + 1}
		metaJSON, _ := json.Marshal(meta)
		row := agentdb.KnowledgeDocumentChunk{
			KnowledgeBaseID: knowledgeID,
			FileID:          fileID,
			ChunkNo:         int32(idx + 1),
			Content:         chunk,
			TokenCount:      int32(len([]rune(chunk))),
			VectorID:        "",
			MetadataJSON:    string(metaJSON),
			CreatedAt:       time.Now(),
		}
		if err := s.runSvc.db.WithContext(ctx).Create(&row).Error; err != nil {
			return code.New(code.InternalError, "insert chunk failed: "+err.Error())
		}
	}
	return nil
}

func (s *AgentService) embedAndIndexKnowledge(ctx context.Context, workspaceID string, knowledgeID int64, fileID string) error {
	if s.ragIndexer == nil {
		return code.New(code.InternalError, "rag indexer unavailable")
	}
	if err := s.rebuildNamespaceIndex(ctx, ragNamespace(workspaceID, fmt.Sprintf("%d", knowledgeID)), fmt.Sprintf("%d", knowledgeID)); err != nil {
		return err
	}
	var chunks []agentdb.KnowledgeDocumentChunk
	if err := s.runSvc.db.WithContext(ctx).Where("knowledge_base_id = ? AND file_id = ?", knowledgeID, fileID).Find(&chunks).Error; err != nil {
		return code.New(code.InternalError, "load chunks failed: "+err.Error())
	}
	for _, row := range chunks {
		vectorID := fmt.Sprintf("vec_%d_%d", row.KnowledgeBaseID, row.ID)
		meta := map[string]any{}
		if strings.TrimSpace(row.MetadataJSON) != "" {
			_ = json.Unmarshal([]byte(row.MetadataJSON), &meta)
		}
		meta["indexedAt"] = time.Now().Format(time.RFC3339)
		metaJSON, _ := json.Marshal(meta)
		if err := s.runSvc.db.WithContext(ctx).Model(&agentdb.KnowledgeDocumentChunk{}).Where("id = ?", row.ID).
			Updates(map[string]any{"vector_id": vectorID, "metadata_json": string(metaJSON)}).Error; err != nil {
			return code.New(code.InternalError, "mark vector failed: "+err.Error())
		}
	}
	return nil
}

func (s *AgentService) updateKnowledgeFileStatus(ctx context.Context, id int64, parse, chunk, embed, index string) error {
	err := s.runSvc.db.WithContext(ctx).Model(&agentdb.KnowledgeFile{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"parse_status": normalizeStageStatus(parse),
			"chunk_status": normalizeStageStatus(chunk),
			"embed_status": normalizeStageStatus(embed),
			"index_status": normalizeStageStatus(index),
			"updated_at":   time.Now(),
		}).Error
	if err != nil {
		return code.New(code.InternalError, "update knowledge file status failed: "+err.Error())
	}
	return nil
}

func (s *AgentService) ensureKnowledgeOwnership(ctx context.Context, workspaceID string, knowledgeID int64) error {
	var kb agentdb.Knowledge
	if err := s.runSvc.db.WithContext(ctx).
		Where("id = ? AND workspace_id = ?", knowledgeID, strings.TrimSpace(workspaceID)).
		First(&kb).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return code.New(code.NotFound, "knowledge not found")
		}
		return code.New(code.InternalError, "query knowledge failed: "+err.Error())
	}
	return nil
}

func (s *AgentService) toKnowledgeFileDetail(ctx context.Context, row *agentdb.KnowledgeFile) *agentapi.KnowledgeFileDetail {
	if row == nil {
		return nil
	}
	fileName := row.FileID
	if s.fileSvc != nil {
		if item, err := s.fileSvc.Get(ctx, row.FileID); err == nil && item != nil && strings.TrimSpace(item.Name) != "" {
			fileName = item.Name
		}
	}
	id := int(row.ID)
	fileID := row.FileID
	parse := agentapi.KnowledgeFileDetailParseStatus(normalizeStageStatus(row.ParseStatus))
	chunk := agentapi.KnowledgeFileDetailChunkStatus(normalizeStageStatus(row.ChunkStatus))
	embed := agentapi.KnowledgeFileDetailEmbedStatus(normalizeStageStatus(row.EmbedStatus))
	index := agentapi.KnowledgeFileDetailIndexStatus(normalizeStageStatus(row.IndexStatus))
	createdAt := row.CreatedAt
	return &agentapi.KnowledgeFileDetail{
		Id:          &id,
		FileId:      &fileID,
		FileName:    &fileName,
		ParseStatus: &parse,
		ChunkStatus: &chunk,
		EmbedStatus: &embed,
		IndexStatus: &index,
		CreatedAt:   &createdAt,
	}
}

func (s *AgentService) parseFileContent(ctx context.Context, fileID string) (string, error) {
	if s.fileSvc == nil {
		return "", code.New(code.InternalError, "file service unavailable")
	}
	item, err := s.fileSvc.Get(ctx, fileID)
	if err != nil || item == nil {
		if err != nil {
			return "", err
		}
		return "", code.New(code.NotFound, "file not found")
	}
	rc, _, _, err := s.fileSvc.OpenPreviewContent(ctx, fileID)
	if err != nil {
		return "", err
	}
	if rc == nil {
		return "", code.New(code.BadRequest, "file content unavailable")
	}
	defer func() { _ = rc.Close() }()
	raw, err := io.ReadAll(io.LimitReader(rc, 4*1024*1024))
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func splitChunks(content string, maxRunes int) []string {
	txt := strings.TrimSpace(content)
	if txt == "" {
		return nil
	}
	if maxRunes <= 0 {
		maxRunes = 700
	}
	runes := []rune(txt)
	if len(runes) <= maxRunes {
		return []string{txt}
	}
	chunks := make([]string, 0, (len(runes)/maxRunes)+1)
	for start := 0; start < len(runes); start += maxRunes {
		end := start + maxRunes
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[start:end]))
	}
	return chunks
}
