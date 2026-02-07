package model

import (
	"context"
	"fmt"

	"github.com/uptrace/bun"
)

type User struct {
	ID       string `bun:",pk,autoincrement,notnull"`
	Name     string `bun:",notnull"`
	Email    string `bun:",notnull"`
	Password string `bun:",notnull"`
	PFP      string `bun:",notnull"`
}

func ModelUser(db *bun.DB) error {
	ctx := context.Background()
	_, err := db.NewCreateTable().
		Model((*User)(nil)).
		IfNotExists().
		Exec(ctx)

	return err
}

func GetUserByID(id int, db *bun.DB) (*User, error) {
	var userModel User
	ctx := context.Background()
	err := db.NewSelect().
		Model(&userModel).
		Where("id = ?", id).
		Scan(ctx)

	if err != nil {
		fmt.Println("Error querying user:", err)
		return nil, err
	}

	fmt.Printf("User: %+v\n", userModel)

	return &userModel, nil
}

func GetUserByEmail(email string, db *bun.DB) (*User, error) {
	var userModel User
	ctx := context.Background()

	err := db.NewSelect().
		Model(&userModel).
		Where("email = ?", email).
		Scan(ctx)

	if err != nil {
		fmt.Println("Error querying user:", err)
		return nil, err
	}

	fmt.Printf("User: %+v\n", userModel)

	return &userModel, nil
}
