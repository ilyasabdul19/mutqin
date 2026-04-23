---
stepsCompleted: ['step-01-init', 'step-02-discovery', 'step-02b-vision', 'step-02c-executive-summary', 'step-03-success', 'step-04-journeys', 'step-05-domain', 'step-06-innovation-skipped', 'step-07-project-type', 'step-08-scoping', 'step-09-functional', 'step-10-nonfunctional', 'step-11-polish', 'step-12-complete']
inputDocuments: ['quran_center_prd.pdf (original 18-page PRD reviewed in conversation)']
workflowType: 'prd'
documentCounts:
  briefs: 0
  research: 0
  brainstorming: 0
  projectDocs: 1
classification:
  projectType: 'saas_b2b'
  domain: 'edtech'
  complexity: 'medium'
  projectContext: 'greenfield'
  primaryMarkets: ['Somalia', 'Kenya']
  infrastructureStrategy: 'self-hosted-low-cost'
  languages: ['Arabic', 'Somali', 'Swahili', 'English']
  techConstraints: ['no-supabase', 'offline-first', 'mobile-first']
---

# Product Requirements Document — Mutqin (مُتقِن)

**Author:** Ilyas Abdulkadir
**Date:** 2026-04-13
**Version:** 1.0
**Status:** Draft

## Executive Summary

Mutqin (مُتقِن) is a SaaS platform that gives Quran memorization centers in East Africa their first digital presence and operational management system. Target markets are Somalia and Kenya, where tens of thousands of halaqat operate entirely through paper notebooks, Telegram groups, and WhatsApp — with zero online visibility, no structured data, and no way for parents to track their children's progress.

The platform combines three capabilities in one system: a simple landing page builder for centers (announcements, info, registration forms), a mobile-first management tool for teachers (recitation recording, attendance, progress tracking), and automated parent communication via WhatsApp. Built for low-bandwidth environments on self-hosted infrastructure to keep costs near zero.

No existing Quran center management system targets Africa. All 16+ competitors focus on Saudi Arabia and the Gulf — pricing, languages, and infrastructure assumptions that don't work in Mogadishu or Mombasa. Mutqin's core insight: these centers don't need a "better management system" — they need to exist digitally for the first time.

The landing page is the entry point. A center admin shares a link instead of a Telegram group invite. Parents register through a form instead of sending a voice note. Structured data flows automatically into attendance, recitation tracking, and progress reports — replacing the paper notebook without changing the teacher's workflow.

**Growth flywheel:** Center creates free landing page → shares link on WhatsApp/Telegram → parents register via form → teachers track recitation on mobile → parents receive automated progress reports → other centers see the results and want their own.

## Project Classification

| Attribute | Value |
|-----------|-------|
| **Product Type** | SaaS B2B — multi-tenant platform (web PWA + WhatsApp integration) |
| **Domain** | EdTech — Quran memorization management |
| **Complexity** | Medium — offline-first, multi-language, Quranic content accuracy |
| **Project Context** | Greenfield — no existing codebase |
| **Primary Markets** | Somalia, Kenya |
| **Infrastructure** | Self-hosted, low-cost VPS (no Supabase) |
| **Team** | 2 developers |
| **Timeline** | 1-3 months for core features |

## Success Criteria

### User Success

- **Center Admin:** Creates a landing page and shares the link within 15 minutes of signup. First parent registers through the form within 48 hours.
- **Teacher:** Records recitation for a full halaqah (15-20 students) faster than a paper notebook — under 10 minutes per session.
- **Parent:** Receives first automated progress update about their child without asking the teacher directly.
- **"Aha!" moment:** The admin sees structured student data (attendance rates, recitation progress) for the first time — data that never existed before.

### Business Success

- **3-month target:** 5-10 centers actively using Mutqin (at least weekly recitation recording)
- **Validation signal:** Test center fully stops using paper notebooks for recitation tracking
- **Organic growth indicator:** At least 1 center signs up without direct outreach (word of mouth or seeing another center's landing page)
- **Infrastructure cost:** Hosting costs remain under $20/month while serving 5-10 centers

### Technical Success

- **Mobile performance:** App loads and is functional on low-end Android phones over 3G connections
- **Offline capability:** Teachers can record recitation and attendance without internet; syncs when connection returns
- **Infrastructure cost efficiency:** Self-hosted stack runs on a single $5-10/month VPS for initial scale
- **Multi-tenancy:** Each center's data is fully isolated — no cross-center data leaks

### Measurable Outcomes

| Metric | Target | Measurement |
|--------|--------|-------------|
| Centers onboarded (3 months) | 5-10 | Active accounts with at least 1 halaqah |
| Teacher daily active usage | 60%+ | Teachers who record recitation at least 4x/week |
| Landing page → registration conversion | 30%+ | Parents who complete registration after visiting page |
| Recitation recording time | <10 min/halaqah | Time from first to last student per session |
| App load time on 3G | <3 seconds | Lighthouse/field measurement |
| Monthly hosting cost | <$20 | Infrastructure billing |

## User Journeys

### Journey 1: Super Admin Onboards a New Center

**Persona:** Ilyas — Platform owner, runs Mutqin.

**Scene:** Sheikh Omar in Garissa, Kenya sends a WhatsApp voice note: "My friend Ustadh Abdi showed me his center's page — how do I get one?" Ilyas logs into the Super Admin dashboard, creates a new organization ("Markaz Al-Noor"), and generates an invite link. Sheikh Omar clicks the link, sets up his admin account with his phone number, and within an hour has created 3 halaqat and invited 2 teachers — without a single support call.

**Requirements revealed:** Super admin tenant management, invite link generation, organization creation, platform-wide dashboard, center activity monitoring.

### Journey 2: Center Admin Sets Up Digital Presence

**Persona:** Ustadh Abdi — 45, runs Markaz Ibn Kathir in Mogadishu. 10 halaqat, 15 teachers, ~150 students. Manages everything via Telegram group and paper notebook. Samsung Galaxy A13, spotty 3G.

**Scene:** After onboarding, Ustadh Abdi taps through a simple wizard — uploads the center's logo, writes a description in Arabic, adds the location and schedule. He creates a registration form and the system generates `mutqin.app/ibn-kathir`. He pastes the link into his Telegram group of 200+ parents. Within 2 hours, 23 parents submit forms. He sees structured data — names, ages, phone numbers — instead of scrolling through 50 voice notes. That evening, he creates halaqat, assigns teachers, and distributes students.

**Requirements revealed:** Landing page builder, registration form builder, public shareable URL, form submission dashboard, halaqah CRUD, teacher assignment, student enrollment.

### Journey 3: Teacher Records Daily Recitation

**Persona:** Mu'allim Yusuf — 28, teaches 18 boys aged 10-14 after Fajr prayer. Tecno Spark phone. Lost his paper notebook twice.

**Scene:** 6:15 AM in the masjid. Yusuf opens Mutqin — student list loads instantly from cache. First student Ahmed recites Surah Al-Mulk ayat 1-10. Yusuf taps Ahmed's name, selects "New Hifz," picks surah/ayah range, grades "Jayyid Jiddan," adds a note about ghunnah. Swipes to next student. Large tap targets, smart defaults based on last session. All 18 students recorded in 17 minutes. Taps "Mark Attendance" — 16 present, 2 absent with one tap each. Data syncs in background when signal returns.

**Requirements revealed:** Offline-capable student list, recitation recording (type, surah/ayah, grade, notes), smart defaults, bulk attendance, background sync, mobile-optimized UI.

### Journey 4: Parent Discovers and Registers

**Persona:** Hoyo Amina — 35, mother of 3 boys in Mogadishu. Uses WhatsApp daily but has never filled out an online form.

**Scene:** Neighbor forwards a WhatsApp link. Amina sees a clean Arabic page — center name, location, schedule, green "Register Your Child" button. Simple form: child's name, age, parent phone, previous hifz level (4 clear options). Done in 2 minutes. WhatsApp confirmation arrives. Next day, Ustadh Abdi assigns her son Khalid to Yusuf's halaqah and messages her the schedule. Amina tells two other mothers. They register that evening.

**Requirements revealed:** Mobile-optimized public landing page, simple registration form, WhatsApp confirmation, registration management, student-to-halaqah assignment.

### Journey Requirements Summary

| Journey | Key Capabilities |
|---------|-----------------|
| Super Admin Onboarding | Tenant management, invite links, organization CRUD, platform dashboard |
| Center Admin Setup | Landing page builder, registration forms, shareable URL, halaqah management |
| Teacher Daily Use | Offline recitation recording, surah/ayah picker, grading, bulk attendance, sync |
| Parent Registration | Public landing page, mobile-friendly forms, WhatsApp notifications |

**Cross-cutting requirements:** Offline-first architecture, WhatsApp integration, mobile-optimized UI, Arabic/Somali language support, low-bandwidth performance (3G).

## Domain-Specific Requirements

### Quranic Content Integrity

- All Quranic text uses approved Uthmani script — no custom rendering or modification
- Surah/ayah data sourced from established Quran APIs (quran.com) and stored locally
- 114 surahs, 6,236 ayat — static reference data, never user-editable
- RTL layout works flawlessly for Arabic content

### Student Data & Privacy

- Most students are minors (ages 6-18) — parent consent embedded in registration flow
- No student photos required in MVP
- Center data fully isolated via multi-tenancy — Center A never sees Center B's students
- Super Admin sees aggregate platform metrics only, not individual student data

### Offline & Connectivity

- Somalia/Kenya infrastructure: expect 2G-3G, frequent disconnections, power outages
- Critical path (recitation + attendance) works fully offline
- Sync strategy: queue locally, push when connection returns, handle conflicts gracefully
- Must function on low-end Android devices (1-2GB RAM, small screens)

### Cultural & Linguistic

- UI supports Arabic (RTL) and Somali (LTR) simultaneously — user chooses language
- Quranic grading terminology matches local usage: Mumtaz, Jayyid Jiddan, Jayyid, Maqbul, Da'if
- Halaqah scheduling accounts for prayer times (after Fajr, between Maghrib and Isha)
- Hijri calendar support for semester/term planning

## SaaS Platform Requirements

### Tenant Model

- **Single-database multi-tenancy** — all centers share one database, isolated by `organization_id`
- **Hierarchy:** Platform (Super Admin) → Organization (Center) → Halaqat → Students
- **Data isolation:** Row-level security. Teacher sees their halaqah only. Admin sees their center. Super Admin sees platform metrics.
- **Provisioning:** Super Admin creates organization → generates invite link → center admin activates via OTP

### Permission Matrix (RBAC)

| Permission | Super Admin | Center Admin | Teacher |
|-----------|:-----------:|:------------:|:-------:|
| Create/manage organizations | Yes | No | No |
| Platform-wide dashboard | Yes | No | No |
| Generate center invite links | Yes | No | No |
| Create/edit landing page | No | Yes | No |
| Manage registration forms | No | Yes | No |
| Create/manage halaqat | No | Yes | No |
| Assign teachers to halaqat | No | Yes | No |
| Enroll students | No | Yes | No |
| View all center data | No | Yes | No |
| Generate teacher invite links | No | Yes | No |
| Record recitation | No | No | Yes |
| Mark attendance | No | No | Yes |
| View own halaqah students | No | No | Yes |

### Pricing Model

**Model:** One-time license fee + monthly hosting/maintenance

| | Free | Asaasi (Basic) | Mutqin Pro |
|---|---|---|---|
| **License fee** | $0 | $49 one-time | $149 one-time |
| **Hosting/maintenance** | $0 | $3/month | $7/month |
| **Halaqat** | 1 | Up to 10 | Unlimited |
| **Students** | 20 | 150 | Unlimited |
| **Teachers** | 1 | 10 | Unlimited |
| **Landing page** | Branded "Powered by Mutqin" | Clean | Custom domain |
| **Registration forms** | 1 basic form | Custom forms | Custom forms |
| **WhatsApp reports** | No | Yes | Yes |
| **Dashboard analytics** | Basic | Full | Full |

**Payment:** M-Pesa (primary), bank transfer (secondary). No credit card dependency.

**Free tier strategy:** Growth engine — "Powered by Mutqin" branding on landing page provides organic marketing.

### Integrations

| Integration | Priority | Purpose |
|-------------|----------|---------|
| WhatsApp Business API | Phase 2 | Automated parent reports, registration confirmations |
| Quran Data API | Phase 1 | Surah/ayah reference data (stored locally) |
| M-Pesa | Phase 2 | License and maintenance fee collection |
| SMS Gateway | Phase 2 | OTP delivery, WhatsApp fallback |

### Technical Architecture

- **Self-hosted infrastructure** — VPS-based (Hetzner/Contabo), $5-10/month initial
- **No Supabase** — PostgreSQL directly with lightweight backend framework
- **Mobile-first PWA** — not native app for MVP. Works across all devices, installable on Android
- **Offline-first** — Service Worker + IndexedDB for local storage, background sync queue
- **API-first design** — REST API backend, consumed by PWA frontend and future native apps
- **Static Quran data** — 114 surahs + 6,236 ayat cached locally, never fetched per-request
- **Authentication:** Phone number + OTP (no email dependency)
- **Invite-based onboarding:** Super Admin → invite link → Center Admin → invite link → Teacher
- **Localization:** i18n from day one — Arabic (RTL) and Somali (LTR)
- **Landing page URLs:** `mutqin.app/{center-slug}` — short, shareable, WhatsApp-friendly

## Project Scope & Phased Development

### MVP Strategy

**Approach:** Problem-solving MVP — deliver the smallest product that replaces paper notebooks and Telegram/WhatsApp groups for ONE center, then validate adoption before scaling.

**Core Bet:** If a center admin can create a landing page in 15 minutes and a teacher can record recitation faster than a notebook, centers will adopt and spread the word organically.

**Resources:** 2 developers, 1-2 months. One backend/API, one frontend/PWA.

### Phase 1 — MVP (Month 1-2)

| Feature | Justification |
|---------|--------------|
| Super Admin dashboard | Gated onboarding — you control who joins |
| Invite link generation | Center admins can't self-register |
| Landing page builder | The hook — gets centers in the door |
| Registration forms | Replaces WhatsApp voice notes — instant structured data |
| Halaqah CRUD | Core management — create, assign teacher, set schedule |
| Student enrollment | Assign registered students to halaqat |
| Teacher invite flow | Admin adds teachers via invite link + OTP |
| Recitation recording | Daily workflow — new hifz + review, grade, notes |
| Surah/ayah picker | References all 114 surahs / 6,236 ayat |
| Attendance | One-tap per student, bulk marking |
| Basic admin dashboard | Student count, attendance rate, active halaqat |
| Offline recitation + attendance | Non-negotiable for mosque environments |
| Arabic + Somali UI | Launch languages |
| Phone OTP authentication | No email dependency |

**Explicitly NOT in MVP:** WhatsApp automated reports, payment/billing, exams, certificates, hifz progress map, parent portal, English/Swahili, multi-center hierarchy, AI analytics.

### Phase 2 — Growth (Month 3-4)

- WhatsApp Business API integration (automated daily/weekly parent reports)
- Student progress tracking with visual hifz map (surah-by-surah completion)
- Basic monthly exam system
- Parent read-only view (web, no separate app)
- English + Swahili language support
- In-app billing (license + maintenance fee collection via M-Pesa)

### Phase 3 — Expansion (Month 6+)

- Multi-center management (Jam'iyyah → Centers → Halaqat hierarchy)
- Certificate generation (juz'/surah completion, customizable design)
- Advanced analytics dashboard (compare halaqat, teacher performance, student trends)
- AI-powered insights (predict dropout risk, suggest review schedules)
- Native mobile app (if PWA proves insufficient)
- Custom domain support for Pro tier landing pages

## Functional Requirements

### Platform Administration

- **FR1:** Super Admin can create a new center organization with name, location, and admin contact details
- **FR2:** Super Admin can generate a unique invite link for a center admin to activate their account
- **FR3:** Super Admin can view a platform-wide dashboard showing all centers, their status, and aggregate activity metrics
- **FR4:** Super Admin can deactivate or suspend a center organization
- **FR5:** Super Admin can view which centers are on which pricing tier (Free/Asaasi/Pro)

### Center Landing Page & Public Presence

- **FR6:** Center Admin can create and publish a public landing page for their center
- **FR7:** Center Admin can add center information to the landing page (name, logo, description, location, schedule)
- **FR8:** Center Admin can post announcements visible on the landing page
- **FR9:** Center Admin can create registration forms with configurable fields
- **FR10:** Center Admin can view and manage submitted registration applications
- **FR11:** Center Admin can approve or reject registration applications
- **FR12:** Parents can view a center's public landing page without authentication
- **FR13:** Parents can submit a registration form for their child without creating an account

### Halaqah Management

- **FR14:** Center Admin can create a halaqah with name, schedule, and maximum capacity
- **FR15:** Center Admin can assign a teacher to a halaqah
- **FR16:** Center Admin can enroll students into a halaqah
- **FR17:** Center Admin can transfer a student between halaqat
- **FR18:** Center Admin can view a list of all halaqat with teacher assignments and student counts
- **FR19:** Center Admin can deactivate a halaqah

### User & Access Management

- **FR20:** Center Admin can generate invite links for teachers
- **FR21:** Teachers can activate their account via invite link and phone OTP
- **FR22:** All users can authenticate via phone number and OTP (no email required)
- **FR23:** System enforces role-based access: Super Admin sees platform, Center Admin sees their center, Teacher sees their halaqah only
- **FR24:** Center Admin can deactivate a teacher account

### Recitation Recording

- **FR25:** Teacher can view their halaqah's student list (available offline)
- **FR26:** Teacher can record a recitation session for a student specifying: type (new hifz / near review / far review), surah, ayah range, grade, and optional notes
- **FR27:** Teacher can see a student's last recorded position to determine where to continue
- **FR28:** System suggests the next surah/ayah based on the student's last session
- **FR29:** Teacher can record recitation for all students in a halaqah in a single session flow
- **FR30:** Teacher can record recitation while offline; data syncs when connectivity returns

### Attendance

- **FR31:** Teacher can mark attendance for all students in their halaqah (present/absent)
- **FR32:** Teacher can mark attendance in bulk with defaults (all present, then mark exceptions)
- **FR33:** Teacher can mark attendance while offline; data syncs when connectivity returns
- **FR34:** Center Admin can view attendance records by halaqah, student, or date range

### Dashboard & Reporting

- **FR35:** Center Admin can view a dashboard showing: total students, total halaqat, active teachers, overall attendance rate
- **FR36:** Center Admin can view attendance trends over time
- **FR37:** Center Admin can view recitation activity per halaqah (sessions recorded today/this week)
- **FR38:** Center Admin can view a student's recitation history and current hifz position

### Quran Reference Data

- **FR39:** System provides a complete reference of 114 surahs and 6,236 ayat in Uthmani script
- **FR40:** Quran data is stored locally on device for offline access
- **FR41:** Quran text is read-only — no user can modify Quranic content

### Localization & Accessibility

- **FR42:** Users can switch the UI language between Arabic and Somali
- **FR43:** System supports RTL layout for Arabic and LTR layout for Somali
- **FR44:** All UI elements use large tap targets optimized for mobile use

### Offline & Sync

- **FR45:** System caches student list, halaqah data, and Quran reference data locally for offline use
- **FR46:** Recitation and attendance recorded offline are queued and synced automatically when connectivity returns
- **FR47:** System handles sync conflicts using last-write-wins with conflict log visible to admin

## Non-Functional Requirements

### Performance

- **NFR1:** Page load time under 3 seconds on a 3G connection (1 Mbps)
- **NFR2:** Recitation recording form responds to tap input within 200ms
- **NFR3:** Student list for a halaqah (up to 30 students) renders within 1 second, including offline
- **NFR4:** Total PWA bundle size under 500KB (compressed) — excluding cached Quran data
- **NFR5:** Landing page loads under 2 seconds on 3G for unauthenticated visitors
- **NFR6:** Surah/ayah picker renders full surah list within 500ms from local cache

### Security

- **NFR7:** All data in transit encrypted via TLS 1.2+
- **NFR8:** OTP codes expire after 5 minutes and are single-use
- **NFR9:** Multi-tenant data isolation enforced at database query level — no API endpoint can return data from another organization
- **NFR10:** Invite links are single-use and expire after 72 hours
- **NFR11:** Super Admin actions (create/suspend organization) are logged with timestamp and actor
- **NFR12:** No student PII exposed in public landing pages or URLs

### Scalability

- **NFR13:** System supports up to 50 concurrent centers on a single $10/month VPS
- **NFR14:** Database schema supports future horizontal scaling without migration
- **NFR15:** API response time remains under 500ms with 500 concurrent users

### Reliability

- **NFR16:** Offline mode supports full recitation and attendance recording for up to 7 days without sync
- **NFR17:** No data loss when device loses power during recording — auto-save on every field change
- **NFR18:** Sync queue processes automatically when connectivity returns without user intervention
- **NFR19:** System uptime target: 99% (allows ~7 hours downtime/month — realistic for self-hosted)
- **NFR20:** Failed sync attempts retry with exponential backoff up to 3 times before alerting the user

## Risks & Mitigation

### Technical Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Offline sync complexity delays MVP | High | High | Start with simple last-write-wins; improve post-MVP |
| PWA performance on low-end Android | Medium | High | <100KB JS budget, no heavy frameworks, test on Tecno Spark early |
| OTP delivery reliability in Somalia | Medium | Medium | Multiple SMS providers; WhatsApp OTP as backup |
| Incorrect Quranic text display | Low | High | Verified Uthmani font, read-only data, no user editing |
| Power outage during recording | Medium | Medium | Auto-save every field change locally |

### Market Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Centers resist switching from paper | High | High | Landing page is the hook — visible value before workflow change |
| Teachers find app slower than notebook | Medium | High | UX testing with real teachers at test center before rollout |
| Free tier users never convert to paid | Medium | Medium | Free tier limited to 1 halaqah/20 students — clear value cliff |

### Resource Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| 2-person team can't deliver in 2 months | Medium | Medium | Cut to landing page + registration + recitation only; add rest in month 2 |
| One developer leaves | Low | High | API-first architecture — frontend/backend independently deployable |
