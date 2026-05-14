package service

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
)

// ExecuteIdempotent 在 file service 内执行进程内幂等控制，不依赖外部幂等表/独立服务。
func (svc *FileService) ExecuteIdempotent(endpoint, idemKey string, requestBody []byte, execute func() (int, any, error)) (statusCode int, response any, replayed bool, err error) {
	idemKey = strings.TrimSpace(idemKey)
	if idemKey == "" {
		statusCode, response, err = execute()
		return statusCode, response, false, err
	}

	reqHash := hashRequest(endpoint, requestBody)
	recordKey := endpoint + "|" + idemKey
	now := time.Now()

	svc.idemMu.Lock()
	if rec, ok := svc.idemRecords[recordKey]; ok {
		if now.After(rec.ExpireAt) {
			delete(svc.idemRecords, recordKey)
		} else {
			if rec.RequestHash != reqHash {
				svc.idemMu.Unlock()
				return 0, nil, false, ErrIdempotencyConflict
			}
			if rec.Processing {
				svc.idemMu.Unlock()
				return 0, nil, false, ErrIdempotencyInProgress
			}
			var replay any
			if len(rec.ResponseRaw) > 0 {
				if unmarshalErr := json.Unmarshal(rec.ResponseRaw, &replay); unmarshalErr != nil {
					replay = map[string]any{"raw": string(rec.ResponseRaw)}
				}
			}
			svc.idemMu.Unlock()
			return rec.StatusCode, replay, true, nil
		}
	}

	svc.idemRecords[recordKey] = idempotencyRecord{
		RequestHash: reqHash,
		Processing:  true,
		ExpireAt:    now.Add(svc.idemTTL),
	}
	svc.idemMu.Unlock()

	statusCode, response, err = execute()
	if err != nil {
		svc.idemMu.Lock()
		delete(svc.idemRecords, recordKey)
		svc.idemMu.Unlock()
		return statusCode, response, false, err
	}

	raw, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		raw = []byte(`{"code":"OK","message":"success","data":{"marshal":"failed"}}`)
	}

	svc.idemMu.Lock()
	svc.idemRecords[recordKey] = idempotencyRecord{
		RequestHash: reqHash,
		Processing:  false,
		StatusCode:  statusCode,
		ResponseRaw: raw,
		ExpireAt:    time.Now().Add(svc.idemTTL),
	}
	svc.idemMu.Unlock()

	return statusCode, response, false, nil
}

func hashRequest(endpoint string, body []byte) string {
	h := sha256.New()
	h.Write([]byte(endpoint))
	h.Write([]byte("|"))
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}
