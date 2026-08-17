package mysql_test

import (
	"context"
	"database/sql"
	"testing"

	_ "github.com/go-sql-driver/mysql"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	mysqlcontainer "github.com/testcontainers/testcontainers-go/modules/mysql"
)

// setupTestDB запускает MySQL контейнер с полной актуальной схемой БД.
func setupTestDB(ctx context.Context, t *testing.T) (*sql.DB, func()) {
	t.Helper()

	container, err := mysqlcontainer.RunContainer(ctx,
		testcontainers.WithImage("mysql:8.0"),
		mysqlcontainer.WithDatabase("test_db"),
		mysqlcontainer.WithUsername("test_user"),
		mysqlcontainer.WithPassword("test_password"),
	)
	require.NoError(t, err, "failed to start mysql container")

	connStr, err := container.ConnectionString(ctx, "parseTime=true&multiStatements=true&loc=UTC")
	require.NoError(t, err, "failed to get connection string")

	db, err := sql.Open("mysql", connStr)
	require.NoError(t, err, "failed to open db connection")
	require.NoError(t, db.PingContext(ctx), "failed to ping db")

	// Полная схема БД на основе миграции 20260813162630_init_schema.sql
	schemaSQL := `
	CREATE TABLE IF NOT EXISTS users (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		email VARCHAR(255) NOT NULL UNIQUE,
		password_hash VARCHAR(255) NOT NULL,
		name VARCHAR(100) NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS teams (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		name VARCHAR(100) NOT NULL,
		created_by BIGINT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS team_members (
		team_id BIGINT NOT NULL,
		user_id BIGINT NOT NULL,
		role ENUM('owner', 'admin', 'member') NOT NULL DEFAULT 'member',
		PRIMARY KEY (team_id, user_id),
		FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS tasks (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		team_id BIGINT NOT NULL,
		title VARCHAR(255) NOT NULL,
		description TEXT NULL,
		status ENUM('todo', 'in_progress', 'done') NOT NULL DEFAULT 'todo',
		priority ENUM('low', 'medium', 'high') NOT NULL DEFAULT 'medium',
		created_by BIGINT NOT NULL,
		assignee_id BIGINT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
		closed_at DATETIME NULL DEFAULT NULL,
		version INT NOT NULL DEFAULT 1,
		FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE,
		FOREIGN KEY (created_by) REFERENCES users(id) ON DELETE CASCADE,
		FOREIGN KEY (assignee_id) REFERENCES users(id) ON DELETE SET NULL
	);

	CREATE TABLE IF NOT EXISTS task_comments (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		task_id BIGINT NOT NULL,
		user_id BIGINT NOT NULL,
		content TEXT NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS task_history (
		id BIGINT AUTO_INCREMENT PRIMARY KEY,
		task_id BIGINT NOT NULL,
		changed_by BIGINT NOT NULL,
		changes JSON NOT NULL,
		created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE,
		FOREIGN KEY (changed_by) REFERENCES users(id) ON DELETE CASCADE
	);
	`
	_, err = db.ExecContext(ctx, schemaSQL)
	require.NoError(t, err, "failed to apply schema migrations")

	cleanup := func() {
		_ = db.Close()
		_ = container.Terminate(ctx)
	}

	return db, cleanup
}

// Вспомогательные функции создания сущностей для тестов

func createTestUser(ctx context.Context, t *testing.T, db *sql.DB, email, name string) int64 {
	t.Helper()
	res, err := db.ExecContext(ctx,
		"INSERT INTO users (email, password_hash, name) VALUES (?, 'hash', ?)",
		email, name,
	)
	require.NoError(t, err, "failed to create test user")

	id, err := res.LastInsertId()
	require.NoError(t, err)
	return id
}

func createTestTeam(ctx context.Context, t *testing.T, db *sql.DB, ownerID int64, name string) int64 {
	t.Helper()
	res, err := db.ExecContext(ctx,
		"INSERT INTO teams (name, created_by) VALUES (?, ?)",
		name, ownerID,
	)
	require.NoError(t, err, "failed to create test team")

	id, err := res.LastInsertId()
	require.NoError(t, err)
	return id
}

func createTestTask(ctx context.Context, t *testing.T, db *sql.DB, teamID, creatorID int64, title string) int64 {
	t.Helper()
	res, err := db.ExecContext(ctx,
		"INSERT INTO tasks (team_id, title, created_by) VALUES (?, ?, ?)",
		teamID, title, creatorID,
	)
	require.NoError(t, err, "failed to create test task")

	id, err := res.LastInsertId()
	require.NoError(t, err)
	return id
}
