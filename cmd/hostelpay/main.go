package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"time"

	"hostelpay/internal/models"
	"hostelpay/internal/repository"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

// calculateRentKobo applies the pricing rules entirely in integer kobo
func calculateRentKobo(occupancyType string) (int64, error) {
	const baseRateKobo int64 = 3_000_000 // ₦30,000

	switch occupancyType {
	case "single":
		return baseRateKobo, nil
	case "shared":
		return baseRateKobo / 2, nil
	default:
		return 0, fmt.Errorf("invalid occupancy type: %s", occupancyType)
	}
}

// generateOrderReference builds a unique reference: timestamp + random suffix
func generateOrderReference() (string, error) {
	b := make([]byte, 4)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	timestamp := time.Now().Format("20060102150405")
	return fmt.Sprintf("hp-ref-%s-%x", timestamp, b), nil
}

func checkoutFormHandler(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, nil); err != nil {
			log.Printf("template execute error: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, err := buf.WriteTo(w); err != nil {
			log.Printf("response write error: %v", err)
		}
	}
}

func checkoutInitializeHandler(repo *repository.PaymentRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			log.Printf("form parse error: %v", err)
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			return
		}

		studentID := r.FormValue("student_identifier")
		block := r.FormValue("block")
		floor := r.FormValue("floor_level")
		room := r.FormValue("room_number")
		occupancy := r.FormValue("occupancy_type")

		if studentID == "" || block == "" || floor == "" || room == "" {
			http.Error(w, "All fields are required", http.StatusBadRequest)
			return
		}

		amountKobo, err := calculateRentKobo(occupancy)
		if err != nil {
			log.Printf("pricing error: %v", err)
			http.Error(w, "Invalid occupancy selection", http.StatusBadRequest)
			return
		}

		orderReference, err := generateOrderReference()
		if err != nil {
			log.Printf("order reference generation error: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Calculate exact Naira layout as a safe string
		nairaPart := amountKobo / 100
		koboPart := amountKobo % 100
		safeNumericStr := fmt.Sprintf("%d.%02d", nairaPart, koboPart)

		// Map to the Go struct ensuring the StudentIdentifier is captured
		payment := models.Payment{
			OrderReference:    orderReference,
			StudentIdentifier: studentID,
			Block:             block,
			FloorLevel:        floor,
			RoomNumber:        room,
			OccupancyType:     occupancy,
			AmountPaid:        safeNumericStr,
		}

		if err := repo.CreatePayment(r.Context(), payment); err != nil {
			log.Printf("database insert error: %v", err)
			http.Error(w, "Failed to save payment record", http.StatusInternalServerError)
			return
		}

		log.Printf("✅ DB SAVED | Ref: %s | Amount: ₦%s | Student: %s", orderReference, safeNumericStr, studentID)
		fmt.Fprintf(w, "Success! Record saved to database. Ref: %s", orderReference)
	}
}

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, relying on system environment variables")
	}

	dbURL := os.Getenv("DATABASE_URL")

	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}

	dbPool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer dbPool.Close()

	log.Println("✅ Successfully connected to PostgreSQL")

	repo := repository.NewPaymentRepository(dbPool)
	tmpl := template.Must(template.ParseFiles("templates/checkout.html"))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", checkoutFormHandler(tmpl))
	mux.HandleFunc("POST /checkout/initialize", checkoutInitializeHandler(repo))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting HostelPay server on http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Server crashed: %v", err)
	}
}
