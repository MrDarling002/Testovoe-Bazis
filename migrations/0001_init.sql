CREATE TABLE IF NOT EXISTS users (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    email VARCHAR(255) NOT NULL,
    username VARCHAR(100) NOT NULL,
    password_hash CHAR(60) NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (id),
    UNIQUE KEY uq_users_email (email),
    UNIQUE KEY uq_users_username (username)
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS teams (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    name VARCHAR(120) NOT NULL,
    description TEXT NULL,
    created_by BIGINT UNSIGNED NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (id),
    KEY idx_teams_created_by (created_by),

    CONSTRAINT fk_teams_created_by
        FOREIGN KEY (created_by) REFERENCES users (id)
        ON DELETE RESTRICT
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS team_members (
    team_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    role ENUM('owner', 'admin', 'member') NOT NULL DEFAULT 'member',
    invited_by BIGINT UNSIGNED NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (team_id, user_id),

    KEY idx_team_members_user_id (user_id),
    KEY idx_team_members_invited_by (invited_by),

    CONSTRAINT fk_team_members_team
        FOREIGN KEY (team_id) REFERENCES teams (id)
        ON DELETE CASCADE,

    CONSTRAINT fk_team_members_user
        FOREIGN KEY (user_id) REFERENCES users (id)
        ON DELETE CASCADE,

    CONSTRAINT fk_team_members_invited_by
        FOREIGN KEY (invited_by) REFERENCES users (id)
        ON DELETE SET NULL
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS tasks (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    team_id BIGINT UNSIGNED NOT NULL,
    title VARCHAR(200) NOT NULL,
    description TEXT NULL,
    status ENUM('todo', 'in_progress', 'done') NOT NULL DEFAULT 'todo',
    assignee_id BIGINT UNSIGNED NULL,
    created_by BIGINT UNSIGNED NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    completed_at DATETIME NULL,

    PRIMARY KEY (id),

    KEY idx_tasks_team_status_assignee (team_id, status, assignee_id),

    KEY idx_tasks_team_status_completed (team_id, status, completed_at),

    KEY idx_tasks_team_created_by_created_at (team_id, created_by, created_at),

    KEY idx_tasks_assignee_id (assignee_id),

    CONSTRAINT fk_tasks_team
        FOREIGN KEY (team_id) REFERENCES teams (id)
        ON DELETE CASCADE,

    CONSTRAINT fk_tasks_assignee
        FOREIGN KEY (assignee_id) REFERENCES users (id)
        ON DELETE SET NULL,

    CONSTRAINT fk_tasks_created_by
        FOREIGN KEY (created_by) REFERENCES users (id)
        ON DELETE RESTRICT
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS task_history (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    task_id BIGINT UNSIGNED NOT NULL,
    changed_by BIGINT UNSIGNED NOT NULL,
    action ENUM('created', 'updated') NOT NULL,
    field VARCHAR(50) NULL,
    old_value VARCHAR(500) NULL,
    new_value VARCHAR(500) NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (id),

    KEY idx_task_history_task_created (task_id, created_at),

    KEY idx_task_history_changed_by (changed_by),

    KEY idx_task_history_field_new_value (field, new_value),

    CONSTRAINT fk_task_history_task
        FOREIGN KEY (task_id) REFERENCES tasks (id)
        ON DELETE CASCADE,

    CONSTRAINT fk_task_history_changed_by
        FOREIGN KEY (changed_by) REFERENCES users (id)
        ON DELETE RESTRICT
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;

CREATE TABLE IF NOT EXISTS task_comments (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    task_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    body TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (id),

    KEY idx_task_comments_task_created (task_id, created_at),
    KEY idx_task_comments_user (user_id),

    CONSTRAINT fk_task_comments_task
        FOREIGN KEY (task_id) REFERENCES tasks (id)
        ON DELETE CASCADE,

    CONSTRAINT fk_task_comments_user
        FOREIGN KEY (user_id) REFERENCES users (id)
        ON DELETE CASCADE
) ENGINE = InnoDB DEFAULT CHARSET = utf8mb4 COLLATE = utf8mb4_unicode_ci;