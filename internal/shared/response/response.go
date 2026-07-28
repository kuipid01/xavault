package response

import (
	"log"

	"github.com/gofiber/fiber/v2"
)

type APIResponse struct {
	Success          bool        `json:"success"`
	Message          string      `json:"message"`
	Data             interface{} `json:"data,omitempty"`
	Error            interface{} `json:"error,omitempty"`
	SupportReference string      `json:"support_reference,omitempty"`
}

func Success(c *fiber.Ctx, statusCode int, message string, data interface{}) error {

	return c.Status(statusCode).JSON(APIResponse{
		Success: true,
		Message: message,
		Data:    data,
	})
}
func Error(c *fiber.Ctx, statusCode int, message string, err interface{}) error {
	supportReference, _ := c.Locals("support_reference").(string)
	log.Printf("[API_ERROR] support_reference=%s status=%d method=%s path=%s message=%q", supportReference, statusCode, c.Method(), c.Path(), message)
	return c.Status(statusCode).JSON(APIResponse{
		Success:          false,
		Message:          message,
		Error:            err,
		SupportReference: supportReference,
	})
}
