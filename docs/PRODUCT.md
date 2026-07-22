# Product specification

Status: approved direction; MVP backend contour and a minimal web review shell
are implemented, while the target frontend redesign is specified in
`FRONTEND.md`.

Audience: product owner, backend/frontend engineers, QA.

## 1. Problem

Health observations are easy to forget and hard to enter into rigid forms. The product must let the user record them in natural Russian through Telegram and later inspect a structured timeline.

The primary initial use case is understanding headache episodes:

- when an episode starts and ends;
- how intensity and associated symptoms change;
- what medication was taken and what happened afterwards;
- what sleep, activity, stress, food or other observations preceded an episode;
- whether a pattern repeats often enough to be worth discussing with a clinician.

## 2. Product principles

1. **Capture first, structure second.** A useful raw note must never be lost because parsing failed.
2. **Fast confirmation.** The common path should take one message and one tap.
3. **Unknown is valid.** The system must not pressure the user to fill every field.
4. **Corrections are normal.** Natural-language correction and web editing are first-class flows.
5. **No causality claims.** The UI says “possible association” and always shows sample size.
6. **User owns the data.** Export and deletion are product requirements, not admin-only operations.
7. **Quiet UX.** Reminders are opt-in, rate-limited and never guilt-inducing.

## 3. Initial user and assumptions

- Single personal user at launch.
- Russian input and UI.
- Default timezone `Europe/Moscow`, configurable per user.
- Telegram is the main input channel; the web app is the main review/analytics channel.
- Schema and authorization remain user-scoped so multi-user support does not require data migration.
- Web app is responsive/PWA, not a native mobile app.

## 4. MVP scope

### 4.1 Telegram capture

- Accept private text messages only.
- Deduplicate Telegram updates.
- Preserve the raw entry before LLM processing.
- Parse one message into zero, one or multiple structured events.
- Return a compact summary with `Верно`, `Исправить`, `Удалить` actions.
- Accept a correction as a reply to the bot summary.
- If parsing fails, retain the note and offer retry/manual classification.
- Support `/start`, `/help`, `/today`, `/login`, `/privacy`.

### 4.2 Event types

- `pain` / headache episode start, update and end;
- `medication_intake`;
- `wellbeing`;
- `activity`;
- `sleep`;
- `food_drink`;
- `measurement`;
- `note`.

Headache-specific fields include intensity `0..10`, location, laterality, quality, associated symptoms and functional impact. All fields except event type and event time may be unknown.

### 4.3 Web

- Telegram-code authentication.
- Month calendar with modes: overview, pain, medication, activity, sleep, wellbeing.
- Day timeline with raw-source link and edit/delete controls.
- Free-form web capture from calendar/day views through the same extraction and confirmation flow as Telegram.
- Episode detail with observations, related medication and duration.
- Basic 7/30/60/90-day summary.
- Explicit pending/unconfirmed items inbox.
- CSV/JSON export.

### 4.4 Analytics

- headache days and headache-free days;
- episode count, duration and intensity;
- time-of-day/day-of-week distribution;
- medication days and recorded response;
- sleep/activity/wellbeing summaries;
- possible associations only after minimum data gates defined in `ANALYTICS.md`.

## 5. Explicitly outside MVP

- Voice transcription, photos and documents.
- Apple Health, Health Connect or wearable synchronization.
- Weather/location enrichment.
- Predictive alerts.
- Public registration or clinician accounts.
- Shared report links.
- Automated diagnosis, triage, medication advice or treatment changes.
- Native iOS/Android applications.

These are candidates for later phases, not hidden MVP requirements.

## 6. Primary journeys

### 6.1 Record and confirm

1. User sends: “Около трёх заболела голова справа, 6 из 10. Выпил ибупрофен 400.”
2. Raw entry is stored and acknowledged quickly.
3. LLM extracts a pain start and medication intake.
4. Bot replies with normalized facts and one-tap actions.
5. `Верно` marks all events confirmed.
6. Events appear in the calendar and analytics.

### 6.2 Correct

1. User taps `Исправить` or replies “не 400, а 200 мг”.
2. The correction is linked to the original entry/events.
3. The system proposes the new normalized result.
4. On confirmation, revision history is saved and projections are recalculated.

### 6.3 Continue/close pain episode

1. User later writes “Стало сильнее, 8 из 10, тошнит”.
2. Open episode context is supplied to extraction.
3. System links the observation to the likely open episode and asks only if ambiguous.
4. “Голова прошла примерно в шесть” closes the episode.

### 6.4 Web login

1. Login page creates an opaque challenge and shows a Telegram deep link.
2. User opens the same bot; the bot binds the challenge to the Telegram account and sends a 6-digit code.
3. User enters the code on the site.
4. Server consumes the challenge and sets a server-side session cookie.

## 7. UX requirements

- Confirmation message should fit on one phone screen for the common case.
- Calendar cells use icons/color plus accessible text; color alone is insufficient.
- All generated interpretations visibly distinguish exact and approximate time.
- Pending/unconfirmed data is visible but excluded from analytics.
- Every event can be corrected or deleted from bot and web.
- Empty days must be distinguishable from “no headache” check-ins; absence of data is not a headache-free day unless explicitly derived by an agreed rule.
- Analytics always displays observation period and sample size.

## 8. Acceptance criteria for first usable release

- A Telegram message with pain + medication produces two validated events.
- Duplicate delivery of the same Telegram update creates no duplicates.
- Unknown dose/time remains `NULL`; parser never fabricates it.
- Confirmation makes the event visible in calendar API within one second.
- Correction changes the rendered event and preserves an audit revision.
- Closing a headache computes a non-negative duration.
- Login code is hashed, expires, is one-use and locks after configured attempts.
- Calendar switches modes without refetching unrelated raw text.
- Deleted events disappear from normal API and analytics.
- Export returns all current confirmed data for the authenticated user.
- `make check` and GitLab required jobs pass.

## 9. Success indicators

Product success is not “LLM accuracy” alone. Track:

- median time from message to confirmation;
- percentage confirmed without correction;
- percentage of parsing failures;
- percentage of pain episodes with a recorded end;
- proportion of medication intakes with a later effect observation;
- number of active diary days per month;
- number of corrections after confirmation.

Do not send raw health text or field values to external product analytics.

## 10. Clinical/product references

- NICE recommends a headache diary record frequency, duration, severity, associated symptoms, medications and possible precipitants for at least 8 weeks: https://www.nice.org.uk/guidance/cg150/chapter/recommendations
- National Headache Foundation recommends starting with a small set of fields and expanding gradually: https://headaches.org/resources/headache-diary-keeping-a-diary-can-help-your-doctor-help-you/

These sources inform diary fields; they do not turn the application into a diagnostic tool.
