// handlers/kanban_dto_builder.go
package handlers

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
)

// loadUsersByIDs fetches the given user ids and returns them keyed by id.
func loadUsersByIDs(ids []uint) map[uint]models.User {
	result := make(map[uint]models.User)
	if len(ids) == 0 {
		return result
	}

	unique := make(map[uint]bool)
	filtered := make([]uint, 0, len(ids))
	for _, id := range ids {
		if id != 0 && !unique[id] {
			unique[id] = true
			filtered = append(filtered, id)
		}
	}
	if len(filtered) == 0 {
		return result
	}

	var users []models.User
	database.GetDB().Where("id IN ?", filtered).Find(&users)
	for _, u := range users {
		result[u.ID] = u
	}
	return result
}

func userRefDTO(userID uint, users map[uint]models.User) *models.TaskUserRefDTO {
	u, ok := users[userID]
	if !ok {
		return nil
	}
	return &models.TaskUserRefDTO{ID: models.IDToStr(u.ID), Name: u.Name, Avatar: ""}
}

func assigneeDTO(userID *uint, users map[uint]models.User) *models.TaskAssigneeDTO {
	if userID == nil {
		return nil
	}
	u, ok := users[*userID]
	if !ok {
		return nil
	}
	return &models.TaskAssigneeDTO{ID: models.IDToStr(u.ID), Name: u.Name, Email: u.Email, Avatar: ""}
}

func buildCommentDTO(c models.TaskComment, users map[uint]models.User, projectID uint, attachments []models.Attachment) models.TaskCommentDTO {
	attachmentDTOs := make([]models.AttachmentDTO, 0, len(attachments))
	for _, a := range attachments {
		attachmentDTOs = append(attachmentDTOs, buildAttachmentDTO(a, users, projectID))
	}
	return models.TaskCommentDTO{
		ID:          models.IDToStr(c.ID),
		TaskID:      models.IDToStr(c.TaskID),
		Content:     c.Content,
		HTMLContent: c.HTMLContent,
		UserID:      models.IDToStr(c.UserID),
		User:        userRefDTO(c.UserID, users),
		IsEdited:    c.IsEdited,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}

func buildTimeEntryDTO(e models.TaskTimeEntry, users map[uint]models.User) models.TimeEntryDTO {
	return models.TimeEntryDTO{
		ID:          models.IDToStr(e.ID),
		TaskID:      models.IDToStr(e.TaskID),
		Hours:       e.Hours,
		Description: e.Description,
		Date:        e.Date.Format("2006-01-02"),
		UserID:      models.IDToStr(e.UserID),
		User:        userRefDTO(e.UserID, users),
		CreatedAt:   e.CreatedAt,
		UpdatedAt:   e.UpdatedAt,
	}
}

func buildAttachmentDTO(a models.Attachment, users map[uint]models.User, projectID uint) models.AttachmentDTO {
	commentID := ""
	if a.CommentID != nil {
		commentID = models.IDToStr(*a.CommentID)
	}
	return models.AttachmentDTO{
		ID:           models.IDToStr(a.ID),
		TaskID:       models.IDToStr(a.TaskID),
		CommentID:    commentID,
		OriginalName: a.OriginalName,
		MimeType:     a.MimeType,
		Size:         a.Size,
		UploadedByID: models.IDToStr(a.UploadedBy),
		UploadedBy:   userRefDTO(a.UploadedBy, users),
		CreatedAt:    a.CreatedAt,
		DownloadUrl:  "/projects/" + models.IDToStr(projectID) + "/attachments/" + models.IDToStr(a.ID) + "/download",
	}
}

// taskProjectID looks up a task's project id (needed by call sites — comment
// handlers — that only have a task/comment id in scope, not the full Task row).
func taskProjectID(taskID uint) uint {
	var projectID uint
	database.GetDB().Model(&models.Task{}).Where("id = ?", taskID).Select("project_id").Scan(&projectID)
	return projectID
}

// buildTaskDTO assembles a full TaskDTO from a task row plus its already-loaded
// comments/time entries (both filtered to this task) and a shared user lookup map.
// placement is optional: when nil (project-scoped, placement-independent contexts)
// ColumnID/Position come back as the zero value ("" / 0); otherwise they're sourced
// from the given board placement.
func buildTaskDTO(t models.Task, placement *models.TaskPlacement, comments []models.TaskComment, entries []models.TaskTimeEntry, taskAttachments []models.Attachment, commentAttachments map[uint][]models.Attachment, users map[uint]models.User) models.TaskDTO {
	commentDTOs := make([]models.TaskCommentDTO, 0, len(comments))
	for _, c := range comments {
		commentDTOs = append(commentDTOs, buildCommentDTO(c, users, t.ProjectID, commentAttachments[c.ID]))
	}

	entryDTOs := make([]models.TimeEntryDTO, 0, len(entries))
	var loggedHours float64
	for _, e := range entries {
		entryDTOs = append(entryDTOs, buildTimeEntryDTO(e, users))
		loggedHours += e.Hours
	}

	attachmentDTOs := make([]models.AttachmentDTO, 0, len(taskAttachments))
	for _, a := range taskAttachments {
		attachmentDTOs = append(attachmentDTOs, buildAttachmentDTO(a, users, t.ProjectID))
	}

	columnID := ""
	position := 0
	if placement != nil {
		columnID = models.IDToStr(placement.ColumnID)
		position = placement.Position
	}

	return models.TaskDTO{
		ID:              models.IDToStr(t.ID),
		Title:           t.Title,
		Description:     t.Description,
		HTMLDescription: t.HTMLDescription,
		Priority:        t.Priority,
		Status:          t.Status,
		ColumnID:        columnID,
		ProjectID:       models.IDToStr(t.ProjectID),
		AssigneeID: func() string {
			if t.AssigneeID != nil {
				return models.IDToStr(*t.AssigneeID)
			}
			return ""
		}(),
		Assignee:       assigneeDTO(t.AssigneeID, users),
		EstimatedHours: t.EstimatedHours,
		LoggedHours:    loggedHours,
		Tags:           models.TagsFromJSON(t.Tags),
		TimeEntries:    entryDTOs,
		Comments:       commentDTOs,
		Attachments:    attachmentDTOs,
		Position:       position,
		DueDate:        models.FormatDate(t.DueDate),
		CreatedAt:      t.CreatedAt,
		UpdatedAt:      t.UpdatedAt,
		CreatedBy:      models.IDToStr(t.CreatedBy),
		UpdatedBy:      models.IDToStr(t.UpdatedBy),
	}
}

// loadTaskDTOs loads all (non-archived) tasks for a project together with their
// comments/time entries/users and returns fully-built DTOs.
func loadTaskDTOs(projectID uint) ([]models.TaskDTO, error) {
	var tasks []models.Task
	if err := database.GetDB().Where("project_id = ? AND is_archived = false", projectID).
		Order("id ASC").Find(&tasks).Error; err != nil {
		return nil, err
	}
	if len(tasks) == 0 {
		return []models.TaskDTO{}, nil
	}

	taskIDs := make([]uint, len(tasks))
	userIDs := make([]uint, 0, len(tasks)*2)
	for i, t := range tasks {
		taskIDs[i] = t.ID
		userIDs = append(userIDs, t.CreatedBy, t.UpdatedBy)
		if t.AssigneeID != nil {
			userIDs = append(userIDs, *t.AssigneeID)
		}
	}

	var comments []models.TaskComment
	database.GetDB().Where("task_id IN ?", taskIDs).Order("created_at ASC").Find(&comments)

	var entries []models.TaskTimeEntry
	database.GetDB().Where("task_id IN ?", taskIDs).Order("date DESC").Find(&entries)

	var attachments []models.Attachment
	database.GetDB().Where("task_id IN ?", taskIDs).Order("created_at ASC").Find(&attachments)

	for _, c := range comments {
		userIDs = append(userIDs, c.UserID)
	}
	for _, e := range entries {
		userIDs = append(userIDs, e.UserID)
	}
	for _, a := range attachments {
		userIDs = append(userIDs, a.UploadedBy)
	}
	users := loadUsersByIDs(userIDs)

	commentsByTask := make(map[uint][]models.TaskComment)
	for _, c := range comments {
		commentsByTask[c.TaskID] = append(commentsByTask[c.TaskID], c)
	}
	entriesByTask := make(map[uint][]models.TaskTimeEntry)
	for _, e := range entries {
		entriesByTask[e.TaskID] = append(entriesByTask[e.TaskID], e)
	}
	taskAttachmentsByTask := make(map[uint][]models.Attachment)
	commentAttachmentsByComment := make(map[uint][]models.Attachment)
	for _, a := range attachments {
		if a.CommentID == nil {
			taskAttachmentsByTask[a.TaskID] = append(taskAttachmentsByTask[a.TaskID], a)
		} else {
			commentAttachmentsByComment[*a.CommentID] = append(commentAttachmentsByComment[*a.CommentID], a)
		}
	}

	dtos := make([]models.TaskDTO, 0, len(tasks))
	for _, t := range tasks {
		dtos = append(dtos, buildTaskDTO(t, nil, commentsByTask[t.ID], entriesByTask[t.ID], taskAttachmentsByTask[t.ID], commentAttachmentsByComment, users))
	}
	return dtos, nil
}

// loadBoardTaskDTOs loads every task placed on a board, using each task's
// board-specific placement for its column/position.
func loadBoardTaskDTOs(boardID uint) ([]models.TaskDTO, error) {
	var placements []models.TaskPlacement
	if err := database.GetDB().Where("board_id = ?", boardID).Find(&placements).Error; err != nil {
		return nil, err
	}
	if len(placements) == 0 {
		return []models.TaskDTO{}, nil
	}

	placementByTask := make(map[uint]models.TaskPlacement, len(placements))
	taskIDs := make([]uint, 0, len(placements))
	for _, p := range placements {
		placementByTask[p.TaskID] = p
		taskIDs = append(taskIDs, p.TaskID)
	}

	var tasks []models.Task
	if err := database.GetDB().Where("id IN ? AND is_archived = false", taskIDs).Find(&tasks).Error; err != nil {
		return nil, err
	}

	userIDs := make([]uint, 0, len(tasks)*2)
	rowIDs := make([]uint, len(tasks))
	for i, t := range tasks {
		rowIDs[i] = t.ID
		userIDs = append(userIDs, t.CreatedBy, t.UpdatedBy)
		if t.AssigneeID != nil {
			userIDs = append(userIDs, *t.AssigneeID)
		}
	}

	var comments []models.TaskComment
	database.GetDB().Where("task_id IN ?", rowIDs).Order("created_at ASC").Find(&comments)

	var entries []models.TaskTimeEntry
	database.GetDB().Where("task_id IN ?", rowIDs).Order("date DESC").Find(&entries)

	var attachments []models.Attachment
	database.GetDB().Where("task_id IN ?", rowIDs).Order("created_at ASC").Find(&attachments)

	for _, c := range comments {
		userIDs = append(userIDs, c.UserID)
	}
	for _, e := range entries {
		userIDs = append(userIDs, e.UserID)
	}
	for _, a := range attachments {
		userIDs = append(userIDs, a.UploadedBy)
	}
	users := loadUsersByIDs(userIDs)

	commentsByTask := make(map[uint][]models.TaskComment)
	for _, c := range comments {
		commentsByTask[c.TaskID] = append(commentsByTask[c.TaskID], c)
	}
	entriesByTask := make(map[uint][]models.TaskTimeEntry)
	for _, e := range entries {
		entriesByTask[e.TaskID] = append(entriesByTask[e.TaskID], e)
	}
	taskAttachmentsByTask := make(map[uint][]models.Attachment)
	commentAttachmentsByComment := make(map[uint][]models.Attachment)
	for _, a := range attachments {
		if a.CommentID == nil {
			taskAttachmentsByTask[a.TaskID] = append(taskAttachmentsByTask[a.TaskID], a)
		} else {
			commentAttachmentsByComment[*a.CommentID] = append(commentAttachmentsByComment[*a.CommentID], a)
		}
	}

	dtos := make([]models.TaskDTO, 0, len(tasks))
	for _, t := range tasks {
		p := placementByTask[t.ID]
		dtos = append(dtos, buildTaskDTO(t, &p, commentsByTask[t.ID], entriesByTask[t.ID], taskAttachmentsByTask[t.ID], commentAttachmentsByComment, users))
	}
	return dtos, nil
}

// loadSingleTaskDTO loads one task with its comments/time entries/users.
// placement is optional (see buildTaskDTO).
func loadSingleTaskDTO(task models.Task, placement *models.TaskPlacement) models.TaskDTO {
	var comments []models.TaskComment
	database.GetDB().Where("task_id = ?", task.ID).Order("created_at ASC").Find(&comments)

	var entries []models.TaskTimeEntry
	database.GetDB().Where("task_id = ?", task.ID).Order("date DESC").Find(&entries)

	var attachments []models.Attachment
	database.GetDB().Where("task_id = ?", task.ID).Order("created_at ASC").Find(&attachments)

	userIDs := []uint{task.CreatedBy, task.UpdatedBy}
	if task.AssigneeID != nil {
		userIDs = append(userIDs, *task.AssigneeID)
	}
	for _, c := range comments {
		userIDs = append(userIDs, c.UserID)
	}
	for _, e := range entries {
		userIDs = append(userIDs, e.UserID)
	}
	for _, a := range attachments {
		userIDs = append(userIDs, a.UploadedBy)
	}
	users := loadUsersByIDs(userIDs)

	taskAttachments := make([]models.Attachment, 0, len(attachments))
	commentAttachments := make(map[uint][]models.Attachment)
	for _, a := range attachments {
		if a.CommentID == nil {
			taskAttachments = append(taskAttachments, a)
		} else {
			commentAttachments[*a.CommentID] = append(commentAttachments[*a.CommentID], a)
		}
	}

	return buildTaskDTO(task, placement, comments, entries, taskAttachments, commentAttachments, users)
}
