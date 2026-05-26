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
		// 没带幂等键时直接执行业务逻辑，不额外引入幂等约束。
		statusCode, response, err = execute()
		return statusCode, response, false, err
	}

	// 幂等键只是请求组键，请求体摘要还要参与比较，防止同 key 不同 payload 混用。
	reqHash := hashRequest(endpoint, requestBody)
	recordKey := endpoint + "|" + idemKey
	now := time.Now()

	svc.idemMu.Lock()
	if rec, ok := svc.idemRecords[recordKey]; ok {
		if now.After(rec.ExpireAt) {
			// 过期记录先删掉，再按新请求重新建立幂等态。
			delete(svc.idemRecords, recordKey)
		} else {
			if rec.RequestHash != reqHash {
				svc.idemMu.Unlock()
				return 0, nil, false, ErrIdempotencyConflict
			}
			if rec.Processing {
				// 同 key 同 payload 但还在处理中时直接返回“进行中”，避免并发重入。
				svc.idemMu.Unlock()
				return 0, nil, false, ErrIdempotencyInProgress
			}
			var replay any
			if len(rec.ResponseRaw) > 0 {
				// 已完成请求优先回放缓存响应，保证客户端重复提交看到同一结果。
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

	// 真正的业务执行放到锁外，避免长耗时接口把整个幂等表都阻塞住。
	statusCode, response, err = execute()
	if err != nil {
		// 执行失败时删掉 processing 记录，允许客户端后续重新发起。
		svc.idemMu.Lock()
		delete(svc.idemRecords, recordKey)
		svc.idemMu.Unlock()
		return statusCode, response, false, err
	}

	raw, marshalErr := json.Marshal(response)
	if marshalErr != nil {
		// 序列化失败时用兜底响应占位，至少保证后续回放不为空。
		raw = []byte(`{"code":"OK","message":"success","data":{"marshal":"failed"}}`)
	}

	svc.idemMu.Lock()
	// 业务成功后把响应缓存下来，后续同 key 重放直接返回这份结果。
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
	// 请求摘要把 endpoint 和 body 一起参与哈希，避免不同接口之间 key 冲突。
	h := sha256.New()
	h.Write([]byte(endpoint))
	h.Write([]byte("|"))
	h.Write(body)
	return hex.EncodeToString(h.Sum(nil))
}
