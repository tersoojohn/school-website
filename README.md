# School portal

A role-based school website in Go. One binary, one SQLite file, no JavaScript frameworks.

## Run it

```bash
cp .env.example .env            # then fill in SCHOOL_DATA_KEY (openssl rand -base64 32)
go mod tidy                     # fetches the SQLite driver and writes go.sum
set -a; source .env; set +a
SCHOOL_SECURE_COOKIES=false go run ./cmd/server     # false only for local http
```

On first start the server prints a **one-time setup link** for the principal in its log. Open it and choose a
password. Then:

1. **Academics**: add a term (mark it current), classes, subjects, and assign teachers.
2. **Staff / Students**: register people. Each registration produces a one-time setup link.
3. Give the link to the person (in person, printed slip, or a message). They choose their own password.
   Nobody else ever sees or knows it. Links work once and expire after 72 hours.

The default driver (`mattn/go-sqlite3`) needs a C compiler. To avoid that, swap in `modernc.org/sqlite`
in `internal/database/driver.go` (instructions in the file).

## Layout

```
cmd/server/            entrypoint, first-run principal, graceful shutdown
internal/config/       environment settings (including image slots)
internal/database/     SQLite open + embedded SQL migrations
internal/models/       shared types and role rules
internal/security/     password hashing, AES-GCM field encryption, tokens
internal/repository/   all SQL (parameterised): users, activation, results, analytics ...
internal/services/     grading, report cards and statistics (pure functions, tested)
internal/middleware/   sessions, role checks, CSRF, rate limit, security headers (CSP)
internal/handlers/     one file per audience: principal, admin, teacher, support, student ...
internal/routes/       every URL and the roles allowed on it, in one file
web/templates/         layouts, partials, pages/<audience>/
web/static/css/        tokens, layout, components, auth, student
web/static/js/         boot, app, charts, student (animations), sheet (roster tools)
web/static/fonts/      Newsreader and IBM Plex Sans, self-hosted (SIL OFL licences included)
```

## What each person sees

| | Principal | Admin | Teacher | Support | Student |
|---|---|---|---|---|---|
| Overview with key figures and charts | school-wide | enrolment and registrations | own classes | profile | own report |
| Analytics (averages, pass rates, grade distribution, term trend) | yes | no | per class, live | no | own, vs class average |
| Register students and staff, issue setup links | yes | yes | no | no | no |
| Classes, subjects, terms, assignments | yes | yes | no | no | no |
| All assigned students in one click, with counts | no | no | yes | no | no |
| Change scores for many students at once (with undo) | no | no | own subjects | no | no |
| Download roster or scores as spreadsheet | no | no | own | no | no |
| Return a submitted sheet to the teacher | yes | no | no | no | no |
| Bank details | view (logged) | view (logged) | own | no | no |
| Post announcements | all audiences | all audiences | own subjects | no | no |

## Frontend

- Navy, white and black palette; Newsreader for headings and figures, IBM Plex Sans for interface text.
- No icons or emoji anywhere. Charts are drawn from the data tables in the page, so a screen reader or a
  browser without scripts still gets the numbers as a table.
- Student area animations: the average ring draws and counts up, subject bars fill in order, grades are stamped,
  rows expand for the CA/exam breakdown. All of it respects "reduce motion".
- The teacher roster updates statistics and the grade chart as marks are typed; Enter and arrow keys move between
  students; nothing is saved until Save draft or Submit, and the server re-validates every score.
- Built to WCAG 2.2 AA: 4.5:1 text contrast, visible 3px focus rings, 44px controls, skip link, labelled forms.

### Photographs

The sign-in page and the student, teacher and support headers use three placeholder photographs from Unsplash
(free under the Unsplash License). Replace them any time, either by setting `SCHOOL_IMG_LOGIN`,
`SCHOOL_IMG_STUDENT`, `SCHOOL_IMG_STAFF` to your own https URLs, or by saving files in `web/static/img/`
and pointing the variables at `/static/img/yourfile.jpg`. If an image cannot load, the page falls back to plain navy.

Placeholders: Doug Linstedt (classroom, Lagos), Jaykumar Bherwani (students studying), Wojciech Pacześ (library).

## Security measures

- Passwords: PBKDF2-HMAC-SHA256, 600,000 iterations, per-user salt. 10+ characters with a letter and a number.
- **No temporary passwords.** New accounts hold an unusable password until the owner sets one from a single-use,
  72-hour setup link (only its SHA-256 is stored). A reset works the same way and signs the person out everywhere.
- Sessions: random 256-bit token in an HttpOnly, SameSite=Lax cookie (`__Host-` prefixed and Secure in production),
  stored hashed. 30 minute idle timeout, 12 hour maximum, rotated at sign-in, activation and password change.
- 5 wrong passwords lock an account for 15 minutes; per-IP rate limits on sign-in and activation; unknown
  usernames take the same time and get the same message as wrong passwords.
- CSRF token on every state-changing form plus Origin/Referer check.
- Access control twice: by role in `routes.go`, then per record in the handler (a teacher opens only their own sheets,
  a student reads only their own submitted results, admins cannot manage admins). Others get 404, so ids cannot be probed.
- Class averages shown to students only when at least 3 results exist, so no individual score is exposed.
- Bank account numbers encrypted with AES-256-GCM, bound to the owner; every view by principal or admin is audit-logged.
- Content-Security-Policy: scripts and styles only from this site (no inline code, no `eval`), images from this site and
  the configured image hosts, no framing. Also `no-store` caching, HSTS, request size and timeout limits.
- CSV downloads neutralise spreadsheet formulas. Templates auto-escape output; every SQL query is parameterised.
- Audit log of sign-ins, registrations, setup links, score submissions, exports and bank views.

## Deploying

Run behind HTTPS (Caddy or Nginx). Pass the original `Host` header through, set `SCHOOL_BASE_URL`, and set
`X-Real-IP` plus `SCHOOL_TRUST_PROXY=true` only if the proxy is yours. Back up `data/` and keep `SCHOOL_DATA_KEY`
safe: without it, saved bank account numbers cannot be read.

## Troubleshooting

**Locked out, or a setup link was lost or used up.** On the server, run
`go run ./cmd/server -setup-link principal` (use any username). It prints a fresh 72-hour link and puts that
account back into "awaiting setup", so the old password stops working. Open the link and choose a new password.
No data is lost. You need `.env` loaded first (`set -a; source .env; set +a`).

**Sign-in says "The sign-in page expired".** The server log prints `sign-in form rejected` with the reason.
Usually a browser blocked cookies or a stale tab was used; trying again works. If it repeats, check
`SCHOOL_BASE_URL` matches the address in your browser.

**403 "You do not have access to this page" after submitting a form (setup link, sign-in, anything).**
The site rejects form posts whose browser `Origin` does not match the server. This happens behind a proxy,
tunnel or port-forward that rewrites the `Host` header. Fix: set `SCHOOL_BASE_URL` to the exact address people
type in the browser (for example `https://portal.yourschool.ng`) and restart. The server log also prints a line
starting `request blocked: Origin does not match this site` showing the two values it compared.
With Nginx also add `proxy_set_header Host $host;`.

## Tests

`go test ./...` covers password hashing, encryption, grading, statistics and report rules.
# school-website
