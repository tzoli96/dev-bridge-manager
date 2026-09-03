// handlers/kanban_handler.go
package handlers

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type KanbanHandler struct{}

func NewKanbanHandler() *KanbanHandler {
	return &KanbanHandler{}
}

var defaultColumnSeed = []struct {
	Title string
	Color string
}{
	{"To Do", "bg-blue-500"},
	{"In Progress", "bg-yellow-500"},
	{"Review", "bg-purple-500"},
	{"Done", "bg-green-500"},
}

func intPtr(v int) *int { return &v }

// ensureColumns creates the default column set for a board the first time it's opened.
func ensureColumns(boardID uint) ([]models.KanbanColumn, error) {
	var columns []models.KanbanColumn
	if err := database.GetDB().Where("board_id = ?", boardID).Order("position ASC").Find(&columns).Error; err != nil {
		return nil, err
	}
	if len(columns) > 0 {
		return columns, nil
	}

	seeded := make([]models.KanbanColumn, 0, len(defaultColumnSeed))
	for i, c := range defaultColumnSeed {
		col := models.KanbanColumn{
			BoardID:  boardID,
			Title:    c.Title,
			Color:    c.Color,
			Position: i,
		}
		if i == 1 {
			col.MaxTasks = intPtr(5)
		}
		seeded = append(seeded, col)
	}
	if err := database.GetDB().Create(&seeded).Error; err != nil {
		return nil, err
	}
	return seeded, nil
}

func columnDTO(c models.KanbanColumn, tasks []models.TaskDTO) models.KanbanColumnDTO {
	colTasks := make([]models.TaskDTO, 0)
	for _, t := range tasks {
		if t.ColumnID == models.IDToStr(c.ID) {
			colTasks = append(colTasks, t)
		}
	}
	return models.KanbanColumnDTO{
		ID:        models.IDToStr(c.ID),
		Title:     c.Title,
		Color:     c.Color,
		Position:  c.Position,
		MaxTasks:  c.MaxTasks,
		Tasks:     colTasks,
		CreatedAt: c.CreatedAt,
		UpdatedAt: c.UpdatedAt,
	}
}

// GetBoard - GET /api/v1/projects/:id/boards/:boardId/kanban
func (h *KanbanHandler) GetBoard(c *fiber.Ctx) error {
	boardID, err := strconv.Atoi(c.Params("boardId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid board ID"})
	}

	var board models.Board
	if err := database.GetDB().First(&board, boardID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Board not found"})
	}

	columns, err := ensureColumns(uint(boardID))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading board"})
	}

	taskDTOs, err := loadBoardTaskDTOs(uint(boardID))
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading tasks"})
	}

	columnDTOs := make([]models.KanbanColumnDTO, 0, len(columns))
	for _, col := range columns {
		columnDTOs = append(columnDTOs, columnDTO(col, taskDTOs))
	}

	return c.JSON(models.KanbanBoardDTO{
		ID:        models.IDToStr(board.ID),
		ProjectID: models.IDToStr(board.ProjectID),
		Name:      board.Name,
		Columns:   columnDTOs,
		Settings:  models.DefaultKanbanSettings(),
		CreatedAt: board.CreatedAt,
		UpdatedAt: board.UpdatedAt,
	})
}

// UpdateBoard - PUT /api/v1/projects/:id/kanban
// The board has no persisted settings/title of its own (columns/tasks are what's
// real); this simply re-returns the current board so the frontend's optimistic
// update has something consistent to apply.
func (h *KanbanHandler) UpdateBoard(c *fiber.Ctx) error {
	return h.GetBoard(c)
}

// CreateColumn - POST /api/v1/projects/:id/boards/:boardId/kanban/columns
func (h *KanbanHandler) CreateColumn(c *fiber.Ctx) error {
	boardID, err := strconv.Atoi(c.Params("boardId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid board ID"})
	}

	var req models.CreateColumnRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if req.Title == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Title is required"})
	}
	if req.Color == "" {
		req.Color = "bg-gray-500"
	}

	column := models.KanbanColumn{
		BoardID:  uint(boardID),
		Title:    req.Title,
		Color:    req.Color,
		Position: req.Position,
		MaxTasks: req.MaxTasks,
	}
	if err := database.GetDB().Create(&column).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error creating column"})
	}

	return c.Status(201).JSON(columnDTO(column, []models.TaskDTO{}))
}

// UpdateColumn - PUT /api/v1/projects/:id/kanban/columns/:columnId
func (h *KanbanHandler) UpdateColumn(c *fiber.Ctx) error {
	columnID, err := strconv.Atoi(c.Params("columnId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid column ID"})
	}

	var column models.KanbanColumn
	if err := database.GetDB().First(&column, columnID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Column not found"})
	}

	var req models.UpdateColumnRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}

	if req.Title != nil {
		column.Title = *req.Title
	}
	if req.Color != nil {
		column.Color = *req.Color
	}
	if req.Position != nil {
		column.Position = *req.Position
	}
	if req.MaxTasks != nil {
		column.MaxTasks = req.MaxTasks
	}

	if err := database.GetDB().Save(&column).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error updating column"})
	}

	taskDTOs, _ := loadBoardTaskDTOs(column.BoardID)
	return c.JSON(columnDTO(column, taskDTOs))
}

// DeleteColumn - DELETE /api/v1/projects/:id/kanban/columns/:columnId
func (h *KanbanHandler) DeleteColumn(c *fiber.Ctx) error {
	columnID, err := strconv.Atoi(c.Params("columnId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid column ID"})
	}

	var column models.KanbanColumn
	if err := database.GetDB().First(&column, columnID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Column not found"})
	}

	db := database.GetDB()

	var affectedTaskIDs []uint
	db.Model(&models.TaskPlacement{}).Where("column_id = ?", columnID).Pluck("task_id", &affectedTaskIDs)

	if err := db.Delete(&column).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error deleting column"})
	}

	if len(affectedTaskIDs) > 0 {
		var stillPlaced []uint
		db.Model(&models.TaskPlacement{}).Where("task_id IN ?", affectedTaskIDs).Pluck("task_id", &stillPlaced)

		stillPlacedSet := make(map[uint]bool, len(stillPlaced))
		for _, id := range stillPlaced {
			stillPlacedSet[id] = true
		}

		orphaned := make([]uint, 0)
		for _, id := range affectedTaskIDs {
			if !stillPlacedSet[id] {
				orphaned = append(orphaned, id)
			}
		}
		if len(orphaned) > 0 {
			db.Delete(&models.Task{}, orphaned)
		}
	}

	return c.JSON(fiber.Map{"success": true, "message": "Column deleted successfully"})
}

// ReorderColumns - PUT /api/v1/projects/:id/kanban/columns/reorder
func (h *KanbanHandler) ReorderColumns(c *fiber.Ctx) error {
	var req models.ReorderColumnsRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}

	db := database.GetDB()
	for _, order := range req.Orders {
		columnID, err := models.StrToID(order.ColumnID)
		if err != nil {
			continue
		}
		db.Model(&models.KanbanColumn{}).Where("id = ?", columnID).Update("position", order.Position)
	}

	return c.JSON(fiber.Map{"success": true, "message": "Columns reordered successfully"})
}
