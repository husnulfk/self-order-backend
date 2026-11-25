package seed

import (
	"fmt"
	"log"

	"selforder/backend/internal/models"
	"selforder/backend/internal/util"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type menuSeed struct {
	name  string
	desc  string
	price int64
}

type catSeed struct {
	name  string
	order int
	items []menuSeed
}

// Run seeds the minimum data needed for an MVP demo, but only into an empty
// database (each seeder is a no-op once its table already has rows).
func Run(db *gorm.DB, adminUsername, adminPassword string) error {
	if err := seedAdmin(db, adminUsername, adminPassword); err != nil {
		return err
	}
	if err := seedCategoriesAndMenu(db); err != nil {
		return err
	}
	if err := seedTables(db); err != nil {
		return err
	}
	return nil
}

func seedAdmin(db *gorm.DB, username, password string) error {
	var count int64
	if err := db.Model(&models.User{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	admin := models.User{
		Username:     username,
		PasswordHash: string(hash),
		Role:         "admin",
	}
	if err := db.Create(&admin).Error; err != nil {
		return err
	}
	log.Printf("seeded admin user %q (role=admin)", username)
	return nil
}

func seedCategoriesAndMenu(db *gorm.DB) error {
	var count int64
	if err := db.Model(&models.Category{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	cats := []catSeed{
		{"Kopi", 1, []menuSeed{
			{"Espresso", "Single shot", 18000},
			{"Americano", "Espresso + air", 20000},
			{"Cappuccino", "Espresso + susu", 26000},
			{"Caffe Latte", "Espresso + susu banyak", 28000},
		}},
		{"Non-Kopi", 2, []menuSeed{
			{"Matcha Latte", "Matcha + susu", 30000},
			{"Cokelat Panas", "Cokelat + susu", 25000},
			{"Lemon Tea", "Teh lemon dingin", 22000},
		}},
		{"Makanan", 3, []menuSeed{
			{"Nasi Goreng", "Nasi goreng spesial", 32000},
			{"Mie Goreng", "Mie goreng", 30000},
			{"Chicken Katsu", "Ayam katsu + nasi", 35000},
		}},
		{"Snack", 4, []menuSeed{
			{"Kentang Goreng", "French fries", 18000},
			{"Croissant", "Butter croissant", 22000},
			{"Roti Bakar", "Roti bakar cokelat", 20000},
		}},
	}

	// Batch-insert semua kategori sekaligus (1 query).
	catModels := make([]models.Category, len(cats))
	for i, cs := range cats {
		catModels[i] = models.Category{Name: cs.name, SortOrder: cs.order}
	}
	if err := db.Create(&catModels).Error; err != nil {
		return err
	}

	var allItems []models.MenuItem
	for i, cs := range cats {
		for _, it := range cs.items {
			allItems = append(allItems, models.MenuItem{
				CategoryID:  catModels[i].ID,
				Name:        it.name,
				Description: it.desc,
				Price:       it.price,
				IsAvailable: true,
				IsActive:    true,
			})
		}
	}
	if len(allItems) > 0 {
		if err := db.Create(&allItems).Error; err != nil {
			return err
		}
	}
	log.Println("seeded categories and menu items")
	return nil
}

func seedTables(db *gorm.DB) error {
	var count int64
	if err := db.Model(&models.Table{}).Count(&count).Error; err != nil {
		return err
	}
	if count > 0 {
		return nil
	}

	for i := 1; i <= 6; i++ {
		t := models.Table{
			Number:   fmt.Sprintf("%d", i),
			Token:    util.TableToken(),
			IsActive: true,
		}
		if err := db.Create(&t).Error; err != nil {
			return err
		}
	}
	log.Println("seeded 6 tables")
	return nil
}
