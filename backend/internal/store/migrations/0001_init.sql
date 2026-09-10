-- Cala 初始 schema。依据设计规格 §6。
-- 表名用 practice_round 而非 round：避免与 SQL 内建函数 ROUND() 混淆。
-- 所有时间列均为 RFC3339 UTC 文本，便于字典序排序与区间查询。

CREATE TABLE "user" (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    username      TEXT    NOT NULL UNIQUE,
    password_hash TEXT    NOT NULL,
    is_admin      INTEGER NOT NULL DEFAULT 0,
    disabled      INTEGER NOT NULL DEFAULT 0,
    created_at    TEXT    NOT NULL
);

CREATE TABLE session (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    token_hash TEXT    NOT NULL UNIQUE,
    user_id    INTEGER NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    expires_at TEXT    NOT NULL,
    created_at TEXT    NOT NULL
);
CREATE INDEX idx_session_user ON session(user_id);

CREATE TABLE setting (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE project (
    id                     INTEGER PRIMARY KEY AUTOINCREMENT,
    owner_id               INTEGER NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    title                  TEXT    NOT NULL,
    description            TEXT    NOT NULL DEFAULT '',
    question_count         INTEGER NOT NULL,
    cfg_json               TEXT    NOT NULL DEFAULT '{}',
    rule_source            TEXT    NOT NULL,
    share_token            TEXT    UNIQUE,
    share_token_updated_at TEXT,
    created_at             TEXT    NOT NULL,
    updated_at             TEXT    NOT NULL
);
CREATE INDEX idx_project_owner ON project(owner_id);

-- 订阅：纯只读跟随（D4）。退订即删除本行，并由应用层同时清除该用户在本项目的做题记录。
CREATE TABLE subscription (
    user_id    INTEGER NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    project_id INTEGER NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    created_at TEXT    NOT NULL,
    PRIMARY KEY (user_id, project_id)
);

-- 做题事实源之一。统计一律由此派生，不物化（D5）。
CREATE TABLE practice_round (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    project_id     INTEGER NOT NULL REFERENCES project(id) ON DELETE CASCADE,
    user_id        INTEGER NOT NULL REFERENCES "user"(id) ON DELETE CASCADE,
    seed           INTEGER NOT NULL,
    started_at     TEXT    NOT NULL,
    finished_at    TEXT    NOT NULL,
    total_ms       INTEGER NOT NULL,
    question_count INTEGER NOT NULL,
    correct_count  INTEGER NOT NULL
);
CREATE INDEX idx_round_project_user  ON practice_round(project_id, user_id);
CREATE INDEX idx_round_user_finished ON practice_round(user_id, finished_at);

-- 判分双记录（D16）：server_is_correct 权威，client_is_correct 仅用于即时反馈与分歧告警。
-- 题面与答案以快照落库，使作者事后改规则不影响历史（规格 §6.1(3)）。
CREATE TABLE attempt (
    id                INTEGER PRIMARY KEY AUTOINCREMENT,
    round_id          INTEGER NOT NULL REFERENCES practice_round(id) ON DELETE CASCADE,
    idx               INTEGER NOT NULL,
    q_snapshot        TEXT    NOT NULL,
    a_snapshot        TEXT    NOT NULL,
    a_envelope_json   TEXT    NOT NULL,
    user_input        TEXT    NOT NULL,
    client_is_correct INTEGER NOT NULL,
    server_is_correct INTEGER NOT NULL,
    elapsed_ms        INTEGER NOT NULL,
    UNIQUE (round_id, idx)
);
