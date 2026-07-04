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
	"hostelpay/internal/nomba"
	"hostelpay/internal/repository"

	"github.com/jackc/pgx/v5/pgxpool"
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

func checkoutInitializeHandler(repo *repository.PaymentRepository, nombaClient *nomba.Client) http.HandlerFunc {
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

		if studentID == "" || block == "" || floor == "" || room == "" || occupancy == "" {
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

		nairaPart := amountKobo / 100
		koboPart := amountKobo % 100
		safeNumericStr := fmt.Sprintf("%d.%02d", nairaPart, koboPart)

		payment := models.Payment{
			OrderReference:    orderReference,
			StudentIdentifier: studentID,
			Block:             block,
			FloorLevel:        floor,
			RoomNumber:        room,
			OccupancyType:     occupancy,
			AmountPaid:        safeNumericStr,
		}

		// 1. Save to Database
		if err := repo.CreatePayment(r.Context(), payment); err != nil {
			log.Printf("database insert error: %v", err)
			http.Error(w, "Failed to save payment record", http.StatusInternalServerError)
			return
		}
		log.Printf("✅ DB SAVED | Ref: %s | Amount: ₦%s | Student: %s", orderReference, safeNumericStr, studentID)

		// 2. Fire the Request to Nomba
		req := nomba.CheckoutRequest{
			OrderReference: payment.OrderReference,
			CustomerID:     payment.StudentIdentifier,
			CustomerEmail:  "student@hackathon.com",
			AmountNaira:    payment.AmountPaid,
			CallbackURL:    "http://localhost:8080/checkout/callback",
		}

		result, err := nombaClient.GenerateCheckoutLink(r.Context(), req)
		if err != nil {
			log.Printf("nomba checkout error: %v", err)
			http.Error(w, "Payment gateway unavailable", http.StatusBadGateway)
			return
		}
		log.Printf("✅ NOMBA LINK GENERATED | Redirecting to: %s", result.CheckoutLink)

		// 3. Redirect the browser to the Nomba payment screen
		http.Redirect(w, r, result.CheckoutLink, http.StatusFound)
	}
}

func main() {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		log.Fatal("DATABASE_URL environment variable is not set")
	}

	dbPool, err := pgxpool.New(context.Background(), dbURL)
	if err != nil {
		log.Fatalf("Failed to create connection pool: %v", err)
	}
	defer dbPool.Close()

	if err := dbPool.Ping(context.Background()); err != nil {
		log.Fatalf("Database ping failed: %v", err)
	}
	log.Println("✅ Successfully connected to PostgreSQL")

	repo := repository.NewPaymentRepository(dbPool)

	nombaClient := nomba.NewClient(
		os.Getenv("NOMBA_CLIENT_ID"),
		os.Getenv("NOMBA_CLIENT_SECRET"),
		os.Getenv("NOMBA_PARENT_ACCOUNT_ID"),
		os.Getenv("NOMBA_SUB_ACCOUNT_ID"),
	)

	tmpl := template.Must(template.ParseFiles("templates/checkout.html"))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", checkoutFormHandler(tmpl))
	mux.HandleFunc("POST /checkout/initialize", checkoutInitializeHandler(repo, nombaClient))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting HostelPay server on http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Server crashed: %v", err)
	}
}
