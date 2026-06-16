package client

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"
)

type IdempotentRequest struct {
	repo      *IdempotentRepository // Use pointer reference
	EnableLog bool
}

func NewIdempotentRequest(repo *IdempotentRepository, enableLog bool) *IdempotentRequest {
	return &IdempotentRequest{repo: repo, EnableLog: enableLog}
}

func (r *IdempotentRequest) HandleIdempotentRequest(
	ctx context.Context,
	key string,
	userID string,
	executeLogic func() (int, interface{}, error),
) (int, interface{}, error) {
	if r.EnableLog {
		log.Printf("[Idempotency] HandleIdempotentRequest initiated: key=%s, userID=%s", key, userID)
	}

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
		if r.EnableLog {
			log.Printf("[Idempotency] Record insertion failed (likely exists) for key=%s", key)
		}
		// Unique key constraint violated -> Record already exists, fetch it
		existingRecord, findErr := r.repo.FindByKey(key)
		if findErr != nil {
			if r.EnableLog {
				log.Printf("[Idempotency] Failed to fetch existing record for key=%s: %v", key, findErr)
			}
			return http.StatusInternalServerError, map[string]string{"error": "Failed to resolve request status"}, findErr
		}

		if r.EnableLog {
			log.Printf("[Idempotency] Existing record found for key=%s, State=%s", key, existingRecord.State)
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
			if r.EnableLog {
				log.Printf("[Idempotency] Returning previously successful response for key=%s", key)
			}
			return code, savedResponse, nil
		}

		// Scenario C: Request failed previously. Flip back to IN_FLIGHT and retry logic execution.
		existingRecord.State = "IN_FLIGHT"
		if updateErr := r.repo.Update(existingRecord); updateErr != nil {
			if r.EnableLog {
				log.Printf("[Idempotency] Failed to update state to IN_FLIGHT for key=%s: %v", key, updateErr)
			}
			return http.StatusInternalServerError, map[string]string{"error": "Failed to lock transaction retry"}, updateErr
		}
		record = existingRecord
	} else {
		if r.EnableLog {
			log.Printf("[Idempotency] New record created for key=%s", key)
		}
	}

	// 2. Execute business logic safely
	if r.EnableLog {
		log.Printf("[Idempotency] Executing business logic for key=%s", key)
	}
	code, payload, err := executeLogic()
	if err != nil {
		if r.EnableLog {
			log.Printf("[Idempotency] Business logic failed for key=%s: %v", key, err)
		}
		// Logic failed! Mark state as FAILED so client can safely retry with the same key
		record.State = "FAILED"
		_ = r.repo.Update(record)
		return code, payload, err
	}

	if r.EnableLog {
		log.Printf("[Idempotency] Business logic successful for key=%s, finalizing state", key)
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
		if r.EnableLog {
			log.Printf("[Idempotency] Failed to finalize transaction caching for key=%s: %v", key, updateErr)
		}
		return http.StatusInternalServerError, map[string]string{"error": "Failed to finalize transaction caching"}, updateErr
	}

	if r.EnableLog {
		log.Printf("[Idempotency] Transaction completed and cached successfully for key=%s", key)
	}
	return code, payload, nil
}
