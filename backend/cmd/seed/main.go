package main

import (
	"dev-bridge-manager/internal/database"
	"dev-bridge-manager/internal/models"
	"fmt"
	"github.com/joho/godotenv"
	"golang.org/x/crypto/bcrypt"
	"log"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	// Connect to database
	database.Connect()

	// Create super admin user
	createSuperAdmin()

	fmt.Println("Database seeded successfully!")
}

func createSuperAdmin() {
	// Check if super admin already exists
	var existingAdmin models.User
	result := database.GetDB().Joins("Role").Where("users.email = ? OR roles.name = ?", "admin@system.com", "super_admin").First(&existingAdmin)

	if result.Error == nil {
		fmt.Println("Super admin already exists")
		return
	}

	// Get super_admin role
	var superAdminRole models.Role
	if err := database.GetDB().Where("name = ?", "super_admin").First(&superAdminRole).Error; err != nil {
		log.Fatal("Super admin role not found. Please run migrations first.")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte("SuperAdmin123!"), bcrypt.DefaultCost)
	if err != nil {
		log.Fatal("Failed to hash password:", err)
	}

	// Create super admin user
	superAdmin := models.User{
		Name:     "System Administrator",
		Email:    "admin@system.com",
		Password: string(hashedPassword),
		Position: "System Administrator",
		RoleID:   superAdminRole.ID,
	}

	if err := database.GetDB().Create(&superAdmin).Error; err != nil {
		log.Fatal("Failed to create super admin:", err)
	}

	fmt.Printf("Super admin created successfully!\n")
	fmt.Printf("Email: %s\n", superAdmin.Email)
	fmt.Printf("Password: SuperAdmin123!\n")
	fmt.Printf("Please change the password after first login!\n")
}
