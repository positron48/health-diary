# Frontend and UX plan

Status: approved direction for the Phase 6 redesign; application code is not
changed by this document.

Audience: product owner, frontend/backend engineers and QA.

## 1. Outcome

The web app should feel like a calm personal journal, not a medical dashboard
or an admin panel. The common mobile path is:

1. open the app;
2. see pending items and today's confirmed timeline;
3. confirm or correct a candidate in one tap;
4. move to the calendar or analytics only when reviewing history.

The interface remains Russian-first, responsive and installable as a PWA. It
must not diagnose, claim causality or suggest medication changes.

## 2. Current boundary

The current frontend is a minimal vertical-slice shell:

- all behavior and CSS live in `web/src/App.vue`;
- there is no router, view hierarchy, component test runner or UI kit;
- confirmed and pending data are rendered as technical event names and raw
  JSON attributes;
- the calendar endpoint is not represented as a month grid;
- analytics is one text block;
- account deletion is visually adjacent to everyday journal actions;
- loading, empty, expired-session, conflict and retry states are not modeled
  independently.

The backend already supports the basic login, event list, pending batch
confirmation/rejection, deletion/restore, calendar read, summary and direct
CSV/JSON export paths. It does not yet implement all contracts required by the
target product UI; these gaps are listed in section 10.

## 3. Information architecture

### 3.1 Primary navigation

Use five destinations:

| Destination | Route | Purpose |
|---|---|---|
| Сегодня | `/today` | default screen: pending notice, open episode and today's timeline |
| Календарь | `/calendar/:month?` | month review with display modes and selected day |
| Аналитика | `/analytics` | coverage-first 7/30/60/90-day summaries |
| Входящие | `/pending` | all unconfirmed extraction batches; badge shows count |
| Ещё | `/settings` | timezone, privacy, export, sessions and destructive actions |

On phones, show `Сегодня`, `Календарь`, `Аналитика` and `Входящие` in a fixed
bottom bar; put settings behind the avatar/menu. On desktop, use a compact left
sidebar with all five destinations. A pending-count badge must be visible in
either navigation.

`Входящие` is a separate destination even though its first item is repeated on
`Сегодня`: unconfirmed facts are the most important correctness task, but they
must not crowd every screen.

### 3.2 Secondary routes

- `/day/:date` — full local-day timeline.
- `/episodes/:id` — headache episode detail.
- `/events/:id/edit` — focused edit form, usable as a modal on desktop and a
  full page on mobile.
- `/login` — Telegram challenge/code flow outside the authenticated shell.
- `/privacy` — readable privacy boundary and retention summary.

Route query is the source of truth for shareable, non-sensitive UI state:
calendar mode, analytics period and selected filters. Do not put decrypted
entry text in the URL or browser storage.

## 4. Application shell and responsive behavior

### Phone

```text
┌──────────────────────────────┐
│  Добрый вечер          (АФ)  │
│  Среда, 22 июля              │
├──────────────────────────────┤
│  2 записи ждут проверки  →   │
├──────────────────────────────┤
│  Сегодня                     │
│  15:10  Головная боль  6/10 │
│  15:25  Ибупрофен      400 мг│
│                              │
│  [Посмотреть весь день]      │
├──────────────────────────────┤
│ Сегодня Календарь Аналитика  │
│              Входящие (2)    │
└──────────────────────────────┘
```

- One content column, `16px` side padding, cards no wider than the viewport.
- Bottom navigation respects `safe-area-inset-bottom` and never covers content.
- Tap targets are at least `44×44px`.
- Detail and edit flows use a full-height sheet/page rather than a tiny modal.

### Desktop/tablet

```text
┌──────────────┬────────────────────────────┬──────────────────┐
│ Сегодня      │  Июль 2026          ‹  ›   │  22 июля        │
│ Календарь    │  [Обзор][Боль][Сон]...     │  Сводка дня     │
│ Аналитика    │                            │  Хронология      │
│ Входящие (2) │       month grid           │  событий         │
│ Ещё          │                            │                  │
└──────────────┴────────────────────────────┴──────────────────┘
```

- Left navigation is `220–240px`.
- Calendar uses the central pane; selecting a day opens a persistent right
  pane around `340–380px` without losing month context.
- Content pages other than calendar use a readable maximum width of `1120px`.

## 5. Visual direction

### 5.1 Character

Calm, warm and precise: off-white background, white surfaces, dark green text
and restrained semantic accents. Avoid hospital blue everywhere, neon
gradients, glass effects, mascots and dense admin-table styling.

Use the system font stack initially so the app remains fast, private and
offline-friendly. Use tabular numerals for dates, time and metrics.

### 5.2 Tokens

Put all values in CSS custom properties rather than inside view components.

| Role | Direction |
|---|---|
| Page background | warm gray-green `#F5F7F4` |
| Surface | white `#FFFFFF` |
| Primary text | near-black green `#17211B` |
| Secondary text | muted green-gray `#5F6D64` |
| Primary action | forest `#246B45` with accessible hover/pressed states |
| Border | soft gray-green `#DCE5DE` |
| Pain | coral/red, never used without icon or text |
| Medication | amber |
| Sleep | indigo |
| Activity | blue/teal |
| Wellbeing | violet |
| Success | green; Danger | dark red |

Spacing scale: `4, 8, 12, 16, 24, 32, 48px`. Card radius: `16px`; control
radius: `10–12px`. Use a subtle border as the main separation and only a very
soft shadow for elevated sheets.

All text/background and control states must meet WCAG AA. Color is never the
only carrier of pain level, event kind, pending status or validation error.

### 5.3 Content style

- Display `Головная боль`, not `pain_observation` or `Наблюдение боли`; use phase-aware titles such as `Головная боль · началась` / `усилилась` / `прошла`.
- Display medication as the stated name (`Цитрамон`), not only the generic kind label.
- Event cards read API keys `name_raw`/`normalized_name`, `locations`, `qualities`, `associated_symptoms`, `entry_id`.
- Display `Время указано приблизительно`, not internal precision enums.
- Display unknown values as `Не указано`, not `null`, `0` or an empty gap. Calendar max intensity without a score is `?/10`.
- Use `Возможная связь` and exact sample counts; never `причина` or `триггер`.
- Avoid guilt language such as “вы пропустили день”. Say “За этот день нет
  записей”.
- Put the medical limitation in a quiet persistent footer and next to analytic
  interpretations, not as a blocking warning on every screen.

### 5.4 Motion

Use only short `150–200ms` transitions for sheets, tab underline and optimistic
state feedback. Respect `prefers-reduced-motion`. Do not animate charts or
numbers by default.

## 6. Screen specifications

### 6.1 Login

One centered card with two explicit steps:

1. `Открыть бота` creates the challenge and opens Telegram.
2. Six separated or visually grouped code cells accept paste and
   `autocomplete="one-time-code"`.

Show challenge expiry, a resend action after expiry and stable error copy for
wrong/expired/locked codes. Preserve the entered code after a temporary network
failure. On desktop, show a QR code only later if it can be generated locally;
it is not required for the first redesign.

### 6.2 Today

Order content by urgency:

1. compact pending banner with count;
2. open-headache card with start time, last observation and `Открыть эпизод`;
3. today's chronological events;
4. quiet empty state with a Telegram deep link and one realistic example;
5. small 7-day snapshot only after the timeline, not a dashboard wall.

Do not add web capture in this phase: Telegram remains the documented primary
entry channel and there is no web-create API contract yet.

### 6.3 Pending inbox

Each extraction batch is one card. It shows:

- message time and source `Telegram`;
- a human-readable list of candidate event cards;
- approximate/unknown field badges;
- primary `Всё верно`, secondary `Исправить`, tertiary `Отклонить` actions;
- source text only after `Показать исходную запись` and a protected no-store
  request.

Confirmation removes the card optimistically and offers a short toast. Reject
requires a lightweight confirmation sheet explaining that the source entry is
retained. Correction opens a text flow only after its API exists; precise field
changes use the event edit form.

### 6.4 Calendar

Header contains month navigation, `Сегодня` and multi-select layer chips:
`Боль`, `Лекарства`, `Активность`, `Сон`, `Самочувствие`, `Контекст`, `Погода`.

Each day cell stays compact:

- pain: icon + max intensity (`6`), never episode count or `/10`;
- medication: pill icon + intake count;
- activity: icon + `45м`;
- sleep: icon + `7ч`;
- wellbeing/motivation: icon + score;
- weather: icon + one temperature;
- context: thin continuous ribbon; city/type label only at the visible segment start.

Present layers share a segmented color stripe (not a blended gradient). Empty cell means `Нет записей`. Switching layers must preserve month and selected day and must not refetch. Selecting a day loads a no-store preview of its ten latest confirmed events.

### 6.5 Day timeline

The header shows local date, counts and pending-state notice. Events are grouped
chronologically, with a visible connector only when it improves episode
continuity. Every event card has:

- event icon and Russian title;
- exact or approximate local time;
- only meaningful known fields in the collapsed state;
- `Изменить`, `Удалить` and `Исходная запись` in an overflow menu;
- episode link where applicable.

After soft deletion, keep a local undo toast for the supported restore window.
If the server reports a revision conflict, keep the user's draft and offer
`Загрузить актуальную версию` instead of silently overwriting it.

Calendar and day timeline expose `Добавить запись`. The shared form accepts
free-form text, keeps the draft only in memory and submits it for extraction.
It must state that recognized facts appear in `Входящие` and do not affect
analytics before confirmation.

### 6.6 Event edit

Render a typed form per event kind. Shared fields are date/time, precision and
optional end time; domain fields remain nullable. Use numeric inputs with clear
ranges, multi-select chips for symptoms and explicit `Не указано` controls.

Submit only changed fields with the current `revision`. Inline validation maps
the API `fields` object to controls. Destructive delete is separated from save
and requires confirmation.

### 6.7 Episode detail

Top summary: `Открыт/Завершён`, start/end precision, duration only when known,
max recorded intensity and observation count. The body is one timeline of pain
observations and related medication. Unknown end is `Эпизод ещё открыт`, not a
zero duration.

Manual close/reopen is a secondary action with a focused form. No effectiveness
or causality statement is generated from missing follow-up data.

### 6.8 Analytics

Show coverage before outcome metrics:

1. selected period `7 / 30 / 60 / 90 дней` and timezone;
2. diary-day coverage, confirmed/pending counts and closed-episode coverage;
3. headache, medication, sleep, activity and wellbeing sections;
4. possible associations only after documented gates pass;
5. data limitations and formula version in a disclosure.

Prefer small line/bar charts with adjacent numeric summaries. Every chart has a
text alternative, visible sample size and useful empty/insufficient-data state.
No pie charts for time series and no red/green good/bad scoring of health data.

### 6.9 Settings and privacy

Separate sections:

- profile: timezone and locale;
- privacy: Telegram cloud-chat, LLM provider and retention explanation;
- data: CSV/JSON export;
- sessions: log out current/all devices when supported;
- danger zone: account deletion at the bottom behind recent re-authentication
  and exact consequences.

Never place account deletion in the everyday timeline. The deletion copy must
distinguish service data, Telegram chat history and encrypted backup expiry.

## 7. Shared UI states

Every data-owning view defines these states explicitly:

- initial skeleton shaped like the final content;
- background refresh without clearing visible data;
- empty with a useful next action;
- recoverable network error with retry;
- expired session with return path preserved;
- validation error attached to fields;
- stale revision conflict with the local draft preserved;
- destructive-action progress with duplicate submission disabled;
- offline shell explaining that private health data is not cached.

Use one toast region with `aria-live="polite"` for success and undo. Errors that
block the current task remain inline with `role="alert"`; do not rely only on
toasts.

## 8. Frontend file ownership

Use feature-oriented folders with a small shared layer. Do not move every piece
of state into a global store.

```text
web/src/
├── app/
│   ├── App.vue                 # root providers and RouterView only
│   ├── router.ts               # routes, auth guard, titles, scroll policy
│   └── AppShell.vue            # desktop sidebar/mobile bottom navigation
├── api/
│   ├── client.ts               # fetch, no-store, error parsing, 401 handling
│   ├── types.ts                # shared API envelopes; generated later if OpenAPI lands
│   ├── auth.ts
│   ├── calendar.ts
│   ├── journal.ts
│   ├── analytics.ts
│   └── settings.ts
├── components/
│   └── ui/                     # Button, Card, Sheet, Dialog, Toast, Skeleton,
│                              # SegmentedControl, EmptyState, FormField
├── features/
│   ├── auth/                   # challenge/code form and session composable
│   ├── calendar/               # MonthGrid, DayCell, mode legend
│   ├── events/                 # typed cards, formatters and edit forms
│   ├── episodes/               # episode summary and timeline
│   ├── pending/                # batch card/actions
│   ├── analytics/              # metric cards/charts/coverage
│   └── settings/               # export/privacy/session/delete sections
├── views/
│   ├── LoginView.vue
│   ├── TodayView.vue
│   ├── CalendarView.vue
│   ├── DayView.vue
│   ├── EpisodeView.vue
│   ├── PendingView.vue
│   ├── AnalyticsView.vue
│   ├── SettingsView.vue
│   └── PrivacyView.vue
├── composables/
│   ├── useAsyncState.ts        # load/refresh/error lifecycle
│   ├── useMediaQuery.ts
│   └── useToast.ts
├── styles/
│   ├── tokens.css              # color/type/space/radius/elevation variables
│   ├── base.css                # reset, typography, focus, reduced motion
│   └── utilities.css           # only repeated layout helpers
├── test/
│   ├── setup.ts
│   ├── fixtures/               # synthetic API payloads only
│   └── accessibility.ts
└── main.ts
```

Rules:

- `views/` compose features and own route-level fetching.
- `features/` understand the health domain but never call `fetch` directly.
- `api/` understands HTTP but never formats UI copy.
- `components/ui/` is domain-neutral and contains no event-kind switches.
- event kind labels, icons, colors and field renderers live in one registry in
  `features/events/eventRegistry.ts`.
- use local/composable state first. Add Pinia only when session, pending count
  or another value has multiple independent writers and prop/composable sharing
  is no longer clear.
- install Lucide, Chart.js, Vue Router, Vitest and Vue Test Utils when their
  first screen/test is implemented, not as unused dependencies.

## 9. Data and cache rules

- Authenticated health requests use `credentials: 'same-origin'` and
  `Cache-Control: no-store`.
- Do not store event payloads, raw text, codes or profile data in
  `localStorage`, IndexedDB, service-worker caches or client telemetry.
- The service worker caches versioned static assets and the offline shell only.
- Preserve harmless UI state such as calendar mode through route query; keep
  drafts only in memory.
- Background refresh keeps existing content and scroll position. Do not replace
  a stable event list when the response is unchanged.

## 10. Backend/API prerequisites

Before building each dependent screen, align the current handlers with
`docs/API.md` or deliberately update that specification. The current code and
the documented target differ in these exact places:

1. Pick one public prefix. The specification says `/api/v1`, while current
   handlers use root paths such as `/events`, `/calendar` and `/analytics/summary`.
2. Make challenge naming consistent: documented `telegram_deep_link` versus
   current `telegram_url`.
3. Return the documented JSON error envelope with stable `error.code` and field
   errors; current handlers mostly return plain text.
4. `GET /calendar` must return local-day aggregates and honor `mode`; it
   currently returns a flat event list.
5. Add `GET /days/{date}`, event detail and `PATCH /events/{id}` before day/edit
   views are considered complete.
6. Add episode list/detail/close/reopen endpoints before episode UX is enabled.
7. Add protected source-entry access with `no-store` and audit before showing
   raw source in the UI.
8. Ensure confirmed-event lists exclude pending/rejected/superseded data by
   contract instead of depending on the client label.
9. Add settings update and session-management endpoints before exposing their
   controls.
10. Decide whether MVP export stays immediate `GET /exports?format=...` or moves
    to the documented asynchronous export lifecycle; the frontend supports one
    explicit contract, not both implicitly.

These are contract corrections, not a reason to bypass user scoping, confirmed
data rules or the privacy boundary.

## 11. Implementation order

### Step 1 — foundation and visual baseline

- add router, shell, tokens, base styles and domain-neutral UI primitives;
- split login from the authenticated shell;
- add typed API client and stable error handling;
- add Vitest/Vue Test Utils and component test setup;
- build a development fixture page or tests for every UI primitive state.

Deliverable: login and empty authenticated shell work on phone and desktop.

### Step 2 — pending and today

- implement session bootstrap, pending-count badge and pending batch cards;
- replace raw event JSON with the centralized event renderer;
- implement today timeline and robust loading/error/empty states;
- add optimistic confirmation/rejection with conflict recovery.

Deliverable: the most common review/confirmation path is polished without
waiting for analytics or the full calendar.

### Step 3 — calendar and day review

- land the calendar/day aggregate API prerequisites;
- implement month grid, mode switch, day side pane/full mobile page;
- preserve route, scroll and selection through mode/month changes;
- add delete/undo and source disclosure.

Deliverable: confirmed data can be reviewed by month and day without exposing
raw text in bulk responses.

### Step 4 — typed editing and episodes

- land event PATCH/detail and episode API prerequisites;
- implement typed nullable forms and revision-conflict UX;
- implement episode summary/timeline/close/reopen flows.

Deliverable: all supported confirmed facts can be corrected and headache
episodes remain consistent.

### Step 5 — analytics

- land complete coverage/metric responses and insufficient-data reasons;
- implement range selection, numeric summaries and accessible charts;
- show associations only after backend gates pass.

Deliverable: every number exposes period, denominator/sample and formula
limitations.

### Step 6 — settings, privacy and PWA hardening

- implement profile, privacy, export, session and danger-zone sections;
- add static-shell-only service worker and offline explanation;
- verify CSP, cache behavior, responsive layout and installed PWA behavior.

Deliverable: privacy/export/deletion are understandable and production-ready
without making sensitive data available offline.

## 12. Tests and visual verification

Automated frontend checks added to `make check`:

```text
vue-tsc --noEmit
eslint
vitest run
vite build
```

Minimum component/interaction coverage:

- login success, wrong code, expired challenge and network retry;
- all calendar modes, month boundaries and selected-day preservation;
- unknown value versus zero versus no data versus explicit no-headache;
- pending batch confirm/reject/stale version;
- day ordering and exact/approximate time copy;
- typed validation and revision-conflict draft preservation;
- delete/undo/restore;
- analytics insufficient-data gates and sample counts;
- session expiry and return-path preservation;
- keyboard focus, dialog focus trap, `aria-live` feedback and reduced motion.

Visual QA viewports: `360×800`, `390×844`, `768×1024`, `1280×800` and
`1440×900`. Check long Russian labels, large text at 200%, dark browser chrome,
safe-area bottom inset, empty/dense months and charts with missing values.

The critical fake-provider E2E remains:

```text
Telegram entry → pending batch → web login → confirm → calendar/day →
edit with revision → analytics update → delete/undo → export
```

## 13. Definition of Done

- The common confirmation task is completable with one primary action from
  `Сегодня` or `Входящие`.
- No internal enum or raw JSON is visible in ordinary UI.
- Phone navigation never covers content and desktop calendar preserves context.
- Unknown, absent and explicit negative observations are visually distinct.
- All authenticated API responses and browser storage follow the no-store rule.
- Pending/deleted data never appears in analytics.
- Editing preserves user input through validation, conflict and network errors.
- Analytics shows observation period and sample size before interpretation.
- Component, accessibility, build and critical E2E checks pass through
  `make check`.
- Final implementation receives visual approval at the required viewports.

## 14. Assumptions and non-goals

- Telegram remains the primary capture channel for this redesign.
- Russian and light theme ship first; dark theme is a later enhancement after
  semantic tokens are stable.
- No clinician mode, sharing, voice, web-created diary entry, reminder setup or
  medical safety alerts are added implicitly.
- Product name/logo can change later without blocking the information
  architecture or component structure.
