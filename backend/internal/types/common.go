package types

// Common response types used across handlers

type RoleInfo struct {
	ID          uint   `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

type UserResponse struct {
	ID          uint     `json:"id"`
	Name        string   `json:"name"`
	Email       string   `json:"email"`
	Position    string   `json:"position"`
	Role        RoleInfo `json:"role"`
	Permissions []string `json:"permissions,omitempty"`
}

type UserListResponse struct {
	Success bool           `json:"success"`
	Message string         `json:"message"`
	Users   []UserResponse `json:"users,omitempty"`
	User    *UserResponse  `json:"user,omitempty"`
}

type AuthResponse struct {
	Success bool          `json:"success"`
	Message string        `json:"message"`
	Token   string        `json:"token,omitempty"`
	User    *UserResponse `json:"user,omitempty"`
}
