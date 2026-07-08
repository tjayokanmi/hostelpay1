package main

import (
	"bytes"
	"html/template"
	"log"
	"net/http"
	"time"

	"hostelpay/internal/auth"
	"hostelpay/internal/models"
	"hostelpay/internal/repository"
)

const sessionCookieName = "session_token"
const sessionDuration = 30 * 24 * time.Hour // 30 days

func isSecureRequest(r *http.Request) bool {
	return r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https"
}

func setSessionCookie(w http.ResponseWriter, r *http.Request, token string, expiresAt time.Time) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Expires:  expiresAt,
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
}

func clearSessionCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Expires:  time.Unix(0, 0),
		HttpOnly: true,
		Secure:   isSecureRequest(r),
		SameSite: http.SameSiteLaxMode,
		Path:     "/",
	})
}

func requireAuth(next func(w http.ResponseWriter, r *http.Request, studentIdentifier string), studentRepo *repository.StudentRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		session, err := studentRepo.GetSession(r.Context(), cookie.Value)
		if err != nil {
			log.Printf("session lookup error: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		if session == nil {
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}

		identifier, err := studentRepo.GetStudentIdentifierByID(r.Context(), session.StudentID)
		if err != nil {
			log.Printf("student lookup error: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		next(w, r, identifier)
	}
}

func loginFormHandler(tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, nil); err != nil {
			log.Printf("login template error: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		buf.WriteTo(w)
	}
}

func loginHandler(studentRepo *repository.StudentRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			http.Error(w, "Failed to parse form", http.StatusBadRequest)
			return
		}

		identifier := r.FormValue("student_identifier")
		password := r.FormValue("password")

		student, err := studentRepo.GetStudentByIdentifier(r.Context(), identifier)
		if err != nil {
			log.Printf("login lookup error: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		if student == nil || !auth.CheckPassword(password, student.PasswordHash) {
			http.Error(w, "Invalid identifier or password", http.StatusUnauthorized)
			return
		}

		token, err := auth.GenerateSessionToken()
		if err != nil {
			log.Printf("session token error: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		expiresAt := time.Now().Add(sessionDuration)
		if err := studentRepo.CreateSession(r.Context(), student.ID, token, expiresAt); err != nil {
			log.Printf("create session error: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		setSessionCookie(w, r, token, expiresAt)
		http.Redirect(w, r, "/account", http.StatusSeeOther)
	}
}

func logoutHandler(studentRepo *repository.StudentRepository) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie(sessionCookieName)
		if err == nil {
			if err := studentRepo.DeleteSession(r.Context(), cookie.Value); err != nil {
				log.Printf("logout session delete error: %v", err)
			}
		}
		clearSessionCookie(w, r)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
	}
}

func accountDashboardHandler(paymentRepo *repository.PaymentRepository, tmpl *template.Template) func(w http.ResponseWriter, r *http.Request, studentIdentifier string) {
	return func(w http.ResponseWriter, r *http.Request, studentIdentifier string) {
		payments, err := paymentRepo.ListPaymentsByStudent(r.Context(), studentIdentifier)
		if err != nil {
			log.Printf("account dashboard query error: %v", err)
			http.Error(w, "Failed to load your transactions", http.StatusInternalServerError)
			return
		}

		subscribedStatus := r.URL.Query().Get("subscribed")

		data := struct {
			StudentIdentifier string
			Payments          []models.Payment
			SubscribedStatus  string
		}{
			StudentIdentifier: studentIdentifier,
			Payments:          payments,
			SubscribedStatus:  subscribedStatus,
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			log.Printf("account dashboard template error: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		buf.WriteTo(w)
	}
}
