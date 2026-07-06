package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"hostelpay/internal/auth"
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

// 1. THE FORM HANDLER
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

// 2. THE INITIALIZATION HANDLER (Sends data to DB and Nomba)
func checkoutInitializeHandler(repo *repository.PaymentRepository, studentRepo *repository.StudentRepository, nombaClient *nomba.Client) http.HandlerFunc {
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
		password := r.FormValue("password")

		if studentID == "" || block == "" || floor == "" || room == "" || occupancy == "" || password == "" {
			http.Error(w, "All fields are required", http.StatusBadRequest)
			return
		}
		if len(password) < 8 {
			http.Error(w, "Password must be at least 8 characters", http.StatusBadRequest)
			return
		}

		// --- Account: create if new, or quietly log in if password matches. Never block payment on this. ---
		existingStudent, err := studentRepo.GetStudentByIdentifier(r.Context(), studentID)
		if err != nil {
			log.Printf("student lookup error: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		var studentDBID string
		if existingStudent == nil {
			passwordHash, err := auth.HashPassword(password)
			if err != nil {
				log.Printf("password hash error: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			studentDBID, err = studentRepo.CreateStudent(r.Context(), studentID, passwordHash)
			if err != nil {
				log.Printf("create student error: %v", err)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}
			log.Printf("✅ ACCOUNT CREATED | Student: %s", studentID)
		} else {
			studentDBID = existingStudent.ID
			if !auth.CheckPassword(password, existingStudent.PasswordHash) {
				log.Printf("password mismatch for existing student %s — proceeding with payment anyway, not logging in", studentID)
			}
		}

		// Log them in (set session cookie) regardless of new/existing, as long as we have a valid studentDBID
		// and (for existing accounts) the password actually matched.
		shouldLogIn := existingStudent == nil || auth.CheckPassword(password, existingStudent.PasswordHash)
		if shouldLogIn {
			token, err := auth.GenerateSessionToken()
			if err != nil {
				log.Printf("session token error: %v", err)
			} else {
				expiresAt := time.Now().Add(sessionDuration)
				if err := studentRepo.CreateSession(r.Context(), studentDBID, token, expiresAt); err != nil {
					log.Printf("create session error: %v", err)
				} else {
					setSessionCookie(w, r, token, expiresAt)
				}
			}
		}

		// --- Existing payment logic, unchanged from here ---
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

		if err := repo.CreatePayment(r.Context(), payment); err != nil {
			log.Printf("database insert error: %v", err)
			http.Error(w, "Failed to save payment record", http.StatusInternalServerError)
			return
		}
		log.Printf("✅ DB SAVED | Ref: %s | Amount: ₦%s", orderReference, safeNumericStr)

		req := nomba.CheckoutRequest{
			OrderReference: payment.OrderReference,
			CustomerID:     payment.StudentIdentifier,
			CustomerEmail:  "student@hackathon.com",
			AmountNaira:    payment.AmountPaid,
			CallbackURL:    "https://hostelpay1.onrender.com/checkout/callback",
		}

		result, err := nombaClient.GenerateCheckoutLink(r.Context(), req)
		if err != nil {
			log.Printf("nomba checkout error: %v", err)
			http.Error(w, "Payment gateway unavailable", http.StatusBadGateway)
			return
		}

		http.Redirect(w, r, result.CheckoutLink, http.StatusFound)
	}
}

// 3. THE CALLBACK HANDLER (Cosmetic UI for the returning user)
// Uses html/template so a crafted orderReference query param can't inject
// script into the page — {{.}} is auto-escaped, unlike raw fmt.Fprintf.
var callbackTmpl = template.Must(template.New("callback").Parse(`
	<!DOCTYPE html>
	<html lang="en">
	<head>
		<meta charset="UTF-8">
		<title>Payment Verification</title>
		<style>
			body { font-family: -apple-system, sans-serif; background-color: #f4f4f9; text-align: center; padding: 50px; }
			.card { background: white; padding: 40px; border-radius: 8px; box-shadow: 0 4px 6px rgba(0,0,0,0.1); max-width: 500px; margin: 0 auto; }
			h1 { color: #2e7d32; }
			.account-link {
				display: inline-block;
				margin-top: 20px;
				background: #2e7d32;
				color: white;
				padding: 10px 24px;
				border-radius: 6px;
				text-decoration: none;
				font-weight: 600;
			}
			.account-link:hover { background: #256428; }
		</style>
	</head>
	<body>
		<div class="card">
			<h1>🎉 Welcome Back!</h1>
			<p>Your transaction has been received and is currently processing.</p>
			<p style="color: #666;">Order Reference: <strong>{{.}}</strong></p>
			<p>You can safely close this window, or view your account below.</p>
			<a href="/account" class="account-link">View My Account</a>
		</div>
	</body>
	</html>
`))

func checkoutCallbackHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		orderRef := r.URL.Query().Get("orderReference")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")

		var buf bytes.Buffer
		if err := callbackTmpl.Execute(&buf, orderRef); err != nil {
			log.Printf("callback template error: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		if _, err := buf.WriteTo(w); err != nil {
			log.Printf("response write error: %v", err)
		}
	}
}

// 4. THE WEBHOOK HANDLER (The secure background trap for Nomba's servers)
func checkoutWebhookHandler(repo *repository.PaymentRepository, signingKey string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			log.Printf("webhook body read error: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		log.Printf("--- INCOMING WEBHOOK HEADERS ---")
		for name, values := range r.Header {
			log.Printf("webhook header: %s = %v", name, values)
		}
		log.Printf("--- RAW WEBHOOK BODY ---")
		log.Printf("%s", string(body))

		receivedSignature := r.Header.Get("nomba-signature")

		if !nomba.VerifySignature(body, receivedSignature, signingKey) {
			log.Printf("⚠️ webhook signature verification FAILED")
		} else {
			log.Printf("✅ webhook signature verified successfully")
		}

		payload, err := nomba.ParseWebhookPayload(body)
		if err != nil {
			log.Printf("webhook parse error: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		// 👇 NEW CODE STARTS HERE — paste this block in
		isNew, err := repo.MarkWebhookProcessed(r.Context(), payload.RequestID)
		if err != nil {
			log.Printf("webhook idempotency check error: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		if !isNew {
			log.Printf("duplicate webhook received (requestId: %s) — skipping", payload.RequestID)
			w.WriteHeader(http.StatusOK)
			return
		}
		// 👆 NEW CODE ENDS HERE

		if payload.EventType != "payment_success" {
			log.Printf("webhook received non-success event: %s — acknowledging, no DB update", payload.EventType)
			w.WriteHeader(http.StatusOK)
			return
		}

		if err := repo.UpdatePaymentStatusByOrderRef(r.Context(), payload.Data.Order.OrderReference, models.StatusSuccess, payload.Data.Transaction.TransactionID); err != nil {
			log.Printf("failed to update payment status: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		log.Printf("✅ PAYMENT CONFIRMED | Ref: %s | Nomba TxID: %s", payload.Data.Order.OrderReference, payload.Data.Transaction.TransactionID)
		w.WriteHeader(http.StatusOK)
	}
}

// 5. THE MANAGER DASHBOARD HANDLERs
var dashboardFuncMap = template.FuncMap{
	"isStalePending": func(status string, created time.Time) bool {
		return status == "PENDING" && time.Since(created) > 30*time.Minute
	},
}

func managerDashboardHandler(repo *repository.PaymentRepository) http.HandlerFunc {
	tmpl := template.Must(template.New("dashboard.html").Funcs(dashboardFuncMap).ParseFiles("templates/dashboard.html"))

	return func(w http.ResponseWriter, r *http.Request) {
		blockFilter := r.URL.Query().Get("block")
		floorFilter := r.URL.Query().Get("floor_level")
		statusFilter := r.URL.Query().Get("status")

		payments, err := repo.GetAllPayments(r.Context(), blockFilter, floorFilter, statusFilter)
		if err != nil {
			log.Printf("dashboard query error: %v", err)
			http.Error(w, "Failed to load dashboard data", http.StatusInternalServerError)
			return
		}

		data := struct {
			Payments       []models.Payment
			SelectedBlock  string
			SelectedFloor  string
			SelectedStatus string
		}{
			Payments:       payments,
			SelectedBlock:  blockFilter,
			SelectedFloor:  floorFilter,
			SelectedStatus: statusFilter,
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			log.Printf("dashboard template error: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if _, err := buf.WriteTo(w); err != nil {
			log.Printf("response write error: %v", err)
		}
	}
}

func healthCheckHandler(dbPool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := dbPool.Ping(r.Context()); err != nil {
			log.Printf("health check failed: %v", err)
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintf(w, `{"status":"unhealthy","error":"database unreachable"}`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, `{"status":"healthy"}`)
	}
}

// basicAuthMiddleware is a low-effort stopgap: single manager username/password
// via HTTP Basic Auth. Not meant to survive past the hackathon — no sessions,
// no per-manager accounts, credentials live in env vars.
func basicAuthMiddleware(next http.HandlerFunc, username, password string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()

		userMatch := subtle.ConstantTimeCompare([]byte(user), []byte(username)) == 1
		passMatch := subtle.ConstantTimeCompare([]byte(pass), []byte(password)) == 1

		if !ok || !userMatch || !passMatch {
			w.Header().Set("WWW-Authenticate", `Basic realm="HostelPay Manager"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		next(w, r)
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
	studentRepo := repository.NewStudentRepository(dbPool)
	loginTmpl := template.Must(template.ParseFiles("templates/login.html"))
	accountTmpl := template.Must(template.ParseFiles("templates/account_dashboard.html"))
	tmpl := template.Must(template.ParseFiles("templates/checkout.html"))

	mux := http.NewServeMux()
	mux.HandleFunc("GET /{$}", checkoutFormHandler(tmpl))
	mux.HandleFunc("POST /checkout/initialize", checkoutInitializeHandler(repo, studentRepo, nombaClient))
	mux.HandleFunc("GET /checkout/callback", checkoutCallbackHandler())
	mux.HandleFunc("POST /checkout/webhook", checkoutWebhookHandler(repo, os.Getenv("NOMBA_SIGNATURE_KEY")))
	mux.HandleFunc(
		"GET /manager/dashboard",
		basicAuthMiddleware(managerDashboardHandler(repo), os.Getenv("MANAGER_USERNAME"), os.Getenv("MANAGER_PASSWORD")),
	)
	mux.HandleFunc("GET /health", healthCheckHandler(dbPool))
	mux.HandleFunc("GET /login", loginFormHandler(loginTmpl))
	mux.HandleFunc("POST /login", loginHandler(studentRepo))
	mux.HandleFunc("POST /logout", logoutHandler(studentRepo))
	mux.HandleFunc("GET /account", requireAuth(accountDashboardHandler(repo, accountTmpl), studentRepo))
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
    w.WriteHeader(http.StatusOK)
    w.Write([]byte("ok"))
})
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Starting HostelPay server on http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatalf("Server crashed: %v", err)
	}
}
