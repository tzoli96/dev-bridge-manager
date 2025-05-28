package handlers

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

type UsersHandler struct{}

type UserHandler struct {
	permissionService *services.PermissionService
}

type CreateUserRequest struct {
	Name     string `json:"name" validate:"required,min=2"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
	Position string `json:"position"`
	RoleName string `json:"role_name" validate:"required"`
}

type UpdateUserRequest struct {
	Name     string `json:"name,omitempty"`
	Email    string `json:"email,omitempty"`
	Position string `json:"position,omitempty"`
	RoleName string `json:"role_name,omitempty"`
}

type UserListResponse struct {
	Success bool           `json:"success"`
	Message string         `json:"message"`
	Users   []UserResponse `json:"users,omitempty"`
	User    *UserResponse  `json:"user,omitempty"`
}

func NewUsersHandler() *UsersHandler {
	return &UsersHandler{}
}

func (h *UsersHandler) GetUsers(c *fiber.Ctx) error {
	var users []models.User

	result := database.GetDB().Find(&users)
	if result.Error != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": result.Error.Error(),
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    users,
		"count":   len(users),
	})
}

func (h *UsersHandler) GetUser(c *fiber.Ctx) error {
	id := c.Params("id")
	var user models.User

	result := database.GetDB().First(&user, id)
	if result.Error != nil {
		return c.Status(404).JSON(fiber.Map{
			"error": "User not found",
		})
	}

	return c.JSON(fiber.Map{
		"success": true,
		"data":    user,
	})
}

func NewUserHandler() *UserHandler {
	return &UserHandler{
		permissionService: services.NewPermissionService(),
	}
}

// GetAllUsers returns all users with their roles
func (h *UserHandler) GetAllUsers(c *fiber.Ctx) error {
	var users []models.User
	if err := database.GetDB().Preload("Role").Find(&users).Error; err != nil {
		return c.Status(500).JSON(UserListResponse{
			Success: false,
			Message: "Failed to fetch users",
		})
	}

	var userResponses []UserResponse
	for _, user := range users {
		userResponses = append(userResponses, UserResponse{
			ID:       user.ID,
			Name:     user.Name,
			Email:    user.Email,
			Position: user.Position,
			Role: RoleInfo{
				ID:          user.Role.ID,
				Name:        user.Role.Name,
				DisplayName: user.Role.DisplayName,
			},
		})
	}

	return c.JSON(UserListResponse{
		Success: true,
		Message: "Users retrieved successfully",
		Users:   userResponses,
	})
}

// GetUser returns a specific user
func (h *UserHandler) GetUser(c *fiber.Ctx) error {
	userID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return c.Status(400).JSON(UserListResponse{
			Success: false,
			Message: "Invalid user ID",
		})
	}

	// Check if user is accessing their own profile or has users.read permission
	currentUserID := c.Locals("userID").(uint)
	if currentUserID != uint(userID) {
		hasPermission, err := h.permissionService.CheckUserPermission(currentUserID, "users.read")
		if err != nil || !hasPermission {
			return c.Status(403).JSON(UserListResponse{
				Success: false,
				Message: "Insufficient permissions",
			})
		}
	}

	user, err := h.permissionService.GetUserWithPermissions(uint(userID))
	if err != nil {
		return c.Status(404).JSON(UserListResponse{
			Success: false,
			Message: "User not found",
		})
	}

	return c.JSON(UserListResponse{
		Success: true,
		Message: "User retrieved successfully",
		User: &UserResponse{
			ID:       user.ID,
			Name:     user.Name,
			Email:    user.Email,
			Position: user.Position,
			Role: RoleInfo{
				ID:          user.Role.ID,
				Name:        user.Role.Name,
				DisplayName: user.Role.DisplayName,
			},
			Permissions: user.GetPermissions(),
		},
	})
}

// CreateUser creates a new user (admin only)
func (h *UserHandler) CreateUser(c *fiber.Ctx) error {
	var req CreateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(UserListResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	// Check if user already exists
	var existingUser models.User
	if err := database.GetDB().Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		return c.Status(409).JSON(UserListResponse{
			Success: false,
			Message: "User with this email already exists",
		})
	}

	// Get role
	var role models.Role
	if err := database.GetDB().Where("name = ? AND is_active = ?", req.RoleName, true).First(&role).Error; err != nil {
		return c.Status(400).JSON(UserListResponse{
			Success: false,
			Message: "Invalid role specified",
		})
	}

	// Create user
	user := models.User{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
		Position: req.Position,
		RoleID:   role.ID,
	}

	if err := user.HashPassword(); err != nil {
		return c.Status(500).JSON(UserListResponse{
			Success: false,
			Message: "Failed to process password",
		})
	}

	if err := database.GetDB().Create(&user).Error; err != nil {
		return c.Status(500).JSON(UserListResponse{
			Success: false,
			Message: "Failed to create user",
		})
	}

	// Load user with role for response
	if err := database.GetDB().Preload("Role").First(&user, user.ID).Error; err != nil {
		return c.Status(500).JSON(UserListResponse{
			Success: false,
			Message: "User created but failed to load details",
		})
	}

	return c.Status(201).JSON(UserListResponse{
		Success: true,
		Message: "User created successfully",
		User: &UserResponse{
			ID:       user.ID,
			Name:     user.Name,
			Email:    user.Email,
			Position: user.Position,
			Role: RoleInfo{
				ID:          user.Role.ID,
				Name:        user.Role.Name,
				DisplayName: user.Role.DisplayName,
			},
		},
	})
}

// UpdateUser updates a user
func (h *UserHandler) UpdateUser(c *fiber.Ctx) error {
	userID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return c.Status(400).JSON(UserListResponse{
			Success: false,
			Message: "Invalid user ID",
		})
	}

	var req UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(UserListResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	// Check permissions
	currentUserID := c.Locals("userID").(uint)
	if currentUserID != uint(userID) {
		hasPermission, err := h.permissionService.CheckUserPermission(currentUserID, "users.update")
		if err != nil || !hasPermission {
			return c.Status(403).JSON(UserListResponse{
				Success: false,
				Message: "Insufficient permissions",
			})
		}
	}

	// Build updates map
	updates := make(map[string]interface{})
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Email != "" {
		updates["email"] = req.Email
	}
	if req.Position != "" {
		updates["position"] = req.Position
	}

	// Handle role update (only if user has permission)
	if req.RoleName != "" {
		hasRolePermission, err := h.permissionService.CheckUserPermission(currentUserID, "users.update")
		if err != nil || !hasRolePermission {
			return c.Status(403).JSON(UserListResponse{
				Success: false,
				Message: "Insufficient permissions to update role",
			})
		}

		var role models.Role
		if err := database.GetDB().Where("name = ? AND is_active = ?", req.RoleName, true).First(&role).Error; err != nil {
			return c.Status(400).JSON(UserListResponse{
				Success: false,
				Message: "Invalid role specified",
			})
		}
		updates["role_id"] = role.ID
	}

	// Update user
	if err := database.GetDB().Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		return c.Status(500).JSON(UserListResponse{
			Success: false,
			Message: "Failed to update user",
		})
	}

	return c.JSON(UserListResponse{
		Success: true,
		Message: "User updated successfully",
	})
}

// DeleteUser deletes a user
func (h *UserHandler) DeleteUser(c *fiber.Ctx) error {
	userID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return c.Status(400).JSON(UserListResponse{
			Success: false,
			Message: "Invalid user ID",
		})
	}

	// Prevent self-deletion
	currentUserID := c.Locals("userID").(uint)
	if currentUserID == uint(userID) {
		return c.Status(400).JSON(UserListResponse{
			Success: false,
			Message: "Cannot delete your own account",
		})
	}

	if err := database.GetDB().Delete(&models.User{}, userID).Error; err != nil {
		return c.Status(500).JSON(UserListResponse{
			Success: false,
			Message: "Failed to delete user",
		})
	}

	return c.JSON(UserListResponse{
		Success: true,
		Message: "User deleted successfully",
	})
}

// GetProfile returns current user's profile
func (h *UserHandler) GetProfile(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)

	user, err := h.permissionService.GetUserWithPermissions(userID)
	if err != nil {
		return c.Status(404).JSON(UserListResponse{
			Success: false,
			Message: "User not found",
		})
	}

	return c.JSON(UserListResponse{
		Success: true,
		Message: "Profile retrieved successfully",
		User: &UserResponse{
			ID:       user.ID,
			Name:     user.Name,
			Email:    user.Email,
			Position: user.Position,
			Role: RoleInfo{
				ID:          user.Role.ID,
				Name:        user.Role.Name,
				DisplayName: user.Role.DisplayName,
			},
			Permissions: user.GetPermissions(),
		},
	})
}
