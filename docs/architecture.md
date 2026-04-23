---
stepsCompleted: [1, 2, 3, 4, 5, 6, 7, 8]
lastStep: 8
status: 'complete'
completedAt: '2026-04-13'
inputDocuments: ['/Volumes/SSD/mutqin/docs/prd.md', '/Volumes/SSD/mutqin/docs/epics.md', '/Volumes/SSD/mutqin/docs/implementation-readiness-report-2026-04-13.md', '/Volumes/SSD/mutqin/docs/ux-design-specification.md']
updatedAt: '2026-04-14'
uxIntegration: true
workflowType: 'architecture'
project_name: 'Mutqin (مُتقِن)'
user_name: 'Ilyas'
date: '2026-04-13'
---

# Architecture Decision Document

_This document builds collaboratively through step-by-step discovery. Sections are appended as we work through each architectural decision together._

## Project Context Analysis

### Requirements Overview

**Functional Requirements:**
47 FRs across 9 capability areas. The system has three primary user flows:
1. **Platform management flow** (FR1-5): Super Admin creates/manages centers — low frequency, admin-only
2. **Center setup flow** (FR6-13, FR14-24): Center Admin builds digital presence and organizes halaqat — medium frequency, setup-oriented
3. **Daily teaching flow** (FR25-34): Teacher records recitation and attendance — high frequency, speed-critical, offline-required

Architecturally, flow #3 is the most demanding — it runs on the worst devices, in the worst connectivity, and must be the fastest experience.

**Non-Functional Requirements:**
20 NFRs with the following architectural weight:

| NFR Category | Architectural Impact |
|-------------|---------------------|
| Performance (NFR1-6) | Constrains framework choice, mandates code splitting, requires local caching |
| Security (NFR7-12) | Drives auth design, tenant isolation strategy, API security model |
| Scalability (NFR13-15) | Shapes database design, but 50 centers on a VPS is modest — don't over-engineer |
| Reliability (NFR16-20) | Drives offline architecture — the most complex technical challenge |

**Scale & Complexity:**

- Primary domain: Full-stack PWA (REST API + Progressive Web App)
- Complexity level: Medium
- Estimated architectural components: 6 (API server, PostgreSQL, PWA frontend, Service Worker, SMS gateway, static file storage)

### Technical Constraints & Dependencies

- **No Supabase** — self-hosted PostgreSQL on VPS ($5-10/month)
- **No heavy frameworks** — bundle must stay under 500KB compressed
- **Phone OTP only** — requires SMS gateway integration from day one
- **Quran data** — 114 surahs + 6,236 ayat must be seeded and cached locally
- **Dual-direction layout** — Arabic RTL + Somali LTR in same app
- **M-Pesa payment** — deferred but architecture must not block future integration

### Cross-Cutting Concerns Identified

1. **Tenant isolation** — every API endpoint, every query, every cache entry must be scoped to organization_id
2. **Offline capability** — Service Worker + IndexedDB must handle recitation, attendance, student lists, and Quran data
3. **Sync conflict resolution** — last-write-wins with conflict logging across all mutable entities
4. **i18n/RTL** — must be built into the component architecture from day one, not retrofitted
5. **Performance budget** — <500KB JS, <3s load, <200ms tap response — constrains every technology choice
6. **Auth chain** — Super Admin → invite → Center Admin → invite → Teacher — single OTP-based auth system serving three roles

## Starter Template Evaluation

### Primary Technology Domain

Full-stack monorepo: Go backend (API) + React TypeScript frontend (PWA). Chosen for minimal resource consumption, excellent performance characteristics, and clean separation of concerns — critical for a self-hosted $5/month VPS serving East African markets on 3G.

### Starter Options Considered

**Option 1: Next.js App Router (Rejected)**
- Bundle too heavy for 500KB budget (~300-400KB framework baseline)
- Offline PWA support is bolted on, not native
- Node.js runtime consumes 150-300MB RAM — expensive on $5 VPS
- Overkill for this project's needs

**Option 2: TanStack Start (Rejected)**
- Still in beta — too risky for a 2-person team
- Good offline support via TanStack Query, but framework instability concern

**Option 3: Vite + React + Hono (Considered)**
- Lightweight and performant, but Hono is JavaScript — same runtime cost as Node.js
- No significant advantage over Go for backend

**Option 4: Go + Vite React (Selected)**
- Go binary: ~15MB, ~10MB RAM, starts instantly
- Vite React: ~100-150KB bundle, native PWA support
- Best performance-to-cost ratio for self-hosted VPS
- Clean API/frontend separation in monorepo

### Selected Stack

| Layer | Technology | Rationale |
|-------|-----------|-----------|
| **Backend Language** | Go 1.22+ | Minimal resource usage, excellent concurrency, single binary deployment |
| **HTTP Router** | Chi | Lightweight, stdlib-compatible, middleware support |
| **Database** | PostgreSQL 16 | Row-level security, JSON support, proven at scale |
| **DB Access** | sqlc | Type-safe SQL — generates Go code from SQL queries. No ORM magic. |
| **Auth** | Custom JWT + OTP | Go crypto stdlib for JWT signing, SMS gateway for OTP delivery |
| **Frontend Framework** | React 19 + TypeScript | Developer familiarity, massive ecosystem |
| **Build Tool** | Vite 6 | Fast builds, small bundles, tree-shaking |
| **Routing** | TanStack Router | Type-safe, file-based routing |
| **Data Fetching** | TanStack Query | Offline mutation queue, caching, background sync |
| **PWA** | vite-plugin-pwa (Workbox) | Service Worker generation, IndexedDB, offline support |
| **Styling** | Tailwind CSS 4 | Utility-first, small CSS output, RTL support via `dir` attribute |
| **i18n** | i18next + react-i18next | Mature, supports RTL/LTR switching, lazy-loaded translations |
| **Offline Storage** | IndexedDB (via idb) | Structured local storage for offline recitation/attendance |
| **Reverse Proxy** | Caddy | Auto-HTTPS, serves static files, proxies to Go API |

### Monorepo Structure

```
mutqin/
├── api/                    # Go backend
│   ├── cmd/server/         # Entry point
│   ├── internal/
│   │   ├── auth/           # OTP + JWT
│   │   ├── handler/        # HTTP handlers
│   │   ├── middleware/      # Tenant isolation, auth, logging
│   │   ├── model/          # sqlc generated types
│   │   ├── service/        # Business logic
│   │   └── db/             # sqlc queries + migrations
│   ├── sql/
│   │   ├── migrations/     # PostgreSQL migrations
│   │   └── queries/        # sqlc query files
│   ├── go.mod
│   └── go.sum
├── web/                    # React frontend (PWA)
│   ├── src/
│   │   ├── components/     # Reusable UI components
│   │   ├── features/       # Feature modules (recitation, attendance, etc.)
│   │   ├── hooks/          # Custom hooks (useOfflineQueue, useAuth, etc.)
│   │   ├── lib/            # Utilities, API client, i18n config
│   │   ├── locales/        # ar.json, so.json translation files
│   │   ├── routes/         # TanStack Router file-based routes
│   │   └── sw/             # Service Worker logic
│   ├── public/
│   │   └── quran/          # Static Quran JSON data (114 surahs)
│   ├── index.html
│   ├── vite.config.ts
│   ├── package.json
│   └── tsconfig.json
├── Caddyfile               # Reverse proxy config
├── Makefile                # Build, migrate, deploy commands
├── docker-compose.yml      # Local dev (PostgreSQL)
└── README.md
```

### Initialization

No single CLI command creates this stack. Story 1.1 will scaffold manually:

```bash
mkdir mutqin && cd mutqin
mkdir -p api/cmd/server api/internal api/sql/migrations api/sql/queries
cd api && go mod init github.com/ilyas/mutqin-api && cd ..
npm create vite@latest web -- --template react-ts
cd web && npm install @tanstack/react-query @tanstack/react-router tailwindcss i18next react-i18next idb && cd ..
cd web && npm install -D vite-plugin-pwa && cd ..
```

## Core Architectural Decisions

### Decision Priority Analysis

**Critical Decisions (Block Implementation):**
- Data model with multi-tenant isolation via organization_id
- Email OTP authentication (changed from phone OTP for cost)
- REST API with JWT tokens
- Offline-first PWA with IndexedDB + sync queue

**Important Decisions (Shape Architecture):**
- golang-migrate for schema migrations
- Cloudflare R2 for file storage
- Structured JSON logging via Go slog

**Deferred Decisions (Post-MVP):**
- Phone/WhatsApp OTP (add when email proves insufficient)
- M-Pesa payment integration
- Monitoring/alerting stack (Grafana/Loki)
- CDN for static assets

### Data Architecture

**Database:** PostgreSQL 16 on VPS
**Migrations:** golang-migrate with sequential SQL files
**Multi-tenancy:** Single database, `organization_id` on every tenant-scoped table with row-level security

**Schema (Core Tables):**

```sql
organizations (
  id UUID PRIMARY KEY,
  name TEXT NOT NULL,
  slug TEXT UNIQUE NOT NULL,
  city TEXT,
  country TEXT DEFAULT 'SO',
  tier TEXT DEFAULT 'free' CHECK (tier IN ('free', 'asaasi', 'pro')),
  status TEXT DEFAULT 'active' CHECK (status IN ('active', 'suspended')),
  logo_url TEXT,
  description TEXT,
  schedule JSONB,
  created_at TIMESTAMPTZ DEFAULT NOW(),
  updated_at TIMESTAMPTZ DEFAULT NOW()
)

users (
  id UUID PRIMARY KEY,
  phone TEXT,
  email TEXT,
  name TEXT NOT NULL,
  role TEXT NOT NULL CHECK (role IN ('super_admin', 'center_admin', 'teacher')),
  organization_id UUID REFERENCES organizations(id),
  status TEXT DEFAULT 'active',
  language TEXT DEFAULT 'ar',
  created_at TIMESTAMPTZ DEFAULT NOW()
)

invites (
  id UUID PRIMARY KEY,
  organization_id UUID REFERENCES organizations(id),
  role TEXT NOT NULL,
  token TEXT UNIQUE NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  used_at TIMESTAMPTZ,
  created_by UUID REFERENCES users(id)
)

otp_codes (
  id UUID PRIMARY KEY,
  email TEXT NOT NULL,
  code TEXT NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  used BOOLEAN DEFAULT FALSE
)

halaqat (
  id UUID PRIMARY KEY,
  organization_id UUID NOT NULL REFERENCES organizations(id),
  name TEXT NOT NULL,
  teacher_id UUID REFERENCES users(id),
  schedule JSONB,
  max_capacity INT DEFAULT 30,
  status TEXT DEFAULT 'active',
  created_at TIMESTAMPTZ DEFAULT NOW()
)

students (
  id UUID PRIMARY KEY,
  organization_id UUID NOT NULL REFERENCES organizations(id),
  halaqah_id UUID REFERENCES halaqat(id),
  name TEXT NOT NULL,
  age INT,
  parent_phone TEXT,
  parent_email TEXT,
  hifz_level TEXT,
  status TEXT DEFAULT 'active',
  created_at TIMESTAMPTZ DEFAULT NOW()
)

registrations (
  id UUID PRIMARY KEY,
  organization_id UUID NOT NULL REFERENCES organizations(id),
  child_name TEXT NOT NULL,
  child_age INT,
  parent_phone TEXT,
  parent_email TEXT,
  hifz_level TEXT,
  status TEXT DEFAULT 'pending' CHECK (status IN ('pending', 'approved', 'rejected')),
  submitted_at TIMESTAMPTZ DEFAULT NOW()
)

recitations (
  id UUID PRIMARY KEY,
  organization_id UUID NOT NULL REFERENCES organizations(id),
  student_id UUID NOT NULL REFERENCES students(id),
  halaqah_id UUID NOT NULL REFERENCES halaqat(id),
  teacher_id UUID NOT NULL REFERENCES users(id),
  type TEXT NOT NULL CHECK (type IN ('new_hifz', 'near_review', 'far_review')),
  surah_number INT NOT NULL,
  ayah_from INT NOT NULL,
  ayah_to INT NOT NULL,
  grade TEXT NOT NULL CHECK (grade IN ('mumtaz', 'jayyid_jiddan', 'jayyid', 'maqbul', 'daif')),
  notes TEXT,
  recorded_at TIMESTAMPTZ DEFAULT NOW(),
  synced_at TIMESTAMPTZ,
  client_id TEXT
)

attendance (
  id UUID PRIMARY KEY,
  organization_id UUID NOT NULL REFERENCES organizations(id),
  halaqah_id UUID NOT NULL REFERENCES halaqat(id),
  student_id UUID NOT NULL REFERENCES students(id),
  date DATE NOT NULL,
  status TEXT NOT NULL CHECK (status IN ('present', 'absent')),
  recorded_at TIMESTAMPTZ DEFAULT NOW(),
  synced_at TIMESTAMPTZ,
  client_id TEXT
)

announcements (
  id UUID PRIMARY KEY,
  organization_id UUID NOT NULL REFERENCES organizations(id),
  title TEXT NOT NULL,
  body TEXT NOT NULL,
  created_at TIMESTAMPTZ DEFAULT NOW()
)

sync_conflicts (
  id UUID PRIMARY KEY,
  organization_id UUID NOT NULL REFERENCES organizations(id),
  table_name TEXT NOT NULL,
  record_id UUID NOT NULL,
  client_data JSONB NOT NULL,
  server_data JSONB NOT NULL,
  resolved_by TEXT DEFAULT 'last_write_wins',
  created_at TIMESTAMPTZ DEFAULT NOW()
)

audit_log (
  id UUID PRIMARY KEY,
  actor_id UUID REFERENCES users(id),
  action TEXT NOT NULL,
  target_type TEXT,
  target_id UUID,
  details JSONB,
  created_at TIMESTAMPTZ DEFAULT NOW()
)
```

**Quran Reference Data:** Static JSON files bundled with frontend in `web/public/quran/`. Pre-processed from quran.com API at build time. Cached in IndexedDB on first load.

**File Storage:** Cloudflare R2 for center logos. Accessed via presigned URLs generated by Go API.

### Authentication & Security

**Auth Method:** Email OTP (phone OTP deferred to post-MVP)

**Auth Flow:**
1. User enters email → API generates 6-digit code (5-min expiry), stores in `otp_codes`
2. API sends code via email (SMTP — Resend or similar)
3. User enters code → API validates → returns JWT access token + refresh token
4. JWT contains: `user_id`, `organization_id`, `role`
5. All requests include JWT in `Authorization: Bearer` header

**Invite Flow:**
1. Super Admin creates organization → generates invite token (72-hour expiry)
2. Link: `mutqin.app/invite/{token}`
3. User clicks link → enters email → receives OTP → account created with assigned role
4. Center Admin generates teacher invite → same flow with `teacher` role

**JWT Payload:**
```json
{"sub": "user_id", "org": "organization_id", "role": "center_admin", "exp": 1234567890}
```

**Security Middleware (Chi):**
- `AuthMiddleware` — validates JWT
- `TenantMiddleware` — extracts `organization_id` from JWT, injects into context
- `RoleMiddleware(roles ...string)` — checks role against allowed roles
- `AuditMiddleware` — logs super_admin actions

**Data Security:** TLS 1.2+ via Caddy, no passwords (OTP-only), JWT secret as env var, no PII in URLs.

### API & Communication Patterns

**Design:** RESTful JSON API at `https://mutqin.app/api/v1`

**Key Endpoints:**

```
# Auth
POST   /auth/otp/request          {email}
POST   /auth/otp/verify           {email, code}
POST   /auth/refresh              {refresh_token}

# Organizations (Super Admin)
POST   /organizations
GET    /organizations
GET    /organizations/:id
PATCH  /organizations/:id
POST   /organizations/:id/invite

# Public Landing Pages
GET    /public/:slug
GET    /public/:slug/announcements
POST   /public/:slug/register

# Halaqat (Center Admin)
POST   /halaqat
GET    /halaqat
PATCH  /halaqat/:id
POST   /halaqat/:id/enroll
POST   /halaqat/:id/transfer

# Teachers (Center Admin)
POST   /teachers/invite
GET    /teachers
DELETE /teachers/:id

# Students & Registrations (Center Admin)
GET    /students
GET    /students/:id
GET    /registrations
PATCH  /registrations/:id

# Recitation (Teacher)
GET    /recitations/halaqah/:id/students
POST   /recitations
POST   /recitations/batch

# Attendance (Teacher)
POST   /attendance
POST   /attendance/batch
GET    /attendance?halaqah_id=&date_from=&date_to=

# Dashboard (Center Admin)
GET    /dashboard/stats
GET    /dashboard/attendance-trends
GET    /dashboard/recitation-activity

# Sync
POST   /sync/push
GET    /sync/pull?since=timestamp
GET    /sync/conflicts

# Platform (Super Admin)
GET    /platform/stats
GET    /platform/audit-log
```

**Error Format:**
```json
{"error": {"code": "VALIDATION_ERROR", "message": "Child name is required", "details": [{"field": "child_name", "message": "required"}]}}
```

**Batch Sync:** Offline records pushed as arrays to `/sync/push`. Server applies last-write-wins using `recorded_at`. Conflicts logged to `sync_conflicts`.

### Frontend Architecture

**State:** TanStack Query for server state + offline mutations. React context for auth only.

**Offline Flow:**
```
React UI → TanStack Query (cache + mutations) → Go API
                    ↓
              IndexedDB (offline queue + cached data)
```

- Online: fetch from API, cache in memory + IndexedDB
- Offline: mutations queue in IndexedDB, UI updates optimistically
- Reconnect: Service Worker triggers sync → push queue → pull updates

**IndexedDB Stores:** students, recitations_queue, attendance_queue, quran_surahs, quran_ayat, sync_meta

**Component Structure:** Feature-based modules (auth, landing, halaqat, recitation, attendance, dashboard, platform)

### Infrastructure & Deployment

**Server:** Hetzner CX22 (2 vCPU, 4GB RAM, ~€4/month), Ubuntu 24.04, PostgreSQL 16 on same VPS

**Deployment:** SSH → git pull → `go build` → restart systemd service → `npm run build` → copy dist to /var/www/mutqin/

**Caddy:** Reverse proxy to Go API on :8080, serves static frontend files, auto-HTTPS

**Logging:** Go slog JSON to stdout, systemd journal capture

**Monitoring:** `/api/v1/health` endpoint + UptimeRobot free tier

### Decision Impact on Implementation

**Sequence:** PostgreSQL + migrations (1.1) → Email OTP + JWT + RBAC (1.2-1.3) → Org CRUD + invites (Epic 2) → Landing pages (Epic 3) → Halaqat (Epic 4) → Recitation + Quran data (Epic 5) → Attendance (Epic 6) → Offline sync (Epic 7) → Dashboard (Epic 8) → i18n + RTL (Epic 9, framework in 1.1)

**Key dependency:** i18n framework setup in Story 1.1 so all UI text is translation-ready from the start.

## Implementation Patterns & Consistency Rules

### Naming Patterns

**Database:**
- Tables: `snake_case`, plural (`organizations`, `halaqat`, `students`)
- Columns: `snake_case` (`organization_id`, `created_at`, `surah_number`)
- Foreign keys: `{referenced_table_singular}_id` (`teacher_id`, `student_id`)
- Indexes: `idx_{table}_{columns}` (`idx_students_organization_id`)
- Migrations: `{sequence}_{description}.up.sql` / `.down.sql` (`001_create_organizations.up.sql`)

**Go Backend:**
- Packages: lowercase, single word (`handler`, `service`, `middleware`)
- Functions/methods: `PascalCase` exported, `camelCase` unexported (`CreateOrganization`, `validateOTP`)
- Variables: `camelCase` (`orgID`, `userRole`)
- Files: `snake_case.go` (`organization_handler.go`, `auth_middleware.go`)
- Errors: `Err` prefix (`ErrNotFound`, `ErrUnauthorized`)

**React Frontend:**
- Components: `PascalCase` files and names (`StudentList.tsx`, `SurahPicker.tsx`)
- Hooks: `camelCase` with `use` prefix (`useAuth.ts`, `useOfflineQueue.ts`)
- Utilities: `camelCase` (`formatDate.ts`, `apiClient.ts`)
- Routes: `kebab-case` paths (`/halaqat/:id/students`)
- CSS classes: Tailwind utilities only — no custom class names

**API:**
- Endpoints: `/api/v1/{resource}` plural, `kebab-case` for multi-word (`/attendance-trends`)
- JSON fields: `snake_case` in API responses (`organization_id`, `created_at`)
- Query params: `snake_case` (`?halaqah_id=&date_from=`)

### Structure Patterns

**Go Backend:**
```
api/internal/
  handler/        # One file per resource (organization.go, halaqah.go)
  service/        # One file per domain (recitation.go, attendance.go)
  middleware/     # Auth, tenant, audit, logging
  model/          # sqlc-generated types (DO NOT edit manually)
  db/             # Database connection, transaction helpers
```

**React Frontend:**
```
web/src/
  features/{name}/
    components/   # Feature-specific components
    hooks/        # Feature-specific hooks
    index.ts      # Public API barrel export
  components/     # Shared UI components (Button, Input, Modal)
  hooks/          # Shared hooks (useAuth, useOfflineQueue)
  lib/            # Utilities, API client, i18n config
  routes/         # TanStack Router file-based routes
```

**Tests:** Co-located. Go: `_test.go` next to source. React: `.test.tsx` next to component.

### Format Patterns

**API Success:** `{"data": { ... }}`
**API List:** `{"data": [...], "total": 42}`
**API Error:** `{"error": {"code": "VALIDATION_ERROR", "message": "...", "details": [...]}}`
**Error Codes:** `VALIDATION_ERROR`, `NOT_FOUND`, `UNAUTHORIZED`, `FORBIDDEN`, `CONFLICT`, `INTERNAL_ERROR`
**Dates:** ISO 8601 UTC in API (`"2026-04-13T06:30:00Z"`). Frontend converts to local.
**IDs:** UUIDs everywhere. Server-generated online, client-generated (v4) offline.

### Process Patterns

**Tenant Isolation:** Every authenticated handler extracts `orgID` from JWT context. Every query includes `WHERE organization_id = $orgID`.

**Error Handling (Go):** Return structured errors via helper: `api.Error(w, statusCode, code, message)`

**Error Handling (React):** TanStack Query error boundaries per feature. Global toast for API errors. Offline indicator on connectivity loss.

**Loading States:** Use TanStack Query's `isLoading`/`isError`/`data` directly. Skeleton loaders for lists, spinners for actions.

**Offline Mutations:** TanStack Query optimistic updates + IndexedDB queue. Auto-sync on reconnect via Service Worker.

### Enforcement Guidelines

**All AI Agents MUST:**
1. Never query tenant-scoped tables without `WHERE organization_id = ?`
2. Never hardcode UI strings — always use `t('key')` from i18next
3. Never skip error handling — every API call has explicit error response
4. Never use `left`/`right` in Tailwind — use `start`/`end` for RTL safety
5. Never store sensitive data in localStorage — httpOnly cookies for refresh tokens

**Anti-Patterns:**
```go
// WRONG: db.Query("SELECT * FROM students")
// RIGHT: db.Query("SELECT * FROM students WHERE organization_id = $1", orgID)
```
```tsx
// WRONG: <div className="ml-4 text-left">
// RIGHT: <div className="ms-4 text-start">
```
```tsx
// WRONG: <button>Save</button>
// RIGHT: <button>{t('common.save')}</button>
```

## Project Structure & Boundaries

### Complete Project Directory Structure

```
mutqin/
├── README.md
├── Makefile
├── docker-compose.yml
├── Caddyfile
├── .gitignore
├── .env.example
│
├── api/
│   ├── go.mod
│   ├── go.sum
│   ├── cmd/server/main.go
│   ├── internal/
│   │   ├── config/config.go
│   │   ├── db/
│   │   │   ├── db.go
│   │   │   ├── queries.sql.go        # sqlc generated
│   │   │   ├── models.go             # sqlc generated
│   │   │   └── tx.go
│   │   ├── auth/
│   │   │   ├── otp.go
│   │   │   ├── jwt.go
│   │   │   └── invite.go
│   │   ├── middleware/
│   │   │   ├── auth.go
│   │   │   ├── tenant.go
│   │   │   ├── role.go
│   │   │   ├── audit.go
│   │   │   └── cors.go
│   │   ├── handler/
│   │   │   ├── auth.go
│   │   │   ├── organization.go
│   │   │   ├── landing.go
│   │   │   ├── announcement.go
│   │   │   ├── registration.go
│   │   │   ├── halaqah.go
│   │   │   ├── teacher.go
│   │   │   ├── student.go
│   │   │   ├── recitation.go
│   │   │   ├── attendance.go
│   │   │   ├── dashboard.go
│   │   │   ├── sync.go
│   │   │   ├── platform.go
│   │   │   └── health.go
│   │   ├── service/
│   │   │   ├── organization.go
│   │   │   ├── recitation.go
│   │   │   ├── attendance.go
│   │   │   ├── sync.go
│   │   │   ├── dashboard.go
│   │   │   └── upload.go
│   │   └── response/response.go
│   └── templates/
│       ├── landing.html           # Center landing page (server-rendered)
│       └── register.html          # Registration form (server-rendered)
│   └── sql/
│       ├── migrations/
│       │   ├── 001_create_organizations.up.sql
│       │   ├── 001_create_organizations.down.sql
│       │   ├── 002_create_users_and_auth.up.sql
│       │   ├── 003_create_halaqat_students.up.sql
│       │   ├── 004_create_recitations.up.sql
│       │   ├── 005_create_attendance.up.sql
│       │   ├── 006_create_announcements_registrations.up.sql
│       │   ├── 007_create_sync_conflicts.up.sql
│       │   └── 008_create_audit_log.up.sql
│       ├── queries/
│       │   ├── organizations.sql
│       │   ├── users.sql
│       │   ├── halaqat.sql
│       │   ├── students.sql
│       │   ├── recitations.sql
│       │   ├── attendance.sql
│       │   ├── registrations.sql
│       │   ├── announcements.sql
│       │   └── dashboard.sql
│       └── sqlc.yaml
│
├── web/
│   ├── package.json
│   ├── tsconfig.json
│   ├── vite.config.ts
│   ├── tailwind.config.ts
│   ├── index.html
│   ├── public/
│   │   ├── manifest.json
│   │   ├── icons/
│   │   ├── fonts/                 # KFGQPC Uthmani Hafs for Quranic text
│   │   └── quran/
│   │       ├── surahs.json
│   │       └── ayat/                  # 114 JSON files
│   └── src/
│       ├── main.tsx
│       ├── App.tsx
│       ├── router.tsx
│       ├── i18n.ts
│       ├── sw/
│       │   ├── sw.ts
│       │   └── sync.ts
│       ├── lib/
│       │   ├── api.ts
│       │   ├── db.ts
│       │   ├── offline-queue.ts
│       │   └── utils.ts
│       ├── hooks/
│       │   ├── useAuth.ts
│       │   ├── useOfflineStatus.ts
│       │   └── useDirection.ts
│       ├── components/
│       │   ├── ui/                    # Button, Input, Modal, Toast, Skeleton, OfflineBanner
│       │   └── layout/               # AppLayout, PublicLayout, Navigation
│       ├── features/
│       │   ├── auth/
│       │   ├── landing/
│       │   ├── halaqat/
│       │   ├── recitation/
│       │   ├── attendance/
│       │   ├── dashboard/
│       │   └── platform/
│       ├── locales/
│       │   ├── ar.json
│       │   └── so.json
│       └── routes/                    # TanStack Router file-based routes
```

### Architectural Boundaries

**API Boundaries:**
- Public endpoints (`/api/v1/public/*`) — no auth, rate-limited
- Authenticated endpoints — require valid JWT
- Super Admin endpoints — require `super_admin` role
- Center Admin endpoints — require `center_admin` + matching `organization_id`
- Teacher endpoints — require `teacher` + matching halaqah assignment

**Data Boundaries:**
- sqlc queries are the ONLY database access — no raw SQL in handlers
- Service layer owns business logic; handlers parse requests and return responses
- IndexedDB mirrors server schema for offline tables only

**Frontend Boundaries:**
- Features never import from other features — use shared `components/` and `hooks/`
- `lib/api.ts` is the single API gateway
- `lib/db.ts` is the single IndexedDB gateway

### Requirements to Structure Mapping

| Epic | Go Handlers | Frontend Feature | Migrations |
|------|------------|-----------------|------------|
| 1: Auth | `auth.go` | `features/auth/` | `002_create_users_and_auth` |
| 2: Platform Admin | `organization.go`, `platform.go` | `features/platform/` | `001_create_organizations` |
| 3: Landing Page | `landing.go`, `announcement.go`, `registration.go` | `features/landing/` | `006_create_announcements_registrations` |
| 4: Halaqah | `halaqah.go`, `teacher.go`, `student.go` | `features/halaqat/` | `003_create_halaqat_students` |
| 5: Recitation | `recitation.go` | `features/recitation/` | `004_create_recitations` |
| 6: Attendance | `attendance.go` | `features/attendance/` | `005_create_attendance` |
| 7: Offline Sync | `sync.go` | `sw/`, `lib/offline-queue.ts` | `007_create_sync_conflicts` |
| 8: Dashboard | `dashboard.go` | `features/dashboard/` | — |
| 9: Localization | — | `locales/`, `hooks/useDirection.ts` | — |

### External Integration Points

| Service | Purpose | Integration Point |
|---------|---------|-------------------|
| Cloudflare R2 | File storage | `service/upload.go` |
| SMTP (Resend) | Email OTP | `auth/otp.go` |
| quran.com API | Quran data (build-time) | `web/public/quran/` |
| UptimeRobot | Health monitoring | `GET /api/v1/health` |

### Data Flow

```
[Browser] → [Caddy] → static files (/var/www/mutqin/)
                    → /api/* → [Go API :8080] → [PostgreSQL]
                                              → [R2]
                                              → [SMTP]

Offline: [React] → [TanStack Query] → [IndexedDB Queue]
         [Service Worker] → connectivity → [POST /sync/push] → [Go API]
```

## UX-Architecture Integration

_Added 2026-04-14 after UX Design Specification completed. See `docs/ux-design-specification.md` for full UX context._

### Frontend Component Architecture (UX-Informed)

The UX spec defines a specific component library strategy that refines the architecture's frontend structure:

**Design System:** Tailwind CSS 4 + Headless UI + Custom Components (no shadcn/ui — bundle weight concern)

**Headless UI components used:**

| Headless UI | Used for |
|-------------|----------|
| `Listbox` | Surah picker, ayah range, hifz level selector |
| `Dialog` | OTP entry, destructive confirmations |
| `Tab Group` | Dashboard sections, attendance filters |
| `Switch` | Attendance present/absent toggle, language toggle |
| `Menu` | Settings overflow, card action menus |
| `Combobox` | Student search, surah search by name |

**Custom components (build from scratch with Tailwind):**

| Component | File | Notes |
|-----------|------|-------|
| `StudentListRow` | `components/ui/StudentListRow.tsx` | Compact row: avatar + name + last surah + grade badge. 64px height. |
| `RecitationForm` | `features/recitation/components/RecitationForm.tsx` | Core experience. Auto-save on every field change to IndexedDB. Swipe-enabled. |
| `SurahPicker` | `features/recitation/components/SurahPicker.tsx` | Wraps Headless UI Combobox. Data from `public/quran/surahs.json`. |
| `GradeSelector` | `features/recitation/components/GradeSelector.tsx` | 5 buttons: mumtaz→daif. Color-coded per UX spec. |
| `BottomTabBar` | `components/layout/BottomTabBar.tsx` | 4 tabs, role-aware. 56px height. Visible until `lg:` breakpoint. |
| `TopBar` | `components/layout/TopBar.tsx` | Title + offline dot + language toggle. 48px height. |
| `OfflineBanner` | `components/ui/OfflineBanner.tsx` | Amber/green/red dot + text. `aria-live="polite"`. Non-blocking. |
| `SessionProgressBar` | `features/recitation/components/SessionProgressBar.tsx` | "7/18" with fill bar. |
| `AttendanceList` | `features/attendance/components/AttendanceList.tsx` | Bulk toggle. All default present. Reuses Switch. |
| `EmptyState` | `components/ui/EmptyState.tsx` | Icon + message + action button. Per-screen variants. |

### Typography Architecture

| Context | Font | Loading |
|---------|------|---------|
| Arabic UI | Cairo (400, 600, 700) | Google Fonts via `@font-face`, `font-display: swap` |
| Somali/Latin UI | Inter (400, 500, 600, 700) | Google Fonts via `@font-face`, `font-display: swap` |
| Quranic text | KFGQPC Uthmani Hafs (400) | Bundled in `public/fonts/`, cached by Service Worker |

Minimum text: 14px. Quranic text: 20px with line-height 1.8 for diacritical marks.

### Color Token Architecture

Define as CSS custom properties in Tailwind config, consumed throughout:

```
--color-primary-50:  #ECFDF5    --color-primary-600: #059669
--color-primary-100: #D1FAE5    --color-primary-700: #047857
                                --color-primary-800: #065F46

--color-gray-50:  #F9FAFB  (page bg)
--color-gray-200: #E5E7EB  (borders)
--color-gray-500: #6B7280  (secondary text)
--color-gray-700: #374151  (body text)
--color-gray-900: #111827  (headings)

--color-success: #059669  (saved/synced)
--color-warning: #D97706  (offline/pending)
--color-error:   #DC2626  (failed/validation)
```

### Landing Page Architecture (UX-Informed)

The UX spec confirms the landing page should be **server-rendered HTML from Go templates** — not part of the React PWA bundle. This is a critical architectural decision:

```
api/internal/handler/landing.go  →  renders Go HTML template
api/templates/landing.html       →  center landing page template
api/templates/register.html      →  registration form template
```

**Why server-rendered:**
- Parents access via WhatsApp in-app browser — no React, no JS needed
- Loads under 2 seconds on 3G (zero JS overhead)
- SEO-friendly for center discovery
- Separate from PWA bundle — doesn't affect 500KB budget

**Go template serves:** center name, logo, description, schedule, announcements, registration form at `mutqin.app/{slug}`

### Route Architecture (UX-Informed)

The UX spec defines the complete screen inventory. TanStack Router file-based routes:

```
web/src/routes/
├── __root.tsx              # Root layout (auth check, direction, offline banner)
├── login.tsx               # OTP login
├── index.tsx               # Teacher home (student list + start session)
├── students.tsx            # Student list (teacher view)
├── session/
│   ├── index.tsx           # Session flow entry
│   ├── $studentId.tsx      # Record individual student
│   └── complete.tsx        # Session summary
├── attendance.tsx          # Mark attendance
├── admin/
│   ├── index.tsx           # Center admin dashboard
│   ├── halaqat.tsx         # Halaqat management
│   ├── registrations.tsx   # Registration management
│   └── landing.tsx         # Landing page editor
├── platform/
│   ├── index.tsx           # Super admin dashboard
│   └── organizations.tsx   # Organization management
└── settings.tsx            # Language, profile
```

### Responsive Architecture (UX-Informed)

| Breakpoint | Tailwind | Navigation | Layout |
|-----------|----------|------------|--------|
| < 640px | default | Bottom tab bar | Single column |
| ≥ 640px | `sm:` | Bottom tab bar | 2-column admin grids |
| ≥ 1024px | `lg:` | Side navigation | Max-width 1024px container |

### Accessibility Architecture (UX-Informed)

**Target: WCAG 2.1 Level AA**

ARIA requirements per component:

| Component | ARIA |
|-----------|------|
| BottomTabBar | `role="tablist"`, `role="tab"`, `aria-selected` |
| StudentListRow | `role="listitem"` within `role="list"` |
| GradeSelector | `role="radiogroup"` + `role="radio"` + `aria-checked` |
| OfflineBanner | `role="status"`, `aria-live="polite"` |
| Error messages | `role="alert"`, `aria-live="assertive"` |
| All Headless UI | Handles ARIA automatically |

Focus management: 2px `primary-600` outline on `:focus-visible`. All interactive elements keyboard-operable.

### Updated Enforcement Guidelines (UX Additions)

In addition to the existing enforcement guidelines, **AI agents MUST also:**

6. Never use box shadows on cards — use 1px `gray-200` border only (performance on low-end devices)
7. Never use CSS animations between screens — exception: swipe in session flow only
8. Never show a blank/empty screen — always use EmptyState component with action
9. Never use a hamburger menu — bottom tab bar on mobile, side nav on desktop
10. Never show success toasts for auto-saved actions — too noisy during session flow
11. Never use `px` for font sizes — always `rem` (exception: 1px borders)
12. Never make a modal/dialog for errors — inline validation for fixable errors, toast for unfixable
13. Never place primary action buttons at top of screen — always bottom, fixed position, in thumb zone
14. Always provide tap alternative for swipe gestures (fallback button)
15. Always use Eastern Arabic numerals (٠١٢٣) when UI language is Arabic

**Additional Anti-Patterns:**
```tsx
// WRONG: <div className="shadow-md rounded-lg">
// RIGHT: <div className="border border-gray-200 rounded-lg">

// WRONG: <div className="text-[13px]">
// RIGHT: <div className="text-sm">  (14px minimum)

// WRONG: {items.length === 0 && null}
// RIGHT: {items.length === 0 && <EmptyState message={t('...')} action={...} />}

// WRONG: <button className="fixed top-4">Primary Action</button>
// RIGHT: <button className="fixed bottom-20">Primary Action</button>
```

## Architecture Validation Results

### Coherence Validation

All technology choices are compatible and well-tested together. Go + Chi + sqlc + PostgreSQL on backend. React 19 + Vite 6 + TanStack + Tailwind 4 on frontend. Caddy as reverse proxy. No version conflicts or incompatibilities detected.

**Minor note:** API returns snake_case JSON, React uses camelCase. A transform utility is needed in `lib/api.ts` — add in Story 1.1.

### Requirements Coverage

All 9 epics and 47 FRs have explicit architectural support. All 20 NFRs are addressed by technology choices, middleware, or infrastructure decisions. No coverage gaps.

### Implementation Readiness

- All critical decisions documented with specific technologies and versions
- Complete SQL schema for all 11 tables
- Full API contract with 30+ endpoints
- Project structure maps every epic to specific files
- Implementation patterns with concrete examples and anti-patterns
- Enforcement guidelines for AI agent consistency

### Minor Gaps (Not Blocking)

1. Add snake_case ↔ camelCase transform in `lib/api.ts` (Story 1.1)
2. Add rate limiting middleware on `/api/v1/public/*` (Epic 3)
3. Pick email provider: Resend recommended (free tier: 100 emails/day)
4. Specify refresh token cookie config: `SameSite=Strict`, `Secure=true`, `Path=/api/v1/auth/refresh`

### Architecture Completeness Checklist

- [x] Project context analyzed
- [x] Technology stack specified with versions
- [x] Data model with complete SQL schema
- [x] API contracts fully defined
- [x] Auth flow specified end-to-end
- [x] Offline sync architecture designed
- [x] Multi-tenancy enforcement at middleware level
- [x] Implementation patterns with examples
- [x] Project structure with epic mapping
- [x] Validation passed — no critical gaps
- [x] UX component architecture integrated (2026-04-14)
- [x] Typography and color token system defined (2026-04-14)
- [x] Route architecture mapped to UX screen inventory (2026-04-14)
- [x] Landing page server-rendering decision confirmed (2026-04-14)
- [x] Responsive breakpoint strategy defined (2026-04-14)
- [x] WCAG AA accessibility requirements specified (2026-04-14)
- [x] UX enforcement guidelines for AI agents added (2026-04-14)

### Readiness Assessment

**Status: READY FOR IMPLEMENTATION**
**Confidence: High**

### Implementation Handoff

AI agents must follow this document exactly. Key rules:
- Create DB tables ONLY in the migration file for the relevant epic
- Every query MUST include `WHERE organization_id = ?`
- Every UI string MUST use `t('key')` from i18next
- Every Tailwind class MUST use logical properties (`ms-`, `me-`, `start`, `end`)
