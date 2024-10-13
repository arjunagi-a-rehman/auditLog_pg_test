package main

import (
	"log"
	"net/http"

	auditlog "github.com/arjunagi-a-rehman/gormAuditlog"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Todo struct {
	ID          uint   `json:"id" gorm:"primaryKey"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Completed   bool   `json:"completed"`
	UserID      string `json:"user_id"`
}

var db *gorm.DB
var auditlogger *auditlog.AuditLogger

func main() {
	// Database connection
	dsn := "host=localhost user=zander password=1234 dbname=test port=5432 sslmode=disable TimeZone=UTC"
	var err error
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})

	if err != nil {
		panic("failed to connect database")
	}

	// Migrate the schema
	db.AutoMigrate(&Todo{})

	auditlogger, err = auditlog.NewAuditLogger(db, &Todo{})
	if err != nil {
		panic(err)
	}

	err = auditlogger.CreateAuditLogTable()
	if err != nil {
		log.Printf("Error creating audit log table: %v", err)
	}

	err = auditlogger.CreateTriggers()
	if err != nil {
		log.Printf("Error creating triggers: %v", err)
	}

	// Setup Gin router
	r := gin.Default()

	// Middleware to check for User-ID header
	r.Use(userIDMiddleware())

	// CRUD routes
	r.POST("/todos", createTodo)
	r.GET("/todos", getTodos)
	r.GET("/todos/:id", getTodo)
	r.PUT("/todos/:id", updateTodo)
	r.DELETE("/todos/:id", deleteTodo)

	r.Run(":8080")
}

func userIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetHeader("User-ID")
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "User-ID header is required"})
			c.Abort()
			return
		}

		c.Set("userID", userID)
		c.Next()
	}
}

func createTodo(c *gin.Context) {
	var todo Todo
	if err := c.ShouldBindJSON(&todo); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := c.GetString("userID")
	todo.UserID = userID

	auditlogger.SetPerformedBy(db, userID)
	if err := db.Create(&todo).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, todo)
}

func getTodos(c *gin.Context) {
	var todos []Todo
	userID := c.GetString("userID")

	if err := db.Where("user_id = ?", userID).Find(&todos).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, todos)
}

func getTodo(c *gin.Context) {
	var todo Todo
	userID := c.GetString("userID")
	if err := db.Where("id = ? AND user_id = ?", c.Param("id"), userID).First(&todo).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
		return
	}
	c.JSON(http.StatusOK, todo)
}

func updateTodo(c *gin.Context) {
	var todo Todo
	userID := c.GetString("userID")
	if err := db.Where("id = ? ", c.Param("id")).First(&todo).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
		return
	}

	if err := c.ShouldBindJSON(&todo); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	todo.UserID = userID // Ensure UserID doesn't change

	auditlogger.SetPerformedBy(db, userID)

	if err := db.Save(&todo).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, todo)
}

func deleteTodo(c *gin.Context) {
	var todo Todo
	userID := c.GetString("userID")
	if err := db.Where("id = ? AND user_id = ?", c.Param("id"), userID).First(&todo).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Todo not found"})
		return
	}

	auditlogger.SetPerformedBy(db, userID)
	if err := db.Delete(&todo).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Todo deleted successfully"})
}
