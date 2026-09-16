// handlers/task_handler.go
package handlers

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type TaskHandler struct{}

func NewTaskHandler() *TaskHandler {
	return &TaskHandler{}
}

func currentUserID(c *fiber.Ctx) uint {
	if id, ok := c.Locals("userID").(uint); ok {
		return id
	}
	return 0
}

// GetTasks - GET /api/v1/projects/:id/tasks
func (h *TaskHandler) GetTasks(c *fiber.Ctx) error {
	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid project ID"})
	}

	tasks, err := loadTaskDTOs(uint(projectID))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading tasks"})
	}

	return c.JSON(tasks)
}

// GetTask - GET /api/v1/projects/:id/tasks/:taskId
func (h *TaskHandler) GetTask(c *fiber.Ctx) error {
	taskID, err := strconv.Atoi(c.Params("taskId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid task ID"})
	}

	var task models.Task
	if err := database.GetDB().First(&task, taskID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Task not found"})
	}

	return c.JSON(loadSingleTaskDTO(task, nil))
}

// GetSubtasks - GET /api/v1/projects/:id/tasks/:taskId/subtasks
func (h *TaskHandler) GetSubtasks(c *fiber.Ctx) error {
	taskID, err := strconv.Atoi(c.Params("taskId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid task ID"})
	}

	var parent models.Task
	if err := database.GetDB().First(&parent, taskID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Task not found"})
	}

	dtos, err := loadSubtaskDTOs(uint(taskID))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading subtasks"})
	}

	return c.JSON(dtos)
}

// CreateTask - POST /api/v1/projects/:id/tasks
func (h *TaskHandler) CreateTask(c *fiber.Ctx) error {
	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid project ID"})
	}

	var req models.CreateTaskRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if req.Title == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Title is required"})
	}

	boardID, err := models.StrToID(req.BoardID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid board ID"})
	}
	columnID, err := models.StrToID(req.ColumnID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid column ID"})
	}

	priority := req.Priority
	if priority == "" {
		priority = "medium"
	}

	var estimatedHours float64
	if req.EstimatedHours != nil {
		estimatedHours = *req.EstimatedHours
	}

	dueDate, err := models.ParseDate(req.DueDate)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid due date"})
	}

	var assigneeID *uint
	if req.AssigneeID != "" {
		id, err := models.StrToID(req.AssigneeID)
		if err == nil {
			assigneeID = &id
		}
	}

	var parentTaskID *uint
	if req.ParentTaskID != "" {
		id, err := models.StrToID(req.ParentTaskID)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid parent task ID"})
		}
		var parent models.Task
		if err := database.GetDB().First(&parent, id).Error; err != nil {
			return c.Status(400).JSON(fiber.Map{"success": false, "message": "Parent task not found"})
		}
		if parent.ProjectID != uint(projectID) {
			return c.Status(400).JSON(fiber.Map{"success": false, "message": "Parent task must be in the same project"})
		}
		if parent.ParentTaskID != nil {
			return c.Status(400).JSON(fiber.Map{"success": false, "message": "A subtask cannot itself have subtasks"})
		}
		parentTaskID = &id
	}

	userID := currentUserID(c)

	task := models.Task{
		ProjectID:       uint(projectID),
		Title:           req.Title,
		Description:     req.Description,
		HTMLDescription: req.HTMLDescription,
		Priority:        priority,
		Status:          "todo",
		AssigneeID:      assigneeID,
		ParentTaskID:    parentTaskID,
		EstimatedHours:  estimatedHours,
		Tags:            models.TagsToJSON(req.Tags),
		DueDate:         dueDate,
		CreatedBy:       userID,
		UpdatedBy:       userID,
	}
	if err := database.GetDB().Create(&task).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error creating task"})
	}

	var maxPosition struct{ Max int }
	database.GetDB().Model(&models.TaskPlacement{}).
		Select("COALESCE(MAX(position), -1) as max").
		Where("column_id = ?", columnID).
		Scan(&maxPosition)

	placement := models.TaskPlacement{
		TaskID:   task.ID,
		BoardID:  boardID,
		ColumnID: columnID,
		Position: maxPosition.Max + 1,
	}
	if err := database.GetDB().Create(&placement).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error placing task"})
	}

	if parentTaskID != nil {
		logActivity(*parentTaskID, userID, "subtask_added", "", "", task.Title)
	}

	return c.Status(201).JSON(loadSingleTaskDTO(task, &placement))
}

// UpdateTask - PUT /api/v1/projects/:id/tasks/:taskId
func (h *TaskHandler) UpdateTask(c *fiber.Ctx) error {
	taskID, err := strconv.Atoi(c.Params("taskId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid task ID"})
	}

	var task models.Task
	if err := database.GetDB().First(&task, taskID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Task not found"})
	}
	original := task

	var req models.UpdateTaskRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}

	if req.Title != nil {
		task.Title = *req.Title
	}
	if req.Description != nil {
		task.Description = *req.Description
	}
	if req.HTMLDescription != nil {
		task.HTMLDescription = *req.HTMLDescription
	}
	if req.Priority != nil {
		task.Priority = *req.Priority
	}
	if req.AssigneeID != nil {
		if *req.AssigneeID == "" {
			task.AssigneeID = nil
		} else if id, err := models.StrToID(*req.AssigneeID); err == nil {
			task.AssigneeID = &id
		}
	}
	if req.EstimatedHours != nil {
		task.EstimatedHours = *req.EstimatedHours
	}
	if req.Tags != nil {
		task.Tags = models.TagsToJSON(*req.Tags)
	}
	if req.DueDate != nil {
		dueDate, err := models.ParseDate(*req.DueDate)
		if err != nil {
			return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid due date"})
		}
		task.DueDate = dueDate
	}
	task.UpdatedBy = currentUserID(c)

	if err := database.GetDB().Save(&task).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error updating task"})
	}
	logTaskFieldChanges(original, task, currentUserID(c))

	return c.JSON(loadSingleTaskDTO(task, nil))
}

// DeleteTask - DELETE /api/v1/projects/:id/tasks/:taskId
func (h *TaskHandler) DeleteTask(c *fiber.Ctx) error {
	taskID, err := strconv.Atoi(c.Params("taskId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid task ID"})
	}

	if err := database.GetDB().Delete(&models.Task{}, taskID).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error deleting task"})
	}

	return c.JSON(fiber.Map{"success": true, "message": "Task deleted successfully"})
}

// MoveTask - PUT /api/v1/projects/:id/boards/:boardId/tasks/:taskId/move
func (h *TaskHandler) MoveTask(c *fiber.Ctx) error {
	boardID, err := strconv.Atoi(c.Params("boardId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid board ID"})
	}
	taskID, err := strconv.Atoi(c.Params("taskId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid task ID"})
	}

	var task models.Task
	if err := database.GetDB().First(&task, taskID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Task not found"})
	}

	var placement models.TaskPlacement
	if err := database.GetDB().Where("task_id = ? AND board_id = ?", taskID, boardID).First(&placement).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Task is not placed on this board"})
	}

	var req models.MoveTaskRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	columnID, err := models.StrToID(req.ColumnID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid column ID"})
	}

	db := database.GetDB()
	oldColumnID := placement.ColumnID
	oldPosition := placement.Position

	if oldColumnID == columnID {
		// Reordering within the same column.
		if req.Position > oldPosition {
			db.Model(&models.TaskPlacement{}).
				Where("column_id = ? AND position > ? AND position <= ?", columnID, oldPosition, req.Position).
				Update("position", gorm.Expr("position - 1"))
		} else if req.Position < oldPosition {
			db.Model(&models.TaskPlacement{}).
				Where("column_id = ? AND position >= ? AND position < ?", columnID, req.Position, oldPosition).
				Update("position", gorm.Expr("position + 1"))
		}
	} else {
		// Moving to a different column: close the gap in the old column, open a gap in the new one.
		db.Model(&models.TaskPlacement{}).
			Where("column_id = ? AND position > ?", oldColumnID, oldPosition).
			Update("position", gorm.Expr("position - 1"))
		db.Model(&models.TaskPlacement{}).
			Where("column_id = ? AND position >= ?", columnID, req.Position).
			Update("position", gorm.Expr("position + 1"))
	}

	placement.ColumnID = columnID
	placement.Position = req.Position
	if err := db.Save(&placement).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error moving task"})
	}

	task.UpdatedBy = currentUserID(c)
	db.Save(&task)

	var cols []models.KanbanColumn
	db.Where("id IN ?", []uint{oldColumnID, columnID}).Find(&cols)
	titleByID := make(map[uint]string, len(cols))
	for _, col := range cols {
		titleByID[col.ID] = col.Title
	}
	if oldColumnID != columnID {
		logActivity(task.ID, currentUserID(c), "moved", "", titleByID[oldColumnID], titleByID[columnID])
	}

	return c.JSON(loadSingleTaskDTO(task, &placement))
}

// PlaceTask - POST /api/v1/projects/:id/boards/:boardId/tasks/:taskId/place
func (h *TaskHandler) PlaceTask(c *fiber.Ctx) error {
	boardID, err := strconv.Atoi(c.Params("boardId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid board ID"})
	}
	taskID, err := strconv.Atoi(c.Params("taskId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid task ID"})
	}

	var task models.Task
	if err := database.GetDB().First(&task, taskID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Task not found"})
	}

	var req models.PlaceTaskRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	columnID, err := models.StrToID(req.ColumnID)
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid column ID"})
	}

	var existing models.TaskPlacement
	if err := database.GetDB().Where("task_id = ? AND board_id = ?", taskID, boardID).First(&existing).Error; err == nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Task is already placed on this board"})
	}

	var maxPosition struct{ Max int }
	database.GetDB().Model(&models.TaskPlacement{}).
		Select("COALESCE(MAX(position), -1) as max").
		Where("column_id = ?", columnID).
		Scan(&maxPosition)

	placement := models.TaskPlacement{
		TaskID:   uint(taskID),
		BoardID:  uint(boardID),
		ColumnID: columnID,
		Position: maxPosition.Max + 1,
	}
	if err := database.GetDB().Create(&placement).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error placing task"})
	}

	return c.Status(201).JSON(loadSingleTaskDTO(task, &placement))
}

// RemovePlacement - DELETE /api/v1/projects/:id/boards/:boardId/tasks/:taskId
// Removes the task's placement from this board only. If this was the
// task's last placement, the task itself is deleted (comments/time
// entries cascade via existing FKs); otherwise only the placement is
// removed and the task survives on its other boards.
func (h *TaskHandler) RemovePlacement(c *fiber.Ctx) error {
	boardID, err := strconv.Atoi(c.Params("boardId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid board ID"})
	}
	taskID, err := strconv.Atoi(c.Params("taskId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid task ID"})
	}

	db := database.GetDB()

	var placement models.TaskPlacement
	if err := db.Where("task_id = ? AND board_id = ?", taskID, boardID).First(&placement).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Task is not placed on this board"})
	}

	if err := db.Delete(&placement).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error removing task from board"})
	}

	var remaining int64
	db.Model(&models.TaskPlacement{}).Where("task_id = ?", taskID).Count(&remaining)
	if remaining == 0 {
		if err := db.Delete(&models.Task{}, taskID).Error; err != nil {
			return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error deleting task"})
		}
	}

	return c.JSON(fiber.Map{"success": true, "message": "Task removed from board successfully"})
}
