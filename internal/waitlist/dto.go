package waitlist

type JoinRequest struct {
	Email  string `json:"email"`
	Source string `json:"source"`
}

type JoinResponse struct {
	Email         string `json:"email"`
	JoinedAt      string `json:"joined_at"`
	AlreadyJoined bool   `json:"already_joined"`
}
