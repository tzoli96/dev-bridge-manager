package handlers

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type BoardHandler struct{}

func NewBoardHandler() *BoardHandler {
	return &BoardHandler{}
}

func boardDTO(b models.Board) models.BoardDTO {
	return models.BoardDTO{
		ID:        models.IDToStr(b.ID),
		ProjectID: models.IDToStr(b.ProjectID),
		Name:      b.Name,
		Position:  b.Position,
		CreatedAt: b.CreatedAt,
		UpdatedAt: b.UpdatedAt,
	}
}

// ListBoards - GET /api/v1/projects/:id/boards
func (h *BoardHandler) ListBoards(c *fiber.Ctx) error {
	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid project ID"})
	}

	var boards []models.Board
	if err := database.GetDB().Where("project_id = ?", projectID).Order("position ASC").Find(&boards).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error loading boards"})
	}

	dtos := make([]models.BoardDTO, 0, len(boards))
	for _, b := range boards {
		dtos = append(dtos, boardDTO(b))
	}
	return c.JSON(dtos)
}

// CreateBoard - POST /api/v1/projects/:id/boards
// Seeds the same default columns used for a project's first board.
func (h *BoardHandler) CreateBoard(c *fiber.Ctx) error {
	projectID, err := strconv.Atoi(c.Params("id"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid project ID"})
	}

	var req models.CreateBoardRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if req.Name == "" {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Name is required"})
	}

	board := models.Board{
		ProjectID: uint(projectID),
		Name:      req.Name,
		Position:  req.Position,
	}
	if err := database.GetDB().Create(&board).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error creating board"})
	}

	if _, err := ensureColumns(board.ID); err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error seeding board columns"})
	}

	return c.Status(201).JSON(boardDTO(board))
}

// UpdateBoard - PUT /api/v1/projects/:id/boards/:boardId
func (h *BoardHandler) UpdateBoard(c *fiber.Ctx) error {
	boardID, err := strconv.Atoi(c.Params("boardId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid board ID"})
	}

	var board models.Board
	if err := database.GetDB().First(&board, boardID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Board not found"})
	}

	var req models.UpdateBoardRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid request body"})
	}
	if req.Name != nil {
		board.Name = *req.Name
	}
	if req.Position != nil {
		board.Position = *req.Position
	}

	if err := database.GetDB().Save(&board).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error updating board"})
	}

	return c.JSON(boardDTO(board))
}

// DeleteBoard - DELETE /api/v1/projects/:id/boards/:boardId
// Cascades (via FK) to the board's columns and their task_placements. Any
// task whose deleted placement was its last one is hard-deleted here.
func (h *BoardHandler) DeleteBoard(c *fiber.Ctx) error {
	boardID, err := strconv.Atoi(c.Params("boardId"))
	if err != nil {
		return c.Status(400).JSON(fiber.Map{"success": false, "message": "Invalid board ID"})
	}

	var board models.Board
	if err := database.GetDB().First(&board, boardID).Error; err != nil {
		return c.Status(404).JSON(fiber.Map{"success": false, "message": "Board not found"})
	}

	db := database.GetDB()

	var affectedTaskIDs []uint
	db.Model(&models.TaskPlacement{}).Where("board_id = ?", boardID).Pluck("task_id", &affectedTaskIDs)

	if err := db.Delete(&board).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"success": false, "message": "Error deleting board"})
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

	return c.JSON(fiber.Map{"success": true, "message": "Board deleted successfully"})
}
