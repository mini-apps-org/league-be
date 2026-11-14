package permissions

import "time"

type Code string

type Permission struct {
	ID          string    `db:"id"`
	Code        Code      `db:"code"`
	Name        string    `db:"name"`
	Description *string   `db:"description"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}
