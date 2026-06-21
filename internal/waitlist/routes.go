package waitlist

import "github.com/gofiber/fiber/v2"

func RegisterRoutes(v1 fiber.Router, handler *Handler) {
	v1.Post("/waitlist", handler.Join)
}
