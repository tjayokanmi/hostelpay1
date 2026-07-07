# HostelPay 🏨💳

Eliminating rent reconciliation nightmares with automated, zero-drift payments.

Built for the DevCareer × Nomba Hackathon.

## Why HostelPay?

Paying hostel rent is often a frustrating and manual process. Students make payments, but hostel managers still have to verify them through bank statements, spreadsheets, or repeated follow-ups. This creates delays, human error, and unnecessary friction for both students and administrators.

HostelPay solves this by providing a simple, secure payment experience that connects student checkout, automated payment confirmation, and a real-time manager ledger in one flow.

## What HostelPay Does

HostelPay is a complete MVP for hostel rent collection built with Go and the Nomba payment platform. It allows students to:

- initiate hostel rent payments from a simple checkout flow
- authenticate into a personal account dashboard
- view their payment history
- opt into recurring billing for future rent charges

It also gives managers:

- a protected dashboard to review payments
- filtering by block and floor level
- a live view of payment status and monthly revenue

## Core Features

- Zero-drift financial handling using integer-based Kobo calculations
- Secure Nomba checkout integration for rent payments
- Webhook-based payment verification and status updates
- PostgreSQL-backed payment tracking and audit history
- Authenticated student and manager experiences
- Recurring subscription support for future charges

## Tech Stack

- Backend: Go (Golang 1.25)
- Database: PostgreSQL with pgx connection pooling
- Frontend: Go server-side HTML templates
- Payments: Nomba Sandbox/Production API
- Security: session-based authentication and webhook signature verification

## System Architecture

1. Student selects their hostel details and occupancy type.
2. The backend calculates the exact amount and stores a pending transaction.
3. HostelPay requests a checkout link from Nomba and redirects the student.
4. Once payment succeeds, Nomba sends a webhook to the backend.
5. The webhook is verified and the payment status is updated in PostgreSQL.
6. Managers can immediately review the updated ledger in the dashboard.

## Project Structure

- cmd/hostelpay - application entry point and HTTP handlers
- internal/auth - authentication helpers
- internal/models - payment, student, and subscription models
- internal/nomba - Nomba API client and webhook verification
- internal/repository - database access layer
- migrations - SQL migrations for the database schema
- templates - HTML templates for the checkout, login, and dashboard pages

## Prerequisites

- Go 1.25 or newer
- PostgreSQL running locally or remotely
- A Nomba developer account with sandbox or production credentials

## Local Setup

1. Clone the repository:

   ```bash
   git clone <repository-url>
   cd hostelpay1
   ```

2. Create a PostgreSQL database and set the connection string:

   ```bash
   export DATABASE_URL="postgres://user:password@localhost:5432/hostelpay"
   ```

3. Apply the database migration:

   ```bash
   psql "$DATABASE_URL" -f migrations/000001_init_schema.up.sql
   ```

4. Configure the required environment variables:

   ```bash
   export PORT=8080
   export NOMBA_ENV=sandbox
   export NOMBA_CLIENT_ID="your-client-id"
   export NOMBA_CLIENT_SECRET="your-client-secret"
   export NOMBA_PARENT_ACCOUNT_ID="your-parent-account-id"
   export NOMBA_SUB_ACCOUNT_ID="your-sub-account-id"
   export NOMBA_SIGNATURE_KEY="your-webhook-signing-key"
   export MANAGER_USERNAME="manager"
   export MANAGER_PASSWORD="change-me"
   ```

5. Run the application:

   ```bash
   go run ./cmd/hostelpay
   ```

6. Open the app:

   - Student checkout: http://localhost:8080/
   - Student login: http://localhost:8080/login
   - Manager dashboard: http://localhost:8080/manager/dashboard

## Main Routes

- GET / - checkout page
- POST /checkout/initialize - create a payment session and start checkout
- GET /checkout/callback - confirmation page after payment flow
- POST /checkout/webhook - Nomba webhook endpoint
- GET /login - student login page
- POST /login - student authentication
- GET /account - student account dashboard
- GET /manager/dashboard - manager ledger dashboard
- POST /admin/run-billing-cycle - manually trigger recurring charges
- GET /health - health check endpoint

## Finished Project Status

HostelPay is now built and ready as a functional MVP for hostel rent payments. The project includes the full end-to-end payment flow, secure webhook handling, authentication, and a manager dashboard for reviewing transactions.

## Built By

Built for the DevCareer × Nomba Hackathon as a finished, working solution.
