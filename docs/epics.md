---
stepsCompleted: ['step-01-validate-prerequisites', 'step-02-design-epics', 'step-03-create-stories']
inputDocuments: ['/Volumes/SSD/mutqin/docs/prd.md']
---

# Mutqin (مُتقِن) - Epic Breakdown

## Overview

This document provides the complete epic and story breakdown for Mutqin, decomposing the requirements from the PRD into implementable stories.

## Requirements Inventory

### Functional Requirements

- FR1: Super Admin can create a new center organization with name, location, and admin contact details
- FR2: Super Admin can generate a unique invite link for a center admin to activate their account
- FR3: Super Admin can view a platform-wide dashboard showing all centers, their status, and aggregate activity metrics
- FR4: Super Admin can deactivate or suspend a center organization
- FR5: Super Admin can view which centers are on which pricing tier (Free/Asaasi/Pro)
- FR6: Center Admin can create and publish a public landing page for their center
- FR7: Center Admin can add center information to the landing page (name, logo, description, location, schedule)
- FR8: Center Admin can post announcements visible on the landing page
- FR9: Center Admin can create registration forms with configurable fields
- FR10: Center Admin can view and manage submitted registration applications
- FR11: Center Admin can approve or reject registration applications
- FR12: Parents can view a center's public landing page without authentication
- FR13: Parents can submit a registration form for their child without creating an account
- FR14: Center Admin can create a halaqah with name, schedule, and maximum capacity
- FR15: Center Admin can assign a teacher to a halaqah
- FR16: Center Admin can enroll students into a halaqah
- FR17: Center Admin can transfer a student between halaqat
- FR18: Center Admin can view a list of all halaqat with teacher assignments and student counts
- FR19: Center Admin can deactivate a halaqah
- FR20: Center Admin can generate invite links for teachers
- FR21: Teachers can activate their account via invite link and phone OTP
- FR22: All users can authenticate via phone number and OTP (no email required)
- FR23: System enforces role-based access: Super Admin sees platform, Center Admin sees their center, Teacher sees their halaqah only
- FR24: Center Admin can deactivate a teacher account
- FR25: Teacher can view their halaqah's student list (available offline)
- FR26: Teacher can record a recitation session for a student specifying: type (new hifz / near review / far review), surah, ayah range, grade, and optional notes
- FR27: Teacher can see a student's last recorded position to determine where to continue
- FR28: System suggests the next surah/ayah based on the student's last session
- FR29: Teacher can record recitation for all students in a halaqah in a single session flow
- FR30: Teacher can record recitation while offline; data syncs when connectivity returns
- FR31: Teacher can mark attendance for all students in their halaqah (present/absent)
- FR32: Teacher can mark attendance in bulk with defaults (all present, then mark exceptions)
- FR33: Teacher can mark attendance while offline; data syncs when connectivity returns
- FR34: Center Admin can view attendance records by halaqah, student, or date range
- FR35: Center Admin can view a dashboard showing: total students, total halaqat, active teachers, overall attendance rate
- FR36: Center Admin can view attendance trends over time
- FR37: Center Admin can view recitation activity per halaqah (sessions recorded today/this week)
- FR38: Center Admin can view a student's recitation history and current hifz position
- FR39: System provides a complete reference of 114 surahs and 6,236 ayat in Uthmani script
- FR40: Quran data is stored locally on device for offline access
- FR41: Quran text is read-only — no user can modify Quranic content
- FR42: Users can switch the UI language between Arabic and Somali
- FR43: System supports RTL layout for Arabic and LTR layout for Somali
- FR44: All UI elements use large tap targets optimized for mobile use
- FR45: System caches student list, halaqah data, and Quran reference data locally for offline use
- FR46: Recitation and attendance recorded offline are queued and synced automatically when connectivity returns
- FR47: System handles sync conflicts using last-write-wins with conflict log visible to admin

### NonFunctional Requirements

- NFR1: Page load time under 3 seconds on a 3G connection (1 Mbps)
- NFR2: Recitation recording form responds to tap input within 200ms
- NFR3: Student list for a halaqah (up to 30 students) renders within 1 second, including offline
- NFR4: Total PWA bundle size under 500KB (compressed) — excluding cached Quran data
- NFR5: Landing page loads under 2 seconds on 3G for unauthenticated visitors
- NFR6: Surah/ayah picker renders full surah list within 500ms from local cache
- NFR7: All data in transit encrypted via TLS 1.2+
- NFR8: OTP codes expire after 5 minutes and are single-use
- NFR9: Multi-tenant data isolation enforced at database query level — no API endpoint can return data from another organization
- NFR10: Invite links are single-use and expire after 72 hours
- NFR11: Super Admin actions (create/suspend organization) are logged with timestamp and actor
- NFR12: No student PII exposed in public landing pages or URLs
- NFR13: System supports up to 50 concurrent centers on a single $10/month VPS
- NFR14: Database schema supports future horizontal scaling without migration
- NFR15: API response time remains under 500ms with 500 concurrent users
- NFR16: Offline mode supports full recitation and attendance recording for up to 7 days without sync
- NFR17: No data loss when device loses power during recording — auto-save on every field change
- NFR18: Sync queue processes automatically when connectivity returns without user intervention
- NFR19: System uptime target: 99%
- NFR20: Failed sync attempts retry with exponential backoff up to 3 times before alerting the user

### Additional Requirements

- Self-hosted VPS infrastructure (Hetzner/Contabo, $5-10/month)
- PostgreSQL database with row-level security via organization_id
- PWA with Service Worker + IndexedDB for offline support
- REST API backend (API-first design)
- Phone OTP authentication — no email dependency
- Invite-based onboarding: Super Admin → Center Admin → Teacher
- i18n framework for Arabic (RTL) + Somali (LTR) from day one
- Static Quran data cached locally (114 surahs, 6,236 ayat from quran.com API)
- Single-database multi-tenancy with organization_id isolation
- Landing page URLs: mutqin.app/{center-slug}
- TLS 1.2+ for all data in transit
- 3-tier RBAC: Super Admin, Center Admin, Teacher

### UX Design Requirements

No UX Design document available. UI requirements are captured in FRs (FR42-44) and NFRs (NFR1-6).

### FR Coverage Map

- FR1: Epic 2 - Create center organization
- FR2: Epic 2 - Generate center admin invite link
- FR3: Epic 2 - Platform-wide dashboard
- FR4: Epic 2 - Deactivate/suspend center
- FR5: Epic 2 - View center pricing tiers
- FR6: Epic 3 - Create/publish landing page
- FR7: Epic 3 - Add center info to landing page
- FR8: Epic 3 - Post announcements
- FR9: Epic 3 - Create registration forms
- FR10: Epic 3 - View/manage registrations
- FR11: Epic 3 - Approve/reject registrations
- FR12: Epic 3 - Public landing page (no auth)
- FR13: Epic 3 - Parent submits registration form
- FR14: Epic 4 - Create halaqah
- FR15: Epic 4 - Assign teacher to halaqah
- FR16: Epic 4 - Enroll students
- FR17: Epic 4 - Transfer student between halaqat
- FR18: Epic 4 - View halaqat list
- FR19: Epic 4 - Deactivate halaqah
- FR20: Epic 4 - Generate teacher invite links
- FR21: Epic 4 - Teacher activates via invite + OTP
- FR22: Epic 1 - Phone OTP authentication
- FR23: Epic 1 - Role-based access enforcement
- FR24: Epic 4 - Deactivate teacher account
- FR25: Epic 5 - View halaqah student list (offline)
- FR26: Epic 5 - Record recitation session
- FR27: Epic 5 - View student's last position
- FR28: Epic 5 - System suggests next surah/ayah
- FR29: Epic 5 - Session flow for all students
- FR30: Epic 5 - Record recitation offline
- FR31: Epic 6 - Mark attendance
- FR32: Epic 6 - Bulk attendance with defaults
- FR33: Epic 6 - Mark attendance offline
- FR34: Epic 6 - View attendance records
- FR35: Epic 8 - Center dashboard stats
- FR36: Epic 8 - Attendance trends
- FR37: Epic 8 - Recitation activity per halaqah
- FR38: Epic 8 - Student recitation history
- FR39: Epic 5 - Quran reference data (114 surahs)
- FR40: Epic 5 - Quran data stored locally
- FR41: Epic 5 - Quran text read-only
- FR42: Epic 9 - Language switching (Arabic/Somali)
- FR43: Epic 9 - RTL/LTR layout support
- FR44: Epic 9 - Large tap targets for mobile
- FR45: Epic 7 - Local data caching
- FR46: Epic 7 - Offline queue and auto-sync
- FR47: Epic 7 - Sync conflict handling

## Epic List

### Epic 1: Project Foundation & Authentication
Users can create accounts via phone OTP, authenticate, and access role-appropriate views. Super Admin can manage the platform. Technical foundation for all subsequent epics.
**FRs covered:** FR22, FR23
**Also addresses:** NFR7-12 (security), NFR13-14 (scalability), Additional Requirements (PostgreSQL, tenancy, REST API, i18n framework, PWA shell)

### Epic 2: Platform Administration & Center Onboarding
Super Admin can create center organizations, generate invite links, and onboard center admins. Center admins can activate accounts and access their center dashboard.
**FRs covered:** FR1, FR2, FR3, FR4, FR5

### Epic 3: Center Landing Page & Public Presence
Center Admin can create a public landing page with center info, announcements, and registration forms. Parents can discover the center and register their children without an account.
**FRs covered:** FR6, FR7, FR8, FR9, FR10, FR11, FR12, FR13
**Also addresses:** NFR5 (landing page load time)

### Epic 4: Halaqah & Teacher Management
Center Admin can create halaqat, invite and assign teachers, and enroll students. Teachers can activate their accounts and see their assigned halaqah.
**FRs covered:** FR14, FR15, FR16, FR17, FR18, FR19, FR20, FR21, FR24

### Epic 5: Recitation Recording
Teachers can record daily recitation sessions for students — new hifz, near review, far review — with surah/ayah selection, grading, and notes. Works fully offline.
**FRs covered:** FR25, FR26, FR27, FR28, FR29, FR30, FR39, FR40, FR41
**Also addresses:** NFR2, NFR3, NFR6 (performance), NFR16, NFR17 (reliability)

### Epic 6: Attendance Tracking
Teachers can mark daily attendance for their halaqah with bulk defaults. Works fully offline.
**FRs covered:** FR31, FR32, FR33, FR34

### Epic 7: Offline Sync & Data Caching
System caches data locally, queues offline changes, and syncs automatically when connectivity returns. Handles conflicts gracefully.
**FRs covered:** FR45, FR46, FR47
**Also addresses:** NFR16, NFR17, NFR18, NFR20 (reliability)

### Epic 8: Dashboard & Reporting
Center Admin can view a dashboard with center stats, attendance trends, recitation activity, and individual student progress.
**FRs covered:** FR35, FR36, FR37, FR38

### Epic 9: Localization & Mobile Optimization
Users can switch between Arabic (RTL) and Somali (LTR). All UI elements are optimized for mobile with large tap targets.
**FRs covered:** FR42, FR43, FR44
**Also addresses:** NFR1, NFR4 (performance budget)

## Epic 1: Project Foundation & Authentication

Set up the technical foundation: PostgreSQL database with multi-tenancy, REST API, PWA shell, and phone OTP authentication with role-based access control.

### Story 1.1: Project Scaffolding & Database Setup

As a developer,
I want a working project with PostgreSQL database, REST API server, and PWA shell,
So that all subsequent features have a foundation to build on.

**Acceptance Criteria:**

**Given** a fresh VPS with PostgreSQL installed
**When** the project is deployed
**Then** the API server starts and responds to health check requests
**And** the PWA shell loads in a browser with a blank authenticated layout
**And** the database has organizations, users, and roles tables with organization_id for tenancy
**And** TLS is configured for all API endpoints (NFR7)
**And** row-level security policies are applied to all tables (NFR9)

### Story 1.2: Phone OTP Authentication

As a user,
I want to log in using my phone number and a one-time code,
So that I can access the system without needing an email account.

**Acceptance Criteria:**

**Given** a user with a registered phone number
**When** they enter their phone number and request an OTP
**Then** a 6-digit code is sent via SMS to their phone
**And** the code expires after 5 minutes (NFR8)
**And** the code is single-use — entering it again fails
**And** successful OTP verification issues a session token
**And** invalid OTP shows a clear error message in Arabic/Somali

### Story 1.3: Role-Based Access Control

As a system,
I want to enforce role-based access so each user sees only their authorized data,
So that Super Admins see the platform, Center Admins see their center, and Teachers see their halaqah.

**Acceptance Criteria:**

**Given** three roles exist: super_admin, center_admin, teacher
**When** a user authenticates
**Then** API responses are filtered by their role and organization_id
**And** Super Admin can access all organizations' aggregate data
**And** Center Admin can only access data where organization_id matches their center
**And** Teacher can only access data for their assigned halaqah
**And** attempting to access unauthorized data returns 403 Forbidden
**And** Super Admin actions are logged with timestamp and actor (NFR11)

## Epic 2: Platform Administration & Center Onboarding

Super Admin can create and manage center organizations, generate invite links, and monitor the platform.

### Story 2.1: Create Center Organization

As a Super Admin,
I want to create a new center organization with name, location, and admin contact details,
So that I can onboard new Quran centers onto the platform.

**Acceptance Criteria:**

**Given** I am logged in as Super Admin
**When** I submit a new organization form with center name, city, country, and admin phone number
**Then** the organization is created with a unique ID and slug
**And** the organization is assigned the Free tier by default
**And** the action is logged with timestamp (NFR11)

### Story 2.2: Generate Center Admin Invite Link

As a Super Admin,
I want to generate a unique invite link for a center admin,
So that the center admin can activate their account independently.

**Acceptance Criteria:**

**Given** an organization exists without an admin account
**When** I click "Generate Invite Link"
**Then** a unique, single-use invite URL is generated
**And** the link expires after 72 hours (NFR10)
**And** I can copy the link to share via WhatsApp
**And** clicking the link after expiry shows "Link expired" message

### Story 2.3: Center Admin Activates Account via Invite

As a Center Admin,
I want to activate my account by clicking an invite link and verifying my phone number,
So that I can start managing my center.

**Acceptance Criteria:**

**Given** I received a valid invite link from the Super Admin
**When** I click the link and enter my phone number
**Then** I receive an OTP on my phone
**And** after verifying the OTP, my account is created with center_admin role
**And** I am linked to the correct organization
**And** I am redirected to my center's empty dashboard
**And** the invite link is invalidated (single-use)

### Story 2.4: Platform-Wide Dashboard

As a Super Admin,
I want to view a dashboard showing all centers, their status, and aggregate metrics,
So that I can monitor platform health and activity.

**Acceptance Criteria:**

**Given** I am logged in as Super Admin
**When** I view the platform dashboard
**Then** I see a list of all organizations with: name, status (active/suspended), pricing tier, date created
**And** I see aggregate metrics: total centers, total students, total teachers
**And** I can see which centers have been active this week (any recitation/attendance recorded)

### Story 2.5: Manage Center Organizations

As a Super Admin,
I want to deactivate/suspend a center and view pricing tiers,
So that I can manage the platform.

**Acceptance Criteria:**

**Given** I am viewing a center's details
**When** I click "Suspend" or "Deactivate"
**Then** the center's status changes and all its users lose access
**And** the action is logged (NFR11)
**And** I can view and change the center's pricing tier (Free/Asaasi/Pro)

## Epic 3: Center Landing Page & Public Presence

Center Admin can create a public landing page. Parents can view it and register their children.

### Story 3.1: Create and Publish Landing Page

As a Center Admin,
I want to create a public landing page for my center,
So that parents can find information about my center online.

**Acceptance Criteria:**

**Given** I am logged in as Center Admin
**When** I fill in center details: name, logo (image upload), description, location, weekly schedule
**Then** a public page is published at mutqin.app/{center-slug}
**And** the page loads under 2 seconds on 3G (NFR5)
**And** the page is viewable without authentication (FR12)
**And** the page displays correctly on mobile devices
**And** no student PII is visible on the public page (NFR12)

### Story 3.2: Post Announcements

As a Center Admin,
I want to post announcements on my landing page,
So that parents can see important updates about the center.

**Acceptance Criteria:**

**Given** my center has a published landing page
**When** I create a new announcement with title and body text
**Then** the announcement appears on the public landing page
**And** announcements display in reverse chronological order (newest first)
**And** I can edit or delete existing announcements

### Story 3.3: Create Registration Form

As a Center Admin,
I want to create a registration form on my landing page,
So that parents can register their children for my center online.

**Acceptance Criteria:**

**Given** my center has a published landing page
**When** I configure a registration form with fields: child name, age, parent phone, previous hifz level
**Then** a "Register Your Child" button appears on the landing page
**And** the form is accessible without creating an account (FR13)
**And** the form validates required fields before submission
**And** the form works on mobile with large tap targets

### Story 3.4: Parent Submits Registration

As a parent,
I want to submit a registration form for my child,
So that I can enroll them in the Quran center.

**Acceptance Criteria:**

**Given** I am viewing a center's landing page
**When** I tap "Register Your Child" and fill in the form
**Then** my submission is saved and linked to the center
**And** I see a confirmation message: "Registration submitted successfully"
**And** no account creation is required from me
**And** my phone number is stored for future communication

### Story 3.5: Manage Registration Applications

As a Center Admin,
I want to view, approve, or reject student registration applications,
So that I can control who enrolls in my center.

**Acceptance Criteria:**

**Given** parents have submitted registration forms
**When** I view the registrations list
**Then** I see all submissions with: child name, age, parent phone, hifz level, date submitted
**And** I can approve a registration (student becomes enrollable)
**And** I can reject a registration with an optional reason
**And** approved students appear in my student pool for halaqah assignment

## Epic 4: Halaqah & Teacher Management

Center Admin can create halaqat, invite teachers, and manage student enrollment.

### Story 4.1: Create and Manage Halaqat

As a Center Admin,
I want to create halaqat with name, schedule, and capacity,
So that I can organize my center's teaching structure.

**Acceptance Criteria:**

**Given** I am logged in as Center Admin
**When** I create a new halaqah with name, schedule (days + time), and max capacity
**Then** the halaqah appears in my halaqat list
**And** I can view all halaqat with their teacher assignments and student counts (FR18)
**And** I can deactivate a halaqah (FR19)
**And** deactivated halaqat are hidden from active lists but data is preserved

### Story 4.2: Invite and Onboard Teachers

As a Center Admin,
I want to invite teachers to join my center via a link,
So that teachers can access the system and manage their halaqat.

**Acceptance Criteria:**

**Given** I am logged in as Center Admin
**When** I generate a teacher invite link (FR20)
**Then** a unique invite URL is created for my center
**And** a teacher clicking the link can verify their phone via OTP (FR21)
**And** the teacher account is created with teacher role linked to my organization
**And** I can deactivate a teacher account (FR24)

### Story 4.3: Assign Teachers to Halaqat

As a Center Admin,
I want to assign a teacher to a halaqah,
So that the teacher can manage that halaqah's students.

**Acceptance Criteria:**

**Given** I have active teachers and halaqat
**When** I assign a teacher to a halaqah (FR15)
**Then** the teacher can see that halaqah and its students when they log in
**And** the halaqat list shows the assigned teacher name
**And** a halaqah can have one assigned teacher

### Story 4.4: Enroll and Transfer Students

As a Center Admin,
I want to enroll approved students into halaqat and transfer them between halaqat,
So that students are organized by level and capacity.

**Acceptance Criteria:**

**Given** I have approved student registrations and active halaqat
**When** I enroll a student into a halaqah (FR16)
**Then** the student appears in that halaqah's student list
**And** the halaqah student count updates
**And** I cannot enroll beyond the halaqah's max capacity
**And** I can transfer a student between halaqat (FR17)
**And** transfer preserves the student's recitation history

## Epic 5: Recitation Recording

Teachers can record daily recitation sessions with surah/ayah selection, grading, and notes.

### Story 5.1: Quran Reference Data Setup

As a system,
I want to provide a complete local reference of 114 surahs and 6,236 ayat in Uthmani script,
So that teachers can select accurate Quran references during recitation recording.

**Acceptance Criteria:**

**Given** the app is loaded
**When** the Quran data is initialized
**Then** all 114 surahs with names (Arabic) and ayah counts are available
**And** all 6,236 ayat are stored locally for offline access (FR40)
**And** Quran text uses verified Uthmani script (FR39)
**And** Quran data is read-only — no user can modify it (FR41)
**And** surah list renders within 500ms from local cache (NFR6)

### Story 5.2: Record Individual Student Recitation

As a Teacher,
I want to record a recitation session for a student with type, surah/ayah range, grade, and notes,
So that I have a structured record of each student's daily progress.

**Acceptance Criteria:**

**Given** I am viewing my halaqah's student list (FR25)
**When** I tap a student and start a recitation record
**Then** I can select recitation type: New Hifz / Near Review / Far Review (FR26)
**And** I can pick surah and ayah range from the Quran reference
**And** I can assign a grade: Mumtaz, Jayyid Jiddan, Jayyid, Maqbul, Da'if
**And** I can add optional text notes
**And** the form responds to tap input within 200ms (NFR2)
**And** I can see the student's last recorded position (FR27)
**And** the system suggests next surah/ayah based on last session (FR28)

### Story 5.3: Session Flow for Full Halaqah

As a Teacher,
I want to record recitation for all students in a single session flow,
So that I can efficiently process my entire halaqah without navigating back and forth.

**Acceptance Criteria:**

**Given** I have started a recitation session
**When** I finish recording one student
**Then** I can swipe or tap to the next student automatically (FR29)
**And** the student list shows who has been recorded and who is remaining
**And** I can skip a student and come back later
**And** the entire halaqah (up to 30 students) can be processed in one flow
**And** student list renders within 1 second (NFR3)

### Story 5.4: Offline Recitation Recording

As a Teacher,
I want to record recitation even without internet,
So that I can work from the masjid where connectivity is unreliable.

**Acceptance Criteria:**

**Given** my device has no internet connection
**When** I record recitation for students
**Then** all records are saved locally on the device (FR30)
**And** data auto-saves on every field change — no loss on power outage (NFR17)
**And** offline recording works for up to 7 days without sync (NFR16)
**And** a visual indicator shows I am in offline mode

## Epic 6: Attendance Tracking

Teachers can mark daily attendance with bulk defaults, online or offline.

### Story 6.1: Mark Daily Attendance

As a Teacher,
I want to mark attendance for all students in my halaqah,
So that the center has a record of who attended each session.

**Acceptance Criteria:**

**Given** I am viewing my halaqah
**When** I tap "Mark Attendance"
**Then** I see all students with present/absent toggle (FR31)
**And** all students default to "present" — I only mark exceptions (FR32)
**And** I can save attendance for the day
**And** attendance works fully offline (FR33)
**And** data auto-saves on every change (NFR17)

### Story 6.2: View Attendance Records

As a Center Admin,
I want to view attendance records by halaqah, student, or date range,
So that I can monitor attendance patterns across my center.

**Acceptance Criteria:**

**Given** attendance has been recorded
**When** I navigate to attendance reports (FR34)
**Then** I can filter by: specific halaqah, specific student, or date range
**And** I see attendance percentage per halaqah
**And** I can identify students with low attendance

## Epic 7: Offline Sync & Data Caching

System caches data locally and syncs automatically when connectivity returns.

### Story 7.1: Local Data Caching

As a user,
I want the app to cache my data locally,
So that I can access student lists and Quran data without internet.

**Acceptance Criteria:**

**Given** I have loaded the app while online
**When** I go offline
**Then** student lists, halaqah data, and Quran reference data are available from local cache (FR45)
**And** the Service Worker intercepts requests and serves cached data
**And** IndexedDB stores structured data for offline queries

### Story 7.2: Offline Queue and Auto-Sync

As a user,
I want my offline changes to sync automatically when internet returns,
So that I don't have to manually upload my work.

**Acceptance Criteria:**

**Given** I have recorded recitation or attendance while offline
**When** internet connectivity returns
**Then** queued changes sync automatically without user intervention (FR46, NFR18)
**And** a sync status indicator shows progress
**And** failed syncs retry with exponential backoff up to 3 times (NFR20)
**And** after 3 failures, the user is alerted to retry manually

### Story 7.3: Sync Conflict Handling

As a Center Admin,
I want sync conflicts to be handled gracefully with a visible log,
So that no data is silently lost.

**Acceptance Criteria:**

**Given** two users edited the same record while offline
**When** both sync their changes
**Then** last-write-wins is applied (FR47)
**And** a conflict log entry is created showing: what was overwritten, by whom, when
**And** Center Admin can view the conflict log

## Epic 8: Dashboard & Reporting

Center Admin can view center performance data through a dashboard.

### Story 8.1: Center Overview Dashboard

As a Center Admin,
I want to see a dashboard with my center's key statistics,
So that I can understand my center's performance at a glance.

**Acceptance Criteria:**

**Given** I am logged in as Center Admin
**When** I view my dashboard (FR35)
**Then** I see: total students, total halaqat, active teachers, overall attendance rate
**And** data refreshes when I load the page
**And** the dashboard loads within 3 seconds on 3G (NFR1)

### Story 8.2: Attendance Trends

As a Center Admin,
I want to see attendance trends over time,
So that I can identify patterns and take action on declining attendance.

**Acceptance Criteria:**

**Given** attendance data exists for multiple days
**When** I view attendance trends (FR36)
**Then** I see a chart or table showing attendance rate by week/month
**And** I can filter by specific halaqah or view center-wide

### Story 8.3: Recitation Activity & Student Progress

As a Center Admin,
I want to view recitation activity per halaqah and individual student progress,
So that I can monitor teaching effectiveness and student advancement.

**Acceptance Criteria:**

**Given** recitation data has been recorded
**When** I view recitation activity (FR37)
**Then** I see sessions recorded today and this week per halaqah
**And** I can drill into a student to see their full recitation history (FR38)
**And** I can see each student's current hifz position (last surah/ayah completed)

## Epic 9: Localization & Mobile Optimization

Users can switch languages and the UI is optimized for mobile use.

### Story 9.1: Arabic and Somali Language Support

As a user,
I want to switch the UI language between Arabic and Somali,
So that I can use the app in my preferred language.

**Acceptance Criteria:**

**Given** the app is loaded
**When** I select Arabic or Somali from language settings (FR42)
**Then** all UI text switches to the selected language
**And** Arabic triggers RTL layout direction (FR43)
**And** Somali triggers LTR layout direction
**And** language preference persists across sessions
**And** Quranic text always remains in Arabic regardless of UI language

### Story 9.2: Mobile-Optimized UI

As a user,
I want all UI elements to be optimized for mobile use,
So that I can use the app comfortably on my phone in the masjid.

**Acceptance Criteria:**

**Given** I am using the app on a mobile device
**When** I interact with any UI element
**Then** all tap targets are at least 44x44px (FR44)
**And** text is readable without zooming
**And** forms use appropriate mobile input types (tel for phone, etc.)
**And** total PWA bundle size is under 500KB compressed (NFR4)
**And** page loads under 3 seconds on 3G (NFR1)
