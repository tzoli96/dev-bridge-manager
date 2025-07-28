package services

import (
	"errors"
	"os"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type JWTClaims struct {
	UserID      uint     `json:"user_id"`
	Email       string   `json:"email"`
	Name        string   `json:"name"`
	RoleID      uint     `json:"role_id"`
	RoleName    string   `json:"role_name"`
	Permissions []string `json:"permissions"`
	jwt.RegisteredClaims
}

type AuthService struct {
	secretKey         []byte
	permissionService *PermissionService
}

func NewAuthService() *AuthService {
	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		secret = "default-secret-key-change-in-production"
	}

	return &AuthService{
		secretKey:         []byte(secret),
		permissionService: NewPermissionService(),
	}
}

// GenerateToken creates a new JWT token with role and permissions
func (s *AuthService) GenerateToken(userID uint, email, name string) (string, error) {
	// Get user with role and permissions
	user, err := s.permissionService.GetUserWithPermissions(userID)
	if err != nil {
		return "", err
	}

	expirationTime := time.Now().Add(24 * time.Hour)
	if expStr := os.Getenv("JWT_EXPIRATION_HOURS"); expStr != "" {
		if hours, err := strconv.Atoi(expStr); err == nil {
			expirationTime = time.Now().Add(time.Duration(hours) * time.Hour)
		}
	}

	claims := &JWTClaims{
		UserID:      userID,
		Email:       email,
		Name:        name,
		RoleID:      user.RoleID,
		RoleName:    user.Role.Name,
		Permissions: user.GetPermissions(),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
			Issuer:    "dev-bridge-manager",
			Subject:   strconv.Itoa(int(userID)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(s.secretKey)
}

// ValidateToken validates and parses JWT token
func (s *AuthService) ValidateToken(tokenString string) (*JWTClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &JWTClaims{}, func(token *jwt.Token) (interface{}, error) {
		return s.secretKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*JWTClaims); ok && token.Valid {
		return claims, nil
	}

	return nil, errors.New("invalid token")
}

// RefreshToken creates a new token with updated permissions
func (s *AuthService) RefreshToken(oldTokenString string) (string, error) {
	claims, err := s.ValidateToken(oldTokenString)
	if err != nil {
		return "", err
	}

	// Generate new token with current permissions
	return s.GenerateToken(claims.UserID, claims.Email, claims.Name)
}
