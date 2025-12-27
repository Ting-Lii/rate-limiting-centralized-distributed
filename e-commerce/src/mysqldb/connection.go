package mysqldb

import (
	"log"

	"database/sql"
	"fmt"
	"os"

	_ "github.com/go-sql-driver/mysql"
)

func Connect() *sql.DB {
	dbHost := os.Getenv("DB_HOST")
	dbPort := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_NAME")
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true",
		dbUser, dbPassword, dbHost, dbPort, dbName)

	// Connect to MySQL
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	// Test the connection
	if err := db.Ping(); err != nil {
		log.Fatal("Database is not reachable:", err)
	}

	log.Println("Successfully connected to MySQL database")

	return db
}

func InitSchema(db *sql.DB) error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS shopping_carts (
            id INT AUTO_INCREMENT PRIMARY KEY,
            customer_id INT NOT NULL,
            created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
            INDEX idx_customer (customer_id)
        )`,

		`CREATE TABLE IF NOT EXISTS cart_items (
            id INT AUTO_INCREMENT PRIMARY KEY,
            cart_id INT NOT NULL,
            product_id INT NOT NULL,
            quantity INT NOT NULL,
            FOREIGN KEY (cart_id) REFERENCES shopping_carts(id) ON DELETE CASCADE,
            UNIQUE KEY unique_cart_product (cart_id, product_id),
            INDEX idx_cart (cart_id)
        )`,
	}

	for _, query := range queries {
		log.Printf("Executing: %s", query[:50]+"...") // Log first 50 chars
		if _, err := db.Exec(query); err != nil {
			return fmt.Errorf("failed to execute schema query: %w", err)
		}
	}

	log.Println("Database schema initialized successfully")
	return nil
}
