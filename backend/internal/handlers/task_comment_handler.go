// handlers/task_comment_handler.go
package handlers

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type TaskCommentHandler struct{}

func NewTaskCommentHandler() *TaskCommentHandler {
	return &TaskCommentHandler{}
}

// GetComments - GET /api/v1/projects/:id/tasks/:taskId/comments
func (h *TaskCommentHandler) GetComments(c *fiber.Ctx) error {
	taskID, err := strconv.Atoi(c.Params("taskId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid task ID"})
	}

	var comments []models.TaskComment
	if err := database.GetDB().Where("task_id = ?", taskID).Order("created_at ASC").Find(&comments).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading comments"})
	}

	userIDs := make([]uint, 0, len(comments))
	for _, cm := range comments {
		userIDs = append(userIDs, cm.UserID)
	}
	users := loadUsersByIDs(userIDs)

	dtos := make([]models.TaskCommentDTO, 0, len(comments))
	for _, cm := range comments {
		dtos = append(dtos, buildCommentDTO(cm, users))
	}

	return c.JSON(dtos)
}

// CreateComment - POST /api/v1/projects/:id/tasks/:taskId/comments
func (h *TaskCommentHandler) CreateComment(c *fiber.Ctx) error {
	taskID, err := strconv.Atoi(c.Params("taskId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid task ID"})
	}

	var req models.CreateCommentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if req.Content == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Content is required"})
	}

	comment := models.TaskComment{
		TaskID:      uint(taskID),
		Content:     req.Content,
		HTMLContent: req.HTMLContent,
		UserID:      currentUserID(c),
	}
	if err := database.GetDB().Create(&comment).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error creating comment"})
	}

	users := loadUsersByIDs([]uint{comment.UserID})
	return c.Status(201).JSON(buildCommentDTO(comment, users))
}

// UpdateComment - PUT /api/v1/projects/:id/comments/:commentId
func (h *TaskCommentHandler) UpdateComment(c *fiber.Ctx) error {
	commentID, err := strconv.Atoi(c.Params("commentId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid comment ID"})
	}

	var comment models.TaskComment
	if err := database.GetDB().First(&comment, commentID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Comment not found"})
	}

	var req models.UpdateCommentRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if req.Content == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Content is required"})
	}

	comment.Content = req.Content
	comment.HTMLContent = req.HTMLContent
	comment.IsEdited = true

	if err := database.GetDB().Save(&comment).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error updating comment"})
	}

	users := loadUsersByIDs([]uint{comment.UserID})
	return c.JSON(buildCommentDTO(comment, users))
}

// DeleteComment - DELETE /api/v1/projects/:id/comments/:commentId
func (h *TaskCommentHandler) DeleteComment(c *fiber.Ctx) error {
	commentID, err := strconv.Atoi(c.Params("commentId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid comment ID"})
	}

	if err := database.GetDB().Delete(&models.TaskComment{}, commentID).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error deleting comment"})
	}

	return c.JSON(fiber.Map{"success": true, "message": "Comment deleted successfully"})
}
