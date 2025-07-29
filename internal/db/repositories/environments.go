package repositories

import (
	"database/sql"
	"station/pkg/models"
)

type EnvironmentRepo struct {
	db *sql.DB
}

func NewEnvironmentRepo(db *sql.DB) *EnvironmentRepo {
	return &EnvironmentRepo{db: db}
}

func (r *EnvironmentRepo) Create(name string, description *string) (*models.Environment, error) {
	query := `INSERT INTO environments (name, description) VALUES (?, ?) RETURNING id, name, description, created_at, updated_at`
	
	var env models.Environment
	err := r.db.QueryRow(query, name, description).Scan(
		&env.ID, &env.Name, &env.Description, &env.CreatedAt, &env.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	
	return &env, nil
}

func (r *EnvironmentRepo) GetByID(id int64) (*models.Environment, error) {
	query := `SELECT id, name, description, created_at, updated_at FROM environments WHERE id = ?`
	
	var env models.Environment
	err := r.db.QueryRow(query, id).Scan(
		&env.ID, &env.Name, &env.Description, &env.CreatedAt, &env.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	
	return &env, nil
}

func (r *EnvironmentRepo) GetByName(name string) (*models.Environment, error) {
	query := `SELECT id, name, description, created_at, updated_at FROM environments WHERE name = ?`
	
	var env models.Environment
	err := r.db.QueryRow(query, name).Scan(
		&env.ID, &env.Name, &env.Description, &env.CreatedAt, &env.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	
	return &env, nil
}

func (r *EnvironmentRepo) List() ([]*models.Environment, error) {
	query := `SELECT id, name, description, created_at, updated_at FROM environments ORDER BY name`
	
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var environments []*models.Environment
	for rows.Next() {
		var env models.Environment
		err := rows.Scan(&env.ID, &env.Name, &env.Description, &env.CreatedAt, &env.UpdatedAt)
		if err != nil {
			return nil, err
		}
		environments = append(environments, &env)
	}
	
	return environments, rows.Err()
}

func (r *EnvironmentRepo) Update(id int64, name string, description *string) error {
	query := `UPDATE environments SET name = ?, description = ? WHERE id = ?`
	_, err := r.db.Exec(query, name, description, id)
	return err
}

func (r *EnvironmentRepo) Delete(id int64) error {
	query := `DELETE FROM environments WHERE id = ?`
	_, err := r.db.Exec(query, id)
	return err
}