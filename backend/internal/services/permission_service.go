package services

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"gorm.io/gorm"
)

type PermissionService struct {
	db *gorm.DB
}

func NewPermissionService() *PermissionService {
	return &PermissionService{
		db: database.GetDB(),
	}
}

// GetUserWithPermissions gets user with role and permissions loaded
func (s *PermissionService) GetUserWithPermissions(userID uint) (*models.User, error) {
	var user models.User
	err := s.db.Preload("Role.Permissions").First(&user, userID).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// CheckUserPermission checks if user has specific permission
func (s *PermissionService) CheckUserPermission(userID uint, permissionName string) (bool, error) {
	user, err := s.GetUserWithPermissions(userID)
	if err != nil {
		return false, err
	}
	return user.HasPermission(permissionName), nil
}

// GetAllRoles returns all active roles with permissions
func (s *PermissionService) GetAllRoles() ([]models.Role, error) {
	var roles []models.Role
	err := s.db.Preload("Permissions").Where("is_active = ?", true).Find(&roles).Error
	return roles, err
}

// GetAllPermissions returns all active permissions
func (s *PermissionService) GetAllPermissions() ([]models.Permission, error) {
	var permissions []models.Permission
	err := s.db.Where("is_active = ?", true).Find(&permissions).Error
	return permissions, err
}

// CreateRole creates a new role
func (s *PermissionService) CreateRole(role *models.Role) error {
	return s.db.Create(role).Error
}

// UpdateRole updates a role
func (s *PermissionService) UpdateRole(roleID uint, updates map[string]interface{}) error {
	return s.db.Model(&models.Role{}).Where("id = ?", roleID).Updates(updates).Error
}

// AssignPermissionsToRole assigns permissions to a role
func (s *PermissionService) AssignPermissionsToRole(roleID uint, permissionIDs []uint) error {
	role := &models.Role{ID: roleID}

	// Clear existing permissions
	if err := s.db.Model(role).Association("Permissions").Clear(); err != nil {
		return err
	}

	// Get permissions
	var permissions []models.Permission
	if err := s.db.Find(&permissions, permissionIDs).Error; err != nil {
		return err
	}

	// Assign new permissions
	return s.db.Model(role).Association("Permissions").Append(permissions)
}

// AssignRoleToUser assigns a role to user
func (s *PermissionService) AssignRoleToUser(userID, roleID uint) error {
	return s.db.Model(&models.User{}).Where("id = ?", userID).Update("role_id", roleID).Error
}
