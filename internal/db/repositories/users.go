package repositories

import (
	"database/sql"
	"station/pkg/models"
)

type UserRepo struct {
	db *sql.DB
}

func NewUserRepo(db *sql.DB) *UserRepo {
	return &UserRepo{db: db}
}

func (r *UserRepo) Create(username string, publicKey string, isAdmin bool, apiKey *string) (*models.User, error) {
	query := `INSERT INTO users (username, public_key, is_admin, api_key) VALUES (?, ?, ?, ?) RETURNING id, username, public_key, is_admin, api_key, created_at, updated_at`
	
	var user models.User
	err := r.db.QueryRow(query, username, publicKey, isAdmin, apiKey).Scan(
		&user.ID, &user.Username, &user.PublicKey, &user.IsAdmin, &user.APIKey, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	
	return &user, nil
}

func (r *UserRepo) GetByID(id int64) (*models.User, error) {
	query := `SELECT id, username, public_key, is_admin, api_key, created_at, updated_at FROM users WHERE id = ?`
	
	var user models.User
	err := r.db.QueryRow(query, id).Scan(
		&user.ID, &user.Username, &user.PublicKey, &user.IsAdmin, &user.APIKey, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	
	return &user, nil
}

func (r *UserRepo) GetByUsername(username string) (*models.User, error) {
	query := `SELECT id, username, public_key, is_admin, api_key, created_at, updated_at FROM users WHERE username = ?`
	
	var user models.User
	err := r.db.QueryRow(query, username).Scan(
		&user.ID, &user.Username, &user.PublicKey, &user.IsAdmin, &user.APIKey, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	
	return &user, nil
}

func (r *UserRepo) GetByAPIKey(apiKey string) (*models.User, error) {
	query := `SELECT id, username, public_key, is_admin, api_key, created_at, updated_at FROM users WHERE api_key = ?`
	
	var user models.User
	err := r.db.QueryRow(query, apiKey).Scan(
		&user.ID, &user.Username, &user.PublicKey, &user.IsAdmin, &user.APIKey, &user.CreatedAt, &user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	
	return &user, nil
}

func (r *UserRepo) List() ([]*models.User, error) {
	query := `SELECT id, username, public_key, is_admin, api_key, created_at, updated_at FROM users ORDER BY username`
	
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var users []*models.User
	for rows.Next() {
		var user models.User
		err := rows.Scan(&user.ID, &user.Username, &user.PublicKey, &user.IsAdmin, &user.APIKey, &user.CreatedAt, &user.UpdatedAt)
		if err != nil {
			return nil, err
		}
		users = append(users, &user)
	}
	
	return users, rows.Err()
}

func (r *UserRepo) Update(id int64, username string, isAdmin bool) error {
	query := `UPDATE users SET username = ?, is_admin = ? WHERE id = ?`
	_, err := r.db.Exec(query, username, isAdmin, id)
	return err
}

func (r *UserRepo) UpdateAPIKey(id int64, apiKey *string) error {
	query := `UPDATE users SET api_key = ? WHERE id = ?`
	_, err := r.db.Exec(query, apiKey, id)
	return err
}

func (r *UserRepo) Delete(id int64) error {
	query := `DELETE FROM users WHERE id = ?`
	_, err := r.db.Exec(query, id)
	return err
}