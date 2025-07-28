package handlers

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"dev-bridge-manager/internal/services"
	"dev-bridge-manager/internal/types"
	"github.com/gofiber/fiber/v2"
)

type AuthHandler struct {
	authService       *services.AuthService
	permissionService *services.PermissionService
}

type RegisterRequest struct {
	Name     string `json:"name" validate:"required,min=2"`
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required,min=6"`
	Position string `json:"position"`
	RoleName string `json:"role_name,omitempty"` // Optional, only admins can set
}

type LoginRequest struct {
	Email    string `json:"email" validate:"required,email"`
	Password string `json:"password" validate:"required"`
}

func NewAuthHandler() *AuthHandler {
	return &AuthHandler{
		authService:       services.NewAuthService(),
		permissionService: services.NewPermissionService(),
	}
}

// Register creates a new user account
func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req RegisterRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(types.AuthResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	// Check if user already exists
	var existingUser models.User
	if err := database.GetDB().Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		return c.Status(409).JSON(types.AuthResponse{
			Success: false,
			Message: "User with this email already exists",
		})
	}

	// Get default role (user)
	var defaultRole models.Role
	if err := database.GetDB().Where("name = ?", "user").First(&defaultRole).Error; err != nil {
		return c.Status(500).JSON(types.AuthResponse{
			Success: false,
			Message: "Default role not found",
		})
	}

	roleID := defaultRole.ID

	// Check if role is being set and if user has permission
	if req.RoleName != "" {
		// Check if current user has permission to assign roles
		currentUserID, ok := c.Locals("userID").(uint)
		if !ok {
			return c.Status(403).JSON(types.AuthResponse{
				Success: false,
				Message: "Only authenticated users with proper permissions can assign roles",
			})
		}

		hasPermission, err := h.permissionService.CheckUserPermission(currentUserID, "users.create")
		if err != nil || !hasPermission {
			return c.Status(403).JSON(types.AuthResponse{
				Success: false,
				Message: "Insufficient permissions to assign roles",
			})
		}

		// Get the requested role
		var requestedRole models.Role
		if err := database.GetDB().Where("name = ? AND is_active = ?", req.RoleName, true).First(&requestedRole).Error; err != nil {
			return c.Status(400).JSON(types.AuthResponse{
				Success: false,
				Message: "Invalid role specified",
			})
		}
		roleID = requestedRole.ID
	}

	// Create new user
	user := models.User{
		Name:     req.Name,
		Email:    req.Email,
		Password: req.Password,
		Position: req.Position,
		RoleID:   roleID,
	}

	// Hash password
	if err := user.HashPassword(); err != nil {
		return c.Status(500).JSON(types.AuthResponse{
			Success: false,
			Message: "Failed to process password",
		})
	}

	// Save to database
	if err := database.GetDB().Create(&user).Error; err != nil {
		return c.Status(500).JSON(types.AuthResponse{
			Success: false,
			Message: "Failed to create user",
		})
	}

	// Load user with role and permissions for response
	userWithRole, err := h.permissionService.GetUserWithPermissions(user.ID)
	if err != nil {
		return c.Status(500).JSON(types.AuthResponse{
			Success: false,
			Message: "User created but failed to load details",
		})
	}

	// Generate token
	token, err := h.authService.GenerateToken(user.ID, user.Email, user.Name)
	if err != nil {
		return c.Status(500).JSON(types.AuthResponse{
			Success: false,
			Message: "Failed to generate token",
		})
	}

	return c.Status(201).JSON(types.AuthResponse{
		Success: true,
		Message: "User registered successfully",
		Token:   token,
		User: &types.UserResponse{
			ID:       userWithRole.ID,
			Name:     userWithRole.Name,
			Email:    userWithRole.Email,
			Position: userWithRole.Position,
			Role: types.RoleInfo{
				ID:          userWithRole.Role.ID,
				Name:        userWithRole.Role.Name,
				DisplayName: userWithRole.Role.DisplayName,
			},
			Permissions: userWithRole.GetPermissions(),
		},
	})
}

// Login authenticates user and returns JWT token
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req LoginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(types.AuthResponse{
			Success: false,
			Message: "Invalid request body",
		})
	}

	// Find user by email with role
	var user models.User
	if err := database.GetDB().Preload("Role.Permissions").Where("email = ?", req.Email).First(&user).Error; err != nil {
		return c.Status(401).JSON(types.AuthResponse{
			Success: false,
			Message: "Invalid credentials",
		})
	}

	// Check password
	if !user.CheckPassword(req.Password) {
		return c.Status(401).JSON(types.AuthResponse{
			Success: false,
			Message: "Invalid credentials",
		})
	}

	// Check if role is active
	if !user.Role.IsActive {
		return c.Status(403).JSON(types.AuthResponse{
			Success: false,
			Message: "Account is deactivated",
		})
	}

	// Generate token
	token, err := h.authService.GenerateToken(user.ID, user.Email, user.Name)
	if err != nil {
		return c.Status(500).JSON(types.AuthResponse{
			Success: false,
			Message: "Failed to generate token",
		})
	}

	return c.JSON(types.AuthResponse{
		Success: true,
		Message: "Login successful",
		Token:   token,
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

// Me returns current user info
func (h *AuthHandler) Me(c *fiber.Ctx) error {
	userID := c.Locals("userID").(uint)

	userWithRole, err := h.permissionService.GetUserWithPermissions(userID)
	if err != nil {
		return c.Status(404).JSON(types.AuthResponse{
			Success: false,
			Message: "User not found",
		})
	}

	return c.JSON(types.AuthResponse{
		Success: true,
		Message: "User info retrieved",
		User: &types.UserResponse{
			ID:       userWithRole.ID,
			Name:     userWithRole.Name,
			Email:    userWithRole.Email,
			Position: userWithRole.Position,
			Role: types.RoleInfo{
				ID:          userWithRole.Role.ID,
				Name:        userWithRole.Role.Name,
				DisplayName: userWithRole.Role.DisplayName,
			},
			Permissions: userWithRole.GetPermissions(),
		},
	})
}

// RefreshToken refreshes the JWT token
func (h *AuthHandler) RefreshToken(c *fiber.Ctx) error {
	authHeader := c.Get("Authorization")
	if authHeader == "" {
		return c.Status(401).JSON(types.AuthResponse{
			Success: false,
			Message: "Authorization header required",
		})
	}

	tokenString := authHeader[7:] // Remove "Bearer " prefix
	newToken, err := h.authService.RefreshToken(tokenString)
	if err != nil {
		return c.Status(401).JSON(types.AuthResponse{
			Success: false,
			Message: "Invalid or expired token",
		})
	}

	return c.JSON(types.AuthResponse{
		Success: true,
		Message: "Token refreshed successfully",
		Token:   newToken,
	})
}
