// backend/internal/services/gmail_sync_client_match.go
package services

import (
	"context"
	"log"

	"dev-bridge-manager/internal/models"

	"gorm.io/gorm"
)

// matchClientAndProject is the client/project half of gmail_sync.go's
// ingestion pipeline: only called for a message categorizeIfInbox already
// tagged "ugyfel". It asks the AI matcher which active client the sender
// most likely is, then deterministically assigns a project only when that
// client has exactly one - with more than one, which project applies is
// genuinely ambiguous and is left for the user to pick when they create a
// task from the email (see CreateTaskFromEmailModal).
func matchClientAndProject(ctx context.Context, db *gorm.DB, matcher EmailClientMatcher, meta *GmailMessageMeta) (clientID *uint, projectID *uint) {
	if matcher == nil {
		return nil, nil
	}

	var clients []models.Client
	if err := db.Where("is_active = ?", true).Select("id, name, email").Find(&clients).Error; err != nil {
		log.Printf("gmail sync: failed to load clients for matching: %v", err)
		return nil, nil
	}
	if len(clients) == 0 {
		return nil, nil
	}

	candidates := make([]ClientCandidate, len(clients))
	for i, c := range clients {
		candidates[i] = ClientCandidate{ID: c.ID, Name: c.Name, Email: c.Email}
	}

	matchedID, err := matcher.MatchClient(ctx, meta.Subject, meta.Snippet, meta.FromAddress, meta.FromName, candidates)
	if err != nil {
		log.Printf("gmail sync: failed to match client for message %s: %v", meta.GmailMessageID, err)
		return nil, nil
	}
	if matchedID == nil {
		return nil, nil
	}

	var projectLinks []models.ProjectClient
	if err := db.Where("client_id = ?", *matchedID).Find(&projectLinks).Error; err != nil {
		log.Printf("gmail sync: failed to load projects for client %d: %v", *matchedID, err)
		return matchedID, nil
	}
	if len(projectLinks) == 1 {
		return matchedID, &projectLinks[0].ProjectID
	}
	return matchedID, nil
}
