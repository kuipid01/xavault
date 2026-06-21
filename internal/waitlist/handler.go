package waitlist

import (
	"errors"

	"github.com/gofiber/fiber/v2"
	"github.com/kuipid01/salary-vault/internal/shared/response"
)

type Handler struct {
	service *Service
}

func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) Join(c *fiber.Ctx) error {
	var req JoinRequest
	if err := c.BodyParser(&req); err != nil {
		return response.Error(c, fiber.StatusBadRequest, "Invalid request payload", "invalid_payload")
	}

	data, created, err := h.service.Join(req)
	if err != nil {
		if errors.Is(err, ErrInvalidEmail) {
			return response.Error(c, fiber.StatusUnprocessableEntity, err.Error(), "invalid_email")
		}
		return response.Error(c, fiber.StatusInternalServerError, "Could not join the waitlist", "waitlist_join_failed")
	}

	if !created {
		return response.Success(c, fiber.StatusOK, "You are already on the Xavault waitlist", data)
	}

	return response.Success(c, fiber.StatusCreated, "Welcome to the Xavault waitlist", data)
}
