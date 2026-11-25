package main

import (
	"log"
	"time"

	"selforder/backend/internal/config"
	"selforder/backend/internal/database"
	"selforder/backend/internal/handlers"
	"selforder/backend/internal/jobs"
	"selforder/backend/internal/middleware"
	"selforder/backend/internal/payment"
	"selforder/backend/internal/seed"

	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.Load()
	// log.Printf("midtrans key len=%d prefix=%.12s prod=%v gateway=%s",
	// 	len(cfg.MidtransServerKey), cfg.MidtransServerKey, cfg.MidtransProduction, cfg.PaymentGateway)

	db, err := database.Connect(cfg.DatabaseURL, cfg.AutoMigrate)
	if err != nil {
		log.Fatalf("database: %v", err)
	}

	if cfg.Seed {
		if err := seed.Run(db, cfg.AdminUsername, cfg.AdminPassword); err != nil {
			log.Fatalf("seed: %v", err)
		}
	}

	gw := payment.New(
		cfg.PaymentGateway,
		cfg.PaymentWebhookSecret,
		cfg.MidtransServerKey,
		cfg.MidtransProduction,
	)
	h := handlers.New(
		db,
		gw,
		cfg.BaseURL,
		time.Duration(cfg.OrderExpiryMinutes)*time.Minute,
		time.Duration(cfg.JWTExpiryHours)*time.Hour,
		cfg.JWTSecret,
	)

	// Auto-expire stale pending_payment orders every minute (PRD §8).
	jobs.StartOrderExpiry(db, time.Minute)

	r := gin.Default()
	r.Use(middleware.CORS(cfg.AllowedOrigins))

	api := r.Group("/api")

	// Public (no login)
	pub := api.Group("/public")
	{
		pub.GET("/tables/:token", h.GetTableByToken)
		pub.GET("/menu", h.GetMenu)
		pub.POST("/orders", h.CreateOrder)
		pub.GET("/orders/:code", h.GetOrderByCode)
	}

	// Payment gateway callback (mendukung dengan atau tanpa prefix /api)
	api.POST("/webhooks/payment", h.PaymentWebhook)
	r.POST("/webhooks/payment", h.PaymentWebhook)

	// Auth
	auth := api.Group("/auth")
	{
		auth.POST("/login", h.Login)
	}

	// Kasir / Admin (JWT)
	admin := api.Group("/admin")
	admin.Use(middleware.JWT(cfg.JWTSecret))
	{
		// Cashier + admin: order board
		orders := admin.Group("")
		orders.Use(middleware.RequireRole("cashier", "admin"))
		{
			orders.GET("/orders", h.ListOrders)
			orders.PATCH("/orders/:id/status", h.UpdateOrderStatus)
		}

		// Admin only: menu + tables
		adminOnly := admin.Group("")
		adminOnly.Use(middleware.RequireRole("admin"))
		{
			adminOnly.GET("/menu-items", h.ListMenuItems)
			adminOnly.POST("/menu-items", h.CreateMenuItem)
			adminOnly.PATCH("/menu-items/:id", h.UpdateMenuItem)
			adminOnly.DELETE("/menu-items/:id", h.DeleteMenuItem)
			adminOnly.PATCH("/menu-items/:id/availability", h.ToggleAvailability)

			adminOnly.GET("/categories", h.ListCategories)
			adminOnly.POST("/categories", h.CreateCategory)

			adminOnly.GET("/tables", h.ListTables)
			adminOnly.POST("/tables", h.CreateTable)
			adminOnly.PATCH("/tables/:id", h.UpdateTable)
			adminOnly.GET("/tables/:id/qr", h.TableQR)
		}
	}

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	log.Printf("selforder backend listening on :%s (gateway=%s)", cfg.Port, cfg.PaymentGateway)
	if err := r.Run(":" + cfg.Port); err != nil {
		log.Fatal(err)
	}
}
