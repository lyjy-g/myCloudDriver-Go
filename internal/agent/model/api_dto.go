package model

import "time"

type CreateKnowledgeRequest struct {
	Description *string `json:"description,omitempty"`
	Name        string  `json:"name"`
}

type AddKnowledgeFileRequest struct {
	FileId           string  `json:"fileId"`
	StorageSettingId *string `json:"storageSettingId,omitempty"`
}

type KnowledgeFileDetail struct {
	ChunkStatus *string    `json:"chunkStatus,omitempty"`
	CreatedAt   *time.Time `json:"createdAt,omitempty"`
	EmbedStatus *string    `json:"embedStatus,omitempty"`
	FileId      *string    `json:"fileId,omitempty"`
	FileName    *string    `json:"fileName,omitempty"`
	Id          *int       `json:"id,omitempty"`
	IndexStatus *string    `json:"indexStatus,omitempty"`
	ParseStatus *string    `json:"parseStatus,omitempty"`
}
