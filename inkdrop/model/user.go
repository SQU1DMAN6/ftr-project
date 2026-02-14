package model

import (
	"context"
	"errors"
	"fmt"
	"regexp"

	"github.com/uptrace/bun"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID       int64  `bun:",pk,autoincrement"`
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

func CreateUser(db *bun.DB, name string, email string, password string) error {
	pass, err := regexp.MatchString("[^a-zA-Z0-9_-]+", name)
	if err != nil {
		return err
	}
	fmt.Println(pass)
	if pass != true {
		err = errors.New("Username contains special characters.")
		return nil
	}

	checkUser, err := GetUserByEmail(email, db)
	if checkUser != nil && err == nil {
		err = errors.New("User already exists")
		return err
	}

	checkUser, err = GetUserByName(name, db)
	if checkUser != nil && err == nil {
		err = errors.New("User already exists")
		return err
	}

	ctx := context.Background()
	hashedPassword, _ := HashPassword(password)
	user := &User{Name: name, Email: email, Password: hashedPassword, PFP: "/assets/default.png"}
	query, err := db.NewInsert().Model(user).Exec(ctx)
	if err != nil {
		fmt.Println("Error:", err)
		return err
	}
	fmt.Println("Database insert complete:", query)
	return nil
}

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 16)
	return string(bytes), err
}

func CheckPassword(db *bun.DB, email, plainTextPassword string) (*User, error) {
	user, err := GetUserByEmail(email, db)
	if err != nil {
		return nil, err
	}
	if err := bcrypt.CompareHashAndPassword(
		[]byte(user.Password),
		[]byte(plainTextPassword),
	); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return nil, err
		}
		return nil, err
	}
	return user, nil
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

func GetUserByName(name string, db *bun.DB) (*User, error) {
	var userModel User
	ctx := context.Background()

	err := db.NewSelect().
		Model(&userModel).
		Where("name = ?", name).
		Scan(ctx)

	if err != nil {
		fmt.Println("Error querying user", err)
		return nil, err
	}

	fmt.Printf("Uder %+v", userModel)

	return &userModel, nil
}
