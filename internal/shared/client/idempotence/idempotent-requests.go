package client

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

type IdempotentRequest struct {
	repo *IdempotentRepository // Use pointer reference
}

func NewIdempotentRequest(repo *IdempotentRepository) *IdempotentRequest {
	return &IdempotentRequest{repo: repo}
}

func (r *IdempotentRequest) HandleIdempotentRequest(
	ctx context.Context,
	key string,
	userID string,
	executeLogic func() (int, interface{}, error),
) (int, interface{}, error) {
	now := time.Now()
	idempotentDTO := &IdempotentKey{
		Key:       key,
		UserId:    userID,
		State:     "IN_FLIGHT",
		CreatedAt: now,
	}

	// 1. Attempt atomic insertion
	record, err := r.repo.Create(idempotentDTO)
	if err != nil {
		// Unique key constraint violated -> Record already exists, fetch it
		existingRecord, findErr := r.repo.FindByKey(key)
		if findErr != nil {
			return http.StatusInternalServerError, map[string]string{"error": "Failed to resolve request status"}, findErr
		}

		// Scenario A: Request is currently running
		if existingRecord.State == "IN_FLIGHT" {
			return http.StatusConflict, map[string]string{"error": "An identical transaction is already processing"}, nil
		}

		// Scenario B: Request completed successfully on a previous try
		if existingRecord.State == "SUCCESS" {
			var savedResponse interface{}
			if existingRecord.ResponseBody != nil {
				_ = json.Unmarshal([]byte(*existingRecord.ResponseBody), &savedResponse)
			}

			code := http.StatusOK
			if existingRecord.ResponseCode != nil {
				code = *existingRecord.ResponseCode
			}
			return code, savedResponse, nil
		}

		// Scenario C: Request failed previously. Flip back to IN_FLIGHT and retry logic execution.
		existingRecord.State = "IN_FLIGHT"
		if updateErr := r.repo.Update(existingRecord); updateErr != nil {
			return http.StatusInternalServerError, map[string]string{"error": "Failed to lock transaction retry"}, updateErr
		}
		record = existingRecord
	}

	// 2. Execute business logic safely
	code, payload, err := executeLogic()
	if err != nil {
		// Logic failed! Mark state as FAILED so client can safely retry with the same key
		record.State = "FAILED"
		_ = r.repo.Update(record)
		return code, payload, err
	}

	// 3. Request completely successful! Serialize payload & save structural outcome
	jsonBytes, jsonErr := json.Marshal(payload)
	if jsonErr == nil {
		bodyStr := string(jsonBytes)
		record.ResponseBody = &bodyStr
	}
	record.ResponseCode = &code
	record.State = "SUCCESS"

	if updateErr := r.repo.Update(record); updateErr != nil {
		return http.StatusInternalServerError, map[string]string{"error": "Failed to finalize transaction caching"}, updateErr
	}

	return code, payload, nil
}
