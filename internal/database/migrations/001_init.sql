CREATE TABLE users (
    id                   INTEGER PRIMARY KEY AUTOINCREMENT,
    username             TEXT    NOT NULL UNIQUE COLLATE NOCASE,
    full_name            TEXT    NOT NULL,
    role                 TEXT    NOT NULL CHECK (role IN ('principal','admin','teacher','support','student')),
    password_hash        TEXT    NOT NULL,
    must_change_password INTEGER NOT NULL DEFAULT 1,
    active               INTEGER NOT NULL DEFAULT 1,
    failed_attempts      INTEGER NOT NULL DEFAULT 0,
    locked_until         INTEGER NOT NULL DEFAULT 0,
    phone                TEXT    NOT NULL DEFAULT '',
    created_by           INTEGER REFERENCES users(id),
    created_at           INTEGER NOT NULL DEFAULT (strftime('%s','now')),
    last_login_at        INTEGER
);
-- The school has exactly one principal account.
CREATE UNIQUE INDEX one_principal ON users(role) WHERE role = 'principal';

CREATE TABLE counters (
    name  TEXT PRIMARY KEY,
    value INTEGER NOT NULL
);

CREATE TABLE sessions (
    token_hash   TEXT    PRIMARY KEY,
    user_id      INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    csrf_token   TEXT    NOT NULL,
    created_at   INTEGER NOT NULL,
    last_seen_at INTEGER NOT NULL,
    expires_at   INTEGER NOT NULL,
    ip           TEXT    NOT NULL DEFAULT '',
    user_agent   TEXT    NOT NULL DEFAULT ''
);
CREATE INDEX idx_sessions_user ON sessions(user_id);

CREATE TABLE terms (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT    NOT NULL UNIQUE,
    is_current INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL DEFAULT (strftime('%s','now'))
);
CREATE UNIQUE INDEX one_current_term ON terms(is_current) WHERE is_current = 1;

CREATE TABLE classes (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE
);

CREATE TABLE subjects (
    id   INTEGER PRIMARY KEY AUTOINCREMENT,
    name TEXT NOT NULL UNIQUE,
    code TEXT NOT NULL UNIQUE
);

-- A subject taught in a class, optionally with a teacher assigned.
CREATE TABLE class_subjects (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    class_id   INTEGER NOT NULL REFERENCES classes(id),
    subject_id INTEGER NOT NULL REFERENCES subjects(id),
    teacher_id INTEGER REFERENCES users(id),
    UNIQUE (class_id, subject_id)
);

CREATE TABLE students (
    user_id        INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    class_id       INTEGER NOT NULL REFERENCES classes(id),
    guardian_name  TEXT NOT NULL DEFAULT '',
    guardian_phone TEXT NOT NULL DEFAULT '',
    date_of_birth  TEXT NOT NULL DEFAULT '',
    gender         TEXT NOT NULL DEFAULT ''
);

CREATE TABLE staff (
    user_id   INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    kind      TEXT NOT NULL CHECK (kind IN ('educating','non_educating')),
    job_title TEXT NOT NULL DEFAULT ''
);

-- Account numbers are stored encrypted (AES-256-GCM). Only the last four digits are kept in clear.
CREATE TABLE staff_bank_details (
    user_id        INTEGER PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    bank_name      TEXT    NOT NULL,
    account_name   TEXT    NOT NULL,
    account_number TEXT    NOT NULL,
    last4          TEXT    NOT NULL,
    updated_at     INTEGER NOT NULL
);

CREATE TABLE results (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    term_id          INTEGER NOT NULL REFERENCES terms(id),
    class_subject_id INTEGER NOT NULL REFERENCES class_subjects(id),
    student_id       INTEGER NOT NULL REFERENCES students(user_id),
    ca_score         REAL CHECK (ca_score   IS NULL OR (ca_score   >= 0 AND ca_score   <= 40)),
    exam_score       REAL CHECK (exam_score IS NULL OR (exam_score >= 0 AND exam_score <= 60)),
    status           TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','submitted')),
    recorded_by      INTEGER REFERENCES users(id),
    updated_at       INTEGER NOT NULL,
    UNIQUE (term_id, class_subject_id, student_id)
);
CREATE INDEX idx_results_student ON results(student_id, term_id);

CREATE TABLE announcements (
    id               INTEGER PRIMARY KEY AUTOINCREMENT,
    author_id        INTEGER NOT NULL REFERENCES users(id),
    title            TEXT    NOT NULL,
    body             TEXT    NOT NULL,
    scope            TEXT    NOT NULL CHECK (scope IN ('school','staff','subject')),
    class_subject_id INTEGER REFERENCES class_subjects(id) ON DELETE CASCADE,
    created_at       INTEGER NOT NULL DEFAULT (strftime('%s','now')),
    CHECK ((scope = 'subject') = (class_subject_id IS NOT NULL))
);

CREATE TABLE audit_log (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id    INTEGER REFERENCES users(id),
    action     TEXT NOT NULL,
    detail     TEXT NOT NULL DEFAULT '',
    ip         TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL DEFAULT (strftime('%s','now'))
);
CREATE INDEX idx_audit_created ON audit_log(created_at DESC);
