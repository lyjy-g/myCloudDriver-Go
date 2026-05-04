package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"gorm.io/gorm"
	agentapi "myclouddrive-go/internal/agent/api/gen"
	agentdb "myclouddrive-go/internal/agent/model/dbmodel"
	"myclouddrive-go/internal/framework/code"
)

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
		parse := agentapi.KnowledgeFileDetailParseStatus(row.ParseStatus)
		chunk := agentapi.KnowledgeFileDetailChunkStatus(row.ChunkStatus)
		embed := agentapi.KnowledgeFileDetailEmbedStatus(row.EmbedStatus)
		index := agentapi.KnowledgeFileDetailIndexStatus(row.IndexStatus)
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

// AddKnowledgeFile 导入文件到知识库并执行 parse -> chunk -> embed -> index。
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
	if err == nil {
		// 已导入则返回当前状态，保证幂等。
		return s.toKnowledgeFileDetail(ctx, &record), nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, code.New(code.InternalError, "query knowledge file failed: "+err.Error())
	}

	now := time.Now()
	record = agentdb.KnowledgeFile{
		KnowledgeBaseID:  knowledgeID,
		WorkspaceID:      ws,
		StorageSettingID: ssid,
		FileID:           fid,
		ParseStatus:      "processing",
		ChunkStatus:      "pending",
		EmbedStatus:      "pending",
		IndexStatus:      "pending",
		CreatedAt:        now,
		UpdatedAt:        now,
	}
	if err = s.runSvc.db.WithContext(ctx).Create(&record).Error; err != nil {
		return nil, code.New(code.InternalError, "create knowledge file failed: "+err.Error())
	}

	// Stage 1: parse
	content, parseErr := s.parseFileContent(ctx, fid)
	if parseErr != nil {
		_ = s.updateKnowledgeFileStatus(ctx, record.ID, "failed", "failed", "failed", "failed")
		return nil, code.New(code.InternalError, "parse failed: "+parseErr.Error())
	}
	if err = s.updateKnowledgeFileStatus(ctx, record.ID, "success", "processing", "pending", "pending"); err != nil {
		return nil, err
	}

	// Stage 2: chunk
	chunks := splitChunks(content, 700)
	if len(chunks) == 0 {
		chunks = []string{fmt.Sprintf("FILE:%s", strings.TrimSpace(item.Name))}
	}
	if err = s.replaceKnowledgeChunks(ctx, knowledgeID, fid, ssid, chunks); err != nil {
		_ = s.updateKnowledgeFileStatus(ctx, record.ID, "success", "failed", "failed", "failed")
		return nil, err
	}
	if err = s.updateKnowledgeFileStatus(ctx, record.ID, "success", "success", "processing", "pending"); err != nil {
		return nil, err
	}

	// Stage 3 & 4: embed/index（当前实现落 metadata/vector_id，后续可替换真实向量库）
	if err = s.markChunkEmbeddedAndIndexed(ctx, knowledgeID, fid); err != nil {
		_ = s.updateKnowledgeFileStatus(ctx, record.ID, "success", "success", "failed", "failed")
		return nil, err
	}
	if err = s.updateKnowledgeFileStatus(ctx, record.ID, "success", "success", "success", "success"); err != nil {
		return nil, err
	}

	_ = s.runSvc.db.WithContext(ctx).First(&record, record.ID).Error
	return s.toKnowledgeFileDetail(ctx, &record), nil
}

func (s *AgentService) replaceKnowledgeChunks(ctx context.Context, knowledgeID int64, fileID, storageSettingID string, chunks []string) error {
	if err := s.runSvc.db.WithContext(ctx).
		Where("knowledge_base_id = ? AND file_id = ?", knowledgeID, fileID).
		Delete(&agentdb.KnowledgeDocumentChunk{}).Error; err != nil {
		return code.New(code.InternalError, "clear old chunks failed: "+err.Error())
	}
	for idx, chunk := range chunks {
		meta := map[string]any{
			"storageSettingId": storageSettingID,
			"chunkNo":          idx + 1,
		}
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

func (s *AgentService) markChunkEmbeddedAndIndexed(ctx context.Context, knowledgeID int64, fileID string) error {
	var chunks []agentdb.KnowledgeDocumentChunk
	if err := s.runSvc.db.WithContext(ctx).
		Where("knowledge_base_id = ? AND file_id = ?", knowledgeID, fileID).
		Find(&chunks).Error; err != nil {
		return code.New(code.InternalError, "load chunks failed: "+err.Error())
	}
	for _, row := range chunks {
		vectorID := fmt.Sprintf("vec_%d_%d", row.KnowledgeBaseID, row.ID)
		if err := s.runSvc.db.WithContext(ctx).
			Model(&agentdb.KnowledgeDocumentChunk{}).
			Where("id = ?", row.ID).
			Updates(map[string]any{"vector_id": vectorID}).Error; err != nil {
			return code.New(code.InternalError, "mark vector failed: "+err.Error())
		}
	}
	return nil
}

func (s *AgentService) updateKnowledgeFileStatus(ctx context.Context, id int64, parse, chunk, embed, index string) error {
	err := s.runSvc.db.WithContext(ctx).Model(&agentdb.KnowledgeFile{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"parse_status": parse,
			"chunk_status": chunk,
			"embed_status": embed,
			"index_status": index,
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
	parse := agentapi.KnowledgeFileDetailParseStatus(row.ParseStatus)
	chunk := agentapi.KnowledgeFileDetailChunkStatus(row.ChunkStatus)
	embed := agentapi.KnowledgeFileDetailEmbedStatus(row.EmbedStatus)
	index := agentapi.KnowledgeFileDetailIndexStatus(row.IndexStatus)
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
		return "", errors.New("file service unavailable")
	}
	rc, _, item, err := s.fileSvc.OpenPreviewContent(ctx, fileID)
	if err != nil {
		// 二进制或无法读取对象时，至少用文件名作为可检索内容兜底。
		if item != nil && strings.TrimSpace(item.Name) != "" {
			return "FILE_NAME: " + item.Name, nil
		}
		return "", err
	}
	defer rc.Close()
	raw, readErr := io.ReadAll(io.LimitReader(rc, 2*1024*1024))
	if readErr != nil {
		return "", readErr
	}
	text := strings.TrimSpace(string(raw))
	if text == "" && item != nil {
		return "FILE_NAME: " + item.Name, nil
	}
	return text, nil
}

func splitChunks(content string, maxLen int) []string {
	clean := strings.TrimSpace(content)
	if clean == "" {
		return nil
	}
	if maxLen <= 0 {
		maxLen = 700
	}
	runes := []rune(clean)
	if len(runes) <= maxLen {
		return []string{clean}
	}
	result := make([]string, 0, len(runes)/maxLen+1)
	for i := 0; i < len(runes); i += maxLen {
		end := i + maxLen
		if end > len(runes) {
			end = len(runes)
		}
		result = append(result, string(runes[i:end]))
	}
	return result
}
