package handlers

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"
	"dev-bridge-manager/internal/types"
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
	Password string `json:"password,omitempty" validate:"omitempty,min=6"` // ÚJ: opcionális jelszó
}

type UpdateProfileRequest struct {
	Name     string `json:"name,omitempty" validate:"min=2"`
	Email    string `json:"email,omitempty" validate:"email"`
	Position string `json:"position,omitempty"`
}

type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" validate:"required"`
	NewPassword     string `json:"new_password" validate:"required,min=6"`
	ConfirmPassword string `json:"confirm_password" validate:"required"`
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
		return c.Status(500).JSON(types.UserListResponse{
			Success: false,
			Message: "Failed to fetch users",
		})
	}

	var userResponses []types.UserResponse
	for _, user := range users {
		userResponses = append(userResponses, types.UserResponse{
			ID:       user.ID,
			Name:     user.Name,
			Email:    user.Email,
			Position: user.Position,
			Role: types.RoleInfo{
				ID:          user.Role.ID,
				Name:        user.Role.Name,
				DisplayName: user.Role.DisplayName,
			},
		})
	}

	return c.JSON(types.UserListResponse{
		Success: true,
		Message: "Users retrieved successfully",
		Users:   userResponses,
	})
}

// GetUser returns a specific user
func (h *UserHandler) GetUser(c *fiber.Ctx) error {
	userID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return c.Status(400).JSON(types.UserListResponse{
			Success: false,
			Message: "Invalid user ID",
		})
	}

	// Check if user is accessing their own profile or has users.read permission
	currentUserID := c.Locals("userID").(uint)
	if currentUserID != uint(userID) {
		hasPermission, err := h.permissionService.CheckUserPermission(currentUserID, "users.read")
		if err != nil || !hasPermission {
			return c.Status(403).JSON(types.UserListResponse{
				Success: false,
				Message: "Insufficient permissions",
			})
		}
	}

	user, err := h.permissionService.GetUserWithPermissions(uint(userID))
	if err != nil {
		return c.Status(404).JSON(types.UserListResponse{
			Success: false,
			Message: "User not found",
		})
	}

	return c.JSON(types.UserListResponse{
		Success: true,
		Message: "User retrieved successfully",
		User: &types.UserResponse{
			ID:       user.ID,
			Name:     user.Name,
			Email:    user.Email,
			Position: user.Position,
			Role: types.RoleInfo{
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
		return c.Status(400).JSON(types.UserListResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	// Input validáció
	if req.Name == "" || req.Email == "" || req.Password == "" || req.RoleName == "" {
		return c.Status(400).JSON(types.UserListResponse{
			Success: false,
			Message: "Name, email, password and role are required",
		})
	}

	// Email egyediség ellenőrzése
	var existingUser models.User
	if err := database.GetDB().Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		return c.Status(409).JSON(types.UserListResponse{
			Success: false,
			Message: "User with this email already exists",
		})
	}

	// Role létezésének ellenőrzése
	var role models.Role
	if err := database.GetDB().Where("name = ? AND is_active = ?", req.RoleName, true).First(&role).Error; err != nil {
		return c.Status(400).JSON(types.UserListResponse{
			Success: false,
			Message: "Invalid role specified",
		})
	}

	// User létrehozása
	user := models.User{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
		Position: req.Position,
		RoleID:   role.ID,
	}

	// Jelszó hash-elése
	if err := user.HashPassword(); err != nil {
		return c.Status(500).JSON(types.UserListResponse{
			Success: false,
			Message: "Failed to process password",
		})
	}

	// User mentése adatbázisba
	if err := database.GetDB().Create(&user).Error; err != nil {
		return c.Status(500).JSON(types.UserListResponse{
			Success: false,
			Message: "Failed to create user",
		})
	}

	// User újratöltése role-lal együtt
	if err := database.GetDB().Preload("Role").First(&user, user.ID).Error; err != nil {
		return c.Status(500).JSON(types.UserListResponse{
			Success: false,
			Message: "User created but failed to load details",
		})
	}

	return c.Status(201).JSON(types.UserListResponse{
		Success: true,
		Message: "User created successfully",
		User: &types.UserResponse{
			ID:       user.ID,
			Name:     user.Name,
			Email:    user.Email,
			Position: user.Position,
			Role: types.RoleInfo{
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
		return c.Status(400).JSON(types.UserListResponse{
			Success: false,
			Message: "Invalid user ID",
		})
	}

	var req UpdateUserRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(types.UserListResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	// Legalább egy mező legyen megadva
	if req.Name == "" && req.Email == "" && req.Position == "" && req.RoleName == "" && req.Password == "" {
		return c.Status(400).JSON(types.UserListResponse{
			Success: false,
			Message: "At least one field must be provided",
		})
	}

	// Permission ellenőrzés - csak admin frissíthet másokat
	currentUserID := c.Locals("userID").(uint)
	if currentUserID != uint(userID) {
		hasPermission, err := h.permissionService.CheckUserPermission(currentUserID, "users.update")
		if err != nil || !hasPermission {
			return c.Status(403).JSON(types.UserListResponse{
				Success: false,
				Message: "Insufficient permissions",
			})
		}
	}

	// Email egyediség ellenőrzése (ha új email van megadva)
	if req.Email != "" {
		var existingUser models.User
		if err := database.GetDB().Where("email = ? AND id != ?", req.Email, userID).First(&existingUser).Error; err == nil {
			return c.Status(409).JSON(types.UserListResponse{
				Success: false,
				Message: "Email already in use by another user",
			})
		}
	}

	// Frissítendő mezők összeállítása
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

	// Role frissítés (admin jogosultságot igényel)
	if req.RoleName != "" {
		hasRolePermission, err := h.permissionService.CheckUserPermission(currentUserID, "users.update")
		if err != nil || !hasRolePermission {
			return c.Status(403).JSON(types.UserListResponse{
				Success: false,
				Message: "Insufficient permissions to update role",
			})
		}

		var role models.Role
		if err := database.GetDB().Where("name = ? AND is_active = ?", req.RoleName, true).First(&role).Error; err != nil {
			return c.Status(400).JSON(types.UserListResponse{
				Success: false,
				Message: "Invalid role specified",
			})
		}
		updates["role_id"] = role.ID
	}

	// Jelszó frissítés (ha meg van adva)
	if req.Password != "" {
		tempUser := models.User{Password: req.Password}
		if err := tempUser.HashPassword(); err != nil {
			return c.Status(500).JSON(types.UserListResponse{
				Success: false,
				Message: "Failed to process new password",
			})
		}
		updates["password"] = tempUser.Password
	}

	// User frissítése
	if err := database.GetDB().Model(&models.User{}).Where("id = ?", userID).Updates(updates).Error; err != nil {
		return c.Status(500).JSON(types.UserListResponse{
			Success: false,
			Message: "Failed to update user",
		})
	}

	// Frissített user lekérése role-lal együtt
	var updatedUser models.User
	if err := database.GetDB().Preload("Role").First(&updatedUser, userID).Error; err != nil {
		return c.Status(500).JSON(types.UserListResponse{
			Success: false,
			Message: "User updated but failed to load details",
		})
	}

	return c.JSON(types.UserListResponse{
		Success: true,
		Message: "User updated successfully",
		User: &types.UserResponse{
			ID:       updatedUser.ID,
			Name:     updatedUser.Name,
			Email:    updatedUser.Email,
			Position: updatedUser.Position,
			Role: types.RoleInfo{
				ID:          updatedUser.Role.ID,
				Name:        updatedUser.Role.Name,
				DisplayName: updatedUser.Role.DisplayName,
			},
		},
	})
}

// DeleteUser deletes a user
func (h *UserHandler) DeleteUser(c *fiber.Ctx) error {
	userID, err := strconv.ParseUint(c.Params("id"), 10, 32)
	if err != nil {
		return c.Status(400).JSON(types.UserListResponse{
			Success: false,
			Message: "Invalid user ID",
		})
	}

	// Self-deletion prevention
	currentUserID := c.Locals("userID").(uint)
	if currentUserID == uint(userID) {
		return c.Status(400).JSON(types.UserListResponse{
			Success: false,
			Message: "Cannot delete your own account",
		})
	}

	// Ellenőrizzük, hogy a user létezik-e
	var user models.User
	if err := database.GetDB().First(&user, userID).Error; err != nil {
		return c.Status(404).JSON(types.UserListResponse{
			Success: false,
			Message: "User not found",
		})
	}

	// User törlése
	if err := database.GetDB().Delete(&models.User{}, userID).Error; err != nil {
		return c.Status(500).JSON(types.UserListResponse{
			Success: false,
			Message: "Failed to delete user",
		})
	}

	return c.JSON(types.UserListResponse{
		Success: true,
		Message: "User deleted successfully",
	})
}

// GetProfile returns current user's profile
func (h *UserHandler) GetProfile(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)

	user, err := h.permissionService.GetUserWithPermissions(userID)
	if err != nil {
		return c.Status(404).JSON(types.UserListResponse{
			Success: false,
			Message: "User not found",
		})
	}

	return c.JSON(types.UserListResponse{
		Success: true,
		Message: "Profile retrieved successfully",
		User: &types.UserResponse{
			ID:       user.ID,
			Name:     user.Name,
			Email:    user.Email,
			Position: user.Position,
			Role: types.RoleInfo{
				ID:          user.Role.ID,
				Name:        user.Role.Name,
				DisplayName: user.Role.DisplayName,
			},
			Permissions: user.GetPermissions(),
		},
	})
}

// UpdateProfile updates the current user's profile
func (h *UserHandler) UpdateProfile(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)

	var req UpdateProfileRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(types.UserListResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	if req.Name == "" && req.Email == "" && req.Position == "" {
		return c.Status(400).JSON(types.UserListResponse{
			Success: false,
			Message: "At least one field must be provided",
		})
	}

	if req.Email != "" {
		var existingUser models.User
		if err := database.GetDB().Where("email = ? AND id != ?", req.Email, currentUserID).First(&existingUser).Error; err == nil {
			return c.Status(409).JSON(types.UserListResponse{
				Success: false,
				Message: "Email already in use by another user",
			})
		}
	}

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

	if err := database.GetDB().Model(&models.User{}).Where("id = ?", currentUserID).Updates(updates).Error; err != nil {
		return c.Status(500).JSON(types.UserListResponse{
			Success: false,
			Message: "Failed to update profile",
		})
	}

	user, err := h.permissionService.GetUserWithPermissions(currentUserID)
	if err != nil {
		return c.Status(500).JSON(types.UserListResponse{
			Success: false,
			Message: "Profile updated but failed to load updated data",
		})
	}

	return c.JSON(types.UserListResponse{
		Success: true,
		Message: "Profile updated successfully",
		User: &types.UserResponse{
			ID:       user.ID,
			Name:     user.Name,
			Email:    user.Email,
			Position: user.Position,
			Role: types.RoleInfo{
				ID:          user.Role.ID,
				Name:        user.Role.Name,
				DisplayName: user.Role.DisplayName,
			},
			Permissions: user.GetPermissions(),
		},
	})
}

func (h *UserHandler) ChangePassword(c *fiber.Ctx) error {
	currentUserID := c.Locals("userID").(uint)

	var req ChangePasswordRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(types.UserListResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	if req.NewPassword != req.ConfirmPassword {
		return c.Status(400).JSON(types.UserListResponse{
			Success: false,
			Message: "Password confirmation does not match",
		})
	}

	user, err := h.permissionService.GetUserWithPermissions(currentUserID)
	if err != nil {
		return c.Status(404).JSON(types.UserListResponse{
			Success: false,
			Message: "User not found",
		})
	}

	if !user.CheckPassword(req.CurrentPassword) {
		return c.Status(400).JSON(types.UserListResponse{
			Success: false,
			Message: "Current password is incorrect",
		})
	}

	if user.CheckPassword(req.NewPassword) {
		return c.Status(400).JSON(types.UserListResponse{
			Success: false,
			Message: "New password must be different from current password",
		})
	}

	tempUser := models.User{Password: req.NewPassword}
	if err := tempUser.HashPassword(); err != nil {
		return c.Status(500).JSON(types.UserListResponse{
			Success: false,
			Message: "Failed to process new password",
		})
	}

	if err := database.GetDB().Model(&models.User{}).Where("id = ?", currentUserID).Update("password", tempUser.Password).Error; err != nil {
		return c.Status(500).JSON(types.UserListResponse{
			Success: false,
			Message: "Failed to update password",
		})
	}

	return c.JSON(types.UserListResponse{
		Success: true,
		Message: "Password changed successfully. Please log in again.",
	})
}
