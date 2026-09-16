package routes

import (
	"dev-bridge-manager/internal/handlers"
	"dev-bridge-manager/internal/middleware"

	"github.com/gofiber/fiber/v2"
)

func SetupKanbanRoutes(api fiber.Router) {
	kanbanHandler := handlers.NewKanbanHandler()
	boardHandler := handlers.NewBoardHandler()
	taskHandler := handlers.NewTaskHandler()
	commentHandler := handlers.NewTaskCommentHandler()
	timeEntryHandler := handlers.NewTaskTimeEntryHandler()
	activityLogHandler := handlers.NewActivityLogHandler()

	projects := api.Group("/projects")
	projects.Use(middleware.JWTMiddleware())

	// Board entity CRUD
	projects.Get("/:id/boards", boardHandler.ListBoards)
	projects.Post("/:id/boards", middleware.RequirePermission("kanban.manage_columns"), boardHandler.CreateBoard)
	projects.Put("/:id/boards/:boardId", middleware.RequirePermission("kanban.manage_columns"), boardHandler.UpdateBoard)
	projects.Delete("/:id/boards/:boardId", middleware.RequirePermission("kanban.manage_columns"), boardHandler.DeleteBoard)

	// Board content (columns)
	projects.Get("/:id/boards/:boardId/kanban", kanbanHandler.GetBoard)
	projects.Put("/:id/boards/:boardId/kanban", middleware.RequirePermission("kanban.manage_columns"), kanbanHandler.UpdateBoard)
	projects.Post("/:id/boards/:boardId/kanban/columns", middleware.RequirePermission("kanban.manage_columns"), kanbanHandler.CreateColumn)
	projects.Put("/:id/boards/:boardId/kanban/columns/reorder", middleware.RequirePermission("kanban.manage_columns"), kanbanHandler.ReorderColumns)
	projects.Put("/:id/boards/:boardId/kanban/columns/:columnId", middleware.RequirePermission("kanban.manage_columns"), kanbanHandler.UpdateColumn)
	projects.Delete("/:id/boards/:boardId/kanban/columns/:columnId", middleware.RequirePermission("kanban.manage_columns"), kanbanHandler.DeleteColumn)

	// Placement management (board-scoped)
	projects.Post("/:id/boards/:boardId/tasks/:taskId/place", middleware.RequirePermission("tasks.move"), taskHandler.PlaceTask)
	projects.Put("/:id/boards/:boardId/tasks/:taskId/move", middleware.RequirePermission("tasks.move"), taskHandler.MoveTask)
	projects.Delete("/:id/boards/:boardId/tasks/:taskId", middleware.RequirePermission("tasks.delete"), taskHandler.RemovePlacement)

	// Project-scoped tasks (board-independent)
	projects.Get("/:id/tasks", taskHandler.GetTasks)
	projects.Get("/:id/tasks/:taskId", taskHandler.GetTask)
	projects.Get("/:id/tasks/:taskId/subtasks", taskHandler.GetSubtasks)
	projects.Post("/:id/tasks", middleware.RequirePermission("tasks.create"), taskHandler.CreateTask)
	projects.Put("/:id/tasks/:taskId", middleware.RequirePermission("tasks.update"), taskHandler.UpdateTask)
	projects.Delete("/:id/tasks/:taskId", middleware.RequirePermission("tasks.delete"), taskHandler.DeleteTask)

	// Comments
	projects.Get("/:id/tasks/:taskId/comments", commentHandler.GetComments)
	projects.Get("/:id/tasks/:taskId/history", activityLogHandler.GetTaskHistory)
	projects.Post("/:id/tasks/:taskId/comments", commentHandler.CreateComment)
	projects.Put("/:id/comments/:commentId", commentHandler.UpdateComment)
	projects.Delete("/:id/comments/:commentId", commentHandler.DeleteComment)

	// Time entries
	projects.Get("/:id/time-entries", middleware.RequirePermission("time_tracking.view"), timeEntryHandler.GetProjectTimeEntries)
	projects.Get("/:id/tasks/:taskId/time-entries", middleware.RequirePermission("time_tracking.view"), timeEntryHandler.GetTaskTimeEntries)
	projects.Post("/:id/tasks/:taskId/time-entries", middleware.RequirePermission("time_tracking.edit"), timeEntryHandler.CreateTimeEntry)
	projects.Put("/:id/time-entries/:entryId", middleware.RequirePermission("time_tracking.edit"), timeEntryHandler.UpdateTimeEntry)
	projects.Delete("/:id/time-entries/:entryId", middleware.RequirePermission("time_tracking.edit"), timeEntryHandler.DeleteTimeEntry)
}
