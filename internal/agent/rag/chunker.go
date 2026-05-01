package rag

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

const (
	DefaultChunkSize    = 512
	DefaultChunkOverlap = 64
)

// Chunker 文本切片器。
type Chunker struct {
	chunkSize    int
	chunkOverlap int
}

func NewChunker(chunkSize, chunkOverlap int) *Chunker {
	if chunkSize <= 0 {
		chunkSize = DefaultChunkSize
	}
	if chunkOverlap < 0 {
		chunkOverlap = 0
	}
	if chunkOverlap >= chunkSize {
		chunkOverlap = chunkSize / 4
	}
	return &Chunker{chunkSize: chunkSize, chunkOverlap: chunkOverlap}
}

// Split 将文本切成多个 Chunk。
func (c *Chunker) Split(doc Document) []Chunk {
	runes := []rune(doc.Content)
	if len(runes) == 0 {
		return nil
	}
	chunks := make([]Chunk, 0)
	idx := 0
	pos := 0
	for start := 0; start < len(runes); start += c.chunkSize - c.chunkOverlap {
		end := start + c.chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		text := string(runes[start:end])
		startByte := utf8.RuneCountInString(string(runes[:start]))
		endByte := utf8.RuneCountInString(string(runes[:end]))
		chunks = append(chunks, Chunk{
			ID:         fmt.Sprintf("%s_chunk_%d", doc.ID, idx),
			DocumentID: doc.ID,
			Index:      idx,
			Text:       strings.TrimSpace(text),
			StartByte:  startByte,
			EndByte:    endByte,
			Metadata: map[string]string{
				"source":    doc.SourceKey,
				"file_name": doc.FileName,
				"file_type": doc.FileType,
			},
		})
		idx++
		pos = end
		if end >= len(runes) {
			break
		}
	}
	_ = pos
	return chunks
}
