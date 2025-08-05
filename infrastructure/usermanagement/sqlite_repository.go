package usermanagement

import (
	"database/sql"
	"fmt"
	"time"

	domainUserManagement "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/usermanagement"
	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/crypto/bcrypt"
)

type repository struct {
	db *sqlx.DB
}

func NewUserManagementRepository(dbPath string) (domainUserManagement.IUserManagementRepository, error) {
	db, err := sqlx.Connect("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to user management database: %w", err)
	}

	repo := &repository{db: db}
	if err := repo.migrate(); err != nil {
		return nil, fmt.Errorf("failed to migrate user management database: %w", err)
	}

	return repo, nil
}

func (r *repository) migrate() error {
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password TEXT NOT NULL,
		is_active BOOLEAN DEFAULT TRUE,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_users_username ON users(username);
	CREATE INDEX IF NOT EXISTS idx_users_is_active ON users(is_active);
	`

	_, err := r.db.Exec(query)
	return err
}

func (r *repository) Create(user *domainUserManagement.User) error {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	query := `
		INSERT INTO users (username, password, is_active, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
	`

	now := time.Now()
	result, err := r.db.Exec(query, user.Username, string(hashedPassword), user.IsActive, now, now)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return fmt.Errorf("failed to get last insert id: %w", err)
	}

	user.ID = int(id)
	user.CreatedAt = now
	user.UpdatedAt = now
	return nil
}

func (r *repository) GetByID(id int) (*domainUserManagement.User, error) {
	var user domainUserManagement.User
	query := "SELECT id, username, password, is_active, created_at, updated_at FROM users WHERE id = ?"

	err := r.db.Get(&user, query, id)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by id: %w", err)
	}

	return &user, nil
}

func (r *repository) GetByUsername(username string) (*domainUserManagement.User, error) {
	var user domainUserManagement.User
	query := "SELECT id, username, password, is_active, created_at, updated_at FROM users WHERE username = ?"

	err := r.db.Get(&user, query, username)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user by username: %w", err)
	}

	return &user, nil
}

func (r *repository) GetAll() ([]domainUserManagement.User, error) {
	var users []domainUserManagement.User
	query := "SELECT id, username, password, is_active, created_at, updated_at FROM users ORDER BY created_at DESC"

	err := r.db.Select(&users, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get all users: %w", err)
	}

	return users, nil
}

func (r *repository) Update(id int, updateReq *domainUserManagement.UpdateUserRequest) error {
	setParts := []string{}
	args := []interface{}{}

	if updateReq.Username != "" {
		setParts = append(setParts, "username = ?")
		args = append(args, updateReq.Username)
	}

	if updateReq.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(updateReq.Password), bcrypt.DefaultCost)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}
		setParts = append(setParts, "password = ?")
		args = append(args, string(hashedPassword))
	}

	if updateReq.IsActive != nil {
		setParts = append(setParts, "is_active = ?")
		args = append(args, *updateReq.IsActive)
	}

	if len(setParts) == 0 {
		return fmt.Errorf("no fields to update")
	}

	setParts = append(setParts, "updated_at = ?")
	args = append(args, time.Now())
	args = append(args, id)

	query := fmt.Sprintf("UPDATE users SET %s WHERE id = ?",
		fmt.Sprintf("%s", setParts[0]))
	for i := 1; i < len(setParts); i++ {
		query = fmt.Sprintf("%s, %s", query, setParts[i])
	}

	_, err := r.db.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}

	return nil
}

func (r *repository) Delete(id int) error {
	query := "DELETE FROM users WHERE id = ?"
	_, err := r.db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}

func (r *repository) GetActiveUsers() ([]domainUserManagement.User, error) {
	var users []domainUserManagement.User
	query := "SELECT id, username, password, is_active, created_at, updated_at FROM users WHERE is_active = TRUE"

	err := r.db.Select(&users, query)
	if err != nil {
		return nil, fmt.Errorf("failed to get active users: %w", err)
	}

	return users, nil
}

func (r *repository) ValidateCredentials(username, password string) bool {
	user, err := r.GetByUsername(username)
	if err != nil || user == nil || !user.IsActive {
		return false
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(password))
	return err == nil
}
