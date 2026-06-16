package client

import "time"

type IdempotentKey struct {
	Key          string     `json:"key" gorm:"primaryKey"`
	UserId       string     `json:"user_id"`
	State        string     `json:"state"`
	ResponseCode *int       `json:"response_code"`
	ResponseBody *string    `json:"response_body"`
	CreatedAt    time.Time  `json:"created_at"`
	LockedUntil  *time.Time `json:"locked_until"`
}

//dto here since it is a small implementation

type IdempotentRequestDTO struct {
	UserID    string    `json:"user_id"`
	Key       string    `json:"key"`
	State     string    `json:"state"`
	CreatedAt time.Time `json:"created_at"`
}
