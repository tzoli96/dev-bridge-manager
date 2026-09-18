// handlers/kanban_dto_builder.go
package handlers

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"
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
		Attachments: attachmentDTOs,
		CreatedAt:   c.CreatedAt,
		UpdatedAt:   c.UpdatedAt,
	}
}

func buildTimeEntryDTO(e models.TaskTimeEntry, users map[uint]models.User, invoicedPeriods []services.InvoicedPeriod) models.TimeEntryDTO {
	return models.TimeEntryDTO{
		ID:          models.IDToStr(e.ID),
		TaskID:      models.IDToStr(e.TaskID),
		Hours:       e.Hours,
		Description: e.Description,
		Date:        e.Date.Format("2006-01-02"),
		UserID:      models.IDToStr(e.UserID),
		User:        userRefDTO(e.UserID, users),
		Invoiced:    services.IsDateInvoiced(e.Date, invoicedPeriods),
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

// invoicedPeriodsForProject looks up a single project's billed hourly-invoice
// periods (see services.InvoicedPeriodsByProject). Errors are swallowed to
// "no periods" since this only drives a non-critical billed/unbilled badge,
// not billing correctness itself.
func invoicedPeriodsForProject(projectID uint) []services.InvoicedPeriod {
	byProject, err := services.InvoicedPeriodsByProject([]uint{projectID})
	if err != nil {
		return nil
	}
	return byProject[projectID]
}

// taskProjectID looks up a task's project id (needed by call sites — comment
// handlers — that only have a task/comment id in scope, not the full Task row).
func taskProjectID(taskID uint) uint {
	var projectID uint
	database.GetDB().Model(&models.Task{}).Where("id = ?", taskID).Select("project_id").Scan(&projectID)
	return projectID
}

// subtaskProgressByParent batch-computes {total, done} subtask counts for every
// id in parentIDs that has at least one subtask, in a single grouped query.
// "done" counts subtasks whose current board placement sits in an is_done column;
// a subtask with no placement, or placed only in non-done columns, counts toward
// total but not done.
func subtaskProgressByParent(parentIDs []uint) map[uint]models.SubtaskProgressDTO {
	result := make(map[uint]models.SubtaskProgressDTO)
	if len(parentIDs) == 0 {
		return result
	}

	type row struct {
		ParentTaskID uint
		Total        int
		Done         int
	}
	var rows []row
	database.GetDB().Raw(`
		SELECT t.parent_task_id AS parent_task_id,
		       COUNT(DISTINCT t.id) AS total,
		       COUNT(DISTINCT t.id) FILTER (
		           WHERE EXISTS (
		               SELECT 1 FROM task_placements tp
		               JOIN kanban_columns kc ON kc.id = tp.column_id
		               WHERE tp.task_id = t.id AND kc.is_done = true
		           )
		       ) AS done
		FROM tasks t
		WHERE t.parent_task_id IN ? AND t.is_archived = false
		GROUP BY t.parent_task_id
	`, parentIDs).Scan(&rows)

	for _, r := range rows {
		result[r.ParentTaskID] = models.SubtaskProgressDTO{Total: r.Total, Done: r.Done}
	}
	return result
}

// tasksInDoneColumn returns the subset of taskIDs whose current placement (any
// board) sits in a column flagged is_done, in a single query.
func tasksInDoneColumn(taskIDs []uint) map[uint]bool {
	result := make(map[uint]bool)
	if len(taskIDs) == 0 {
		return result
	}
	var ids []uint
	database.GetDB().Raw(`
		SELECT DISTINCT tp.task_id
		FROM task_placements tp
		JOIN kanban_columns kc ON kc.id = tp.column_id
		WHERE tp.task_id IN ? AND kc.is_done = true
	`, taskIDs).Scan(&ids)
	for _, id := range ids {
		result[id] = true
	}
	return result
}

// taskBoardIDs returns each task's current board id (via its placement), for
// tasks that have one. A task with multiple placements resolves to whichever
// one the query returns first.
func taskBoardIDs(taskIDs []uint) map[uint]uint {
	result := make(map[uint]uint)
	if len(taskIDs) == 0 {
		return result
	}
	var rows []struct {
		TaskID  uint
		BoardID uint
	}
	database.GetDB().Raw(`
		SELECT DISTINCT ON (task_id) task_id AS task_id, board_id AS board_id
		FROM task_placements
		WHERE task_id IN ?
		ORDER BY task_id, id
	`, taskIDs).Scan(&rows)
	for _, r := range rows {
		result[r.TaskID] = r.BoardID
	}
	return result
}

// parentTaskRefsByID batch-loads {id, title} for a set of task ids, used to
// resolve the parentTask reference on a subtask's own DTO.
func parentTaskRefsByID(ids []uint) map[uint]models.TaskParentRefDTO {
	result := make(map[uint]models.TaskParentRefDTO)
	if len(ids) == 0 {
		return result
	}
	var rows []models.Task
	database.GetDB().Select("id, title").Where("id IN ?", ids).Find(&rows)
	for _, t := range rows {
		result[t.ID] = models.TaskParentRefDTO{ID: models.IDToStr(t.ID), Title: t.Title}
	}
	return result
}

// buildTaskDTO assembles a full TaskDTO from a task row plus its already-loaded
// comments/time entries (both filtered to this task) and a shared user lookup map.
// placement is optional: when nil (project-scoped, placement-independent contexts)
// ColumnID/Position come back as the zero value ("" / 0); otherwise they're sourced
// from the given board placement.
func buildTaskDTO(t models.Task, placement *models.TaskPlacement, comments []models.TaskComment, entries []models.TaskTimeEntry, taskAttachments []models.Attachment, commentAttachments map[uint][]models.Attachment, users map[uint]models.User, subtaskProgress *models.SubtaskProgressDTO, parentTask *models.TaskParentRefDTO, invoicedPeriods []services.InvoicedPeriod) models.TaskDTO {
	commentDTOs := make([]models.TaskCommentDTO, 0, len(comments))
	for _, c := range comments {
		commentDTOs = append(commentDTOs, buildCommentDTO(c, users, t.ProjectID, commentAttachments[c.ID]))
	}

	entryDTOs := make([]models.TimeEntryDTO, 0, len(entries))
	var loggedHours float64
	var hasUninvoicedHours bool
	for _, e := range entries {
		entryDTO := buildTimeEntryDTO(e, users, invoicedPeriods)
		entryDTOs = append(entryDTOs, entryDTO)
		loggedHours += e.Hours
		if e.Hours > 0 && !entryDTO.Invoiced {
			hasUninvoicedHours = true
		}
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
		Assignee:           assigneeDTO(t.AssigneeID, users),
		EstimatedHours:     t.EstimatedHours,
		LoggedHours:        loggedHours,
		Tags:               models.TagsFromJSON(t.Tags),
		TimeEntries:        entryDTOs,
		Comments:           commentDTOs,
		Attachments:        attachmentDTOs,
		Position:           position,
		DueDate:            models.FormatDate(t.DueDate),
		SubtaskProgress:    subtaskProgress,
		ParentTask:         parentTask,
		HasUninvoicedHours: hasUninvoicedHours,
		CreatedAt:          t.CreatedAt,
		UpdatedAt:          t.UpdatedAt,
		CreatedBy:          models.IDToStr(t.CreatedBy),
		UpdatedBy:          models.IDToStr(t.UpdatedBy),
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

	progressByParent := subtaskProgressByParent(taskIDs)

	parentIDs := make([]uint, 0)
	for _, t := range tasks {
		if t.ParentTaskID != nil {
			parentIDs = append(parentIDs, *t.ParentTaskID)
		}
	}
	parentRefs := parentTaskRefsByID(parentIDs)

	invoicedPeriods := invoicedPeriodsForProject(projectID)
	boardIDs := taskBoardIDs(taskIDs)

	dtos := make([]models.TaskDTO, 0, len(tasks))
	for _, t := range tasks {
		var progress *models.SubtaskProgressDTO
		if p, ok := progressByParent[t.ID]; ok {
			progress = &p
		}
		var parentRef *models.TaskParentRefDTO
		if t.ParentTaskID != nil {
			if ref, ok := parentRefs[*t.ParentTaskID]; ok {
				parentRef = &ref
			}
		}
		dto := buildTaskDTO(t, nil, commentsByTask[t.ID], entriesByTask[t.ID], taskAttachmentsByTask[t.ID], commentAttachmentsByComment, users, progress, parentRef, invoicedPeriods)
		if boardID, ok := boardIDs[t.ID]; ok {
			dto.BoardID = models.IDToStr(boardID)
		}
		dtos = append(dtos, dto)
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

	progressByParent := subtaskProgressByParent(rowIDs)

	parentIDs := make([]uint, 0)
	for _, t := range tasks {
		if t.ParentTaskID != nil {
			parentIDs = append(parentIDs, *t.ParentTaskID)
		}
	}
	parentRefs := parentTaskRefsByID(parentIDs)

	projectIDSet := make(map[uint]struct{})
	for _, t := range tasks {
		projectIDSet[t.ProjectID] = struct{}{}
	}
	projectIDs := make([]uint, 0, len(projectIDSet))
	for id := range projectIDSet {
		projectIDs = append(projectIDs, id)
	}
	invoicedPeriodsByProject, _ := services.InvoicedPeriodsByProject(projectIDs)

	dtos := make([]models.TaskDTO, 0, len(tasks))
	for _, t := range tasks {
		p := placementByTask[t.ID]
		var progress *models.SubtaskProgressDTO
		if pr, ok := progressByParent[t.ID]; ok {
			progress = &pr
		}
		var parentRef *models.TaskParentRefDTO
		if t.ParentTaskID != nil {
			if ref, ok := parentRefs[*t.ParentTaskID]; ok {
				parentRef = &ref
			}
		}
		dtos = append(dtos, buildTaskDTO(t, &p, commentsByTask[t.ID], entriesByTask[t.ID], taskAttachmentsByTask[t.ID], commentAttachmentsByComment, users, progress, parentRef, invoicedPeriodsByProject[t.ProjectID]))
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

	progressByParent := subtaskProgressByParent([]uint{task.ID})
	var progress *models.SubtaskProgressDTO
	if p, ok := progressByParent[task.ID]; ok {
		progress = &p
	}

	var parentRef *models.TaskParentRefDTO
	if task.ParentTaskID != nil {
		if ref, ok := parentTaskRefsByID([]uint{*task.ParentTaskID})[*task.ParentTaskID]; ok {
			parentRef = &ref
		}
	}

	return buildTaskDTO(task, placement, comments, entries, taskAttachments, commentAttachments, users, progress, parentRef, invoicedPeriodsForProject(task.ProjectID))
}

// loadSubtaskDTOs loads all (non-archived) subtasks of a given parent task, each
// resolved with its earliest known board placement (a subtask is usually placed on
// exactly one board — if placed on more than one, this only affects the informational
// columnId shown on the subtask row, not the parent's own subtaskProgress count computed
// by subtaskProgressByParent above). Subtasks are one level deep only, so every returned
// DTO's own subtaskProgress is nil.
func loadSubtaskDTOs(parentTaskID uint) ([]models.TaskDTO, error) {
	var tasks []models.Task
	if err := database.GetDB().Where("parent_task_id = ? AND is_archived = false", parentTaskID).
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

	var placements []models.TaskPlacement
	database.GetDB().Where("task_id IN ?", taskIDs).Order("id ASC").Find(&placements)
	placementByTask := make(map[uint]models.TaskPlacement, len(placements))
	for _, p := range placements {
		if _, exists := placementByTask[p.TaskID]; !exists {
			placementByTask[p.TaskID] = p
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

	parentRefs := parentTaskRefsByID([]uint{parentTaskID})
	parentRef, hasParentRef := parentRefs[parentTaskID]

	doneSet := tasksInDoneColumn(taskIDs)

	projectIDSet := make(map[uint]struct{})
	for _, t := range tasks {
		projectIDSet[t.ProjectID] = struct{}{}
	}
	projectIDs := make([]uint, 0, len(projectIDSet))
	for id := range projectIDSet {
		projectIDs = append(projectIDs, id)
	}
	invoicedPeriodsByProject, _ := services.InvoicedPeriodsByProject(projectIDs)

	dtos := make([]models.TaskDTO, 0, len(tasks))
	for _, t := range tasks {
		var pt *models.TaskParentRefDTO
		if hasParentRef {
			ref := parentRef
			pt = &ref
		}
		var placement *models.TaskPlacement
		if p, ok := placementByTask[t.ID]; ok {
			placement = &p
		}
		dto := buildTaskDTO(t, placement, commentsByTask[t.ID], entriesByTask[t.ID], taskAttachmentsByTask[t.ID], commentAttachmentsByComment, users, nil, pt, invoicedPeriodsByProject[t.ProjectID])
		dto.IsDoneColumn = doneSet[t.ID]
		dtos = append(dtos, dto)
	}
	return dtos, nil
}
