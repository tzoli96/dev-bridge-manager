// handlers/task_comment_handler.go
package handlers

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"log"
	"os"
	"path/filepath"
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

	commentIDs := make([]uint, len(comments))
	userIDs := make([]uint, 0, len(comments))
	for i, cm := range comments {
		commentIDs[i] = cm.ID
		userIDs = append(userIDs, cm.UserID)
	}

	var attachments []models.Attachment
	if len(commentIDs) > 0 {
		database.GetDB().Where("comment_id IN ?", commentIDs).Order("created_at ASC").Find(&attachments)
	}
	attachmentsByComment := make(map[uint][]models.Attachment)
	for _, a := range attachments {
		attachmentsByComment[*a.CommentID] = append(attachmentsByComment[*a.CommentID], a)
		userIDs = append(userIDs, a.UploadedBy)
	}
	users := loadUsersByIDs(userIDs)
	projectID := taskProjectID(uint(taskID))

	dtos := make([]models.TaskCommentDTO, 0, len(comments))
	for _, cm := range comments {
		dtos = append(dtos, buildCommentDTO(cm, users, projectID, attachmentsByComment[cm.ID]))
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

	preview := commentPreview(comment.Content)
	logActivity(comment.TaskID, comment.UserID, "comment_added", "", "", preview)

	users := loadUsersByIDs([]uint{comment.UserID})
	projectID := taskProjectID(uint(taskID))
	return c.Status(201).JSON(buildCommentDTO(comment, users, projectID, nil))
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

	var attachments []models.Attachment
	database.GetDB().Where("comment_id = ?", comment.ID).Order("created_at ASC").Find(&attachments)

	userIDs := []uint{comment.UserID}
	for _, a := range attachments {
		userIDs = append(userIDs, a.UploadedBy)
	}
	users := loadUsersByIDs(userIDs)
	projectID := taskProjectID(comment.TaskID)
	return c.JSON(buildCommentDTO(comment, users, projectID, attachments))
}

// DeleteComment - DELETE /api/v1/projects/:id/comments/:commentId
func (h *TaskCommentHandler) DeleteComment(c *fiber.Ctx) error {
	commentID, err := strconv.Atoi(c.Params("commentId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid comment ID"})
	}

	var comment models.TaskComment
	database.GetDB().First(&comment, commentID)

	var attachments []models.Attachment
	database.GetDB().Where("comment_id = ?", commentID).Find(&attachments)

	if err := database.GetDB().Delete(&models.TaskComment{}, commentID).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error deleting comment"})
	}

	for _, a := range attachments {
		path := filepath.Join(uploadsBaseDir, "tasks", strconv.Itoa(int(a.TaskID)), a.Filename)
		if err := os.Remove(path); err != nil {
			log.Printf("⚠️ Failed to remove attachment file %s: %v", path, err)
		}
	}

	preview := commentPreview(comment.Content)
	logActivity(comment.TaskID, currentUserID(c), "comment_deleted", "", "", preview)

	return c.JSON(fiber.Map{"success": true, "message": "Comment deleted successfully"})
}
