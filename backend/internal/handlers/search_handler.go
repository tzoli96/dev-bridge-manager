// backend/internal/handlers/search_handler.go
package handlers

import (
	"strings"

	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"

	"github.com/gofiber/fiber/v2"
)

const searchResultLimit = 5

type SearchHandler struct{}

func NewSearchHandler() *SearchHandler {
	return &SearchHandler{}
}

// ilikePattern builds a case-insensitive "contains" pattern for a raw
// search query, escaping ILIKE's own wildcard characters so a query like
// "50%" or "a_b" is matched literally rather than as a pattern.
func ilikePattern(q string) string {
	escaped := strings.NewReplacer("%", "\\%", "_", "\\_").Replace(q)
	return "%" + escaped + "%"
}

// Search - GET /api/v1/search?q=... - Cégszintű gyorskeresés ügyfelek,
// projektek és feladatok címében/nevében, mindenkinek aki be van
// jelentkezve (ugyanaz a szabály, mint a GetAllClients/GetAllProjects
// listázóknál).
func (h *SearchHandler) Search(c *fiber.Ctx) error {
	q := strings.TrimSpace(c.Query("q"))
	if len(q) < 2 {
		return c.JSON(models.SearchResponse{Success: true, Clients: []models.SearchClientResult{}, Projects: []models.SearchProjectResult{}, Tasks: []models.SearchTaskResult{}})
	}

	pattern := ilikePattern(q)
	db := database.GetDB()

	var clients []models.SearchClientResult
	if err := db.Table("clients").
		Select("id, name").
		Where("name ILIKE ? OR email ILIKE ?", pattern, pattern).
		Order("name ASC").
		Limit(searchResultLimit).
		Scan(&clients).Error; err != nil {
		return c.Status(500).JSON(models.SearchResponse{Success: false, Message: "Error searching clients"})
	}

	var projects []models.SearchProjectResult
	if err := db.Table("projects").
		Select("id, name").
		Where("name ILIKE ?", pattern).
		Order("name ASC").
		Limit(searchResultLimit).
		Scan(&projects).Error; err != nil {
		return c.Status(500).JSON(models.SearchResponse{Success: false, Message: "Error searching projects"})
	}

	var tasks []models.SearchTaskResult
	if err := db.Table("tasks").
		Select("tasks.id as id, tasks.title as title, tasks.project_id as project_id, projects.name as project_name").
		Joins("JOIN projects ON projects.id = tasks.project_id").
		Where("tasks.title ILIKE ? AND tasks.is_archived = false", pattern).
		Order("tasks.title ASC").
		Limit(searchResultLimit).
		Scan(&tasks).Error; err != nil {
		return c.Status(500).JSON(models.SearchResponse{Success: false, Message: "Error searching tasks"})
	}

	return c.JSON(models.SearchResponse{Success: true, Clients: clients, Projects: projects, Tasks: tasks})
}
