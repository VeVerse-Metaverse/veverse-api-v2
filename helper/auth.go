package helper

import (
	"context"
	"errors"
	"github.com/dgrijalva/jwt-go"
	"golang.org/x/crypto/bcrypt"
	"strconv"
	"time"
	"veverse-api/database"
	"veverse-api/model"
)

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func LoginInternal(ctx context.Context, email, pass string) (token string, err error) {
	if email == "" || pass == "" {
		return "", errors.New("no credentials")
	}

	db := database.DB

	// Try to find a user by email
	q := `SELECT u.id, u.hash, u.is_admin FROM users u WHERE u.email = $1`

	row := db.QueryRow(ctx, q, email)

	var user model.User
	err = row.Scan(&user.Id, &user.PasswordHash, &user.IsAdmin)
	if err != nil {
		return "", errors.New("no user")
	}

	if user.PasswordHash == nil {
		return "", errors.New("no password")
	}

	if !CheckPasswordHash(pass, *user.PasswordHash) {
		return "", errors.New("invalid password")
	}

	t := jwt.New(jwt.SigningMethodHS256)

	claims := t.Claims.(jwt.MapClaims)
	claims["id"] = user.Id
	claims["is_admin"] = user.IsAdmin

	var exp int
	exp, err = strconv.Atoi(model.AUTH_EXPIRATION)
	if err != nil {
		exp = 24 * 30 // Default to 30 days
	}
	claims["exp"] = time.Now().Add(time.Duration(exp) * time.Hour).Unix()

	token, err = t.SignedString([]byte(model.AUTH_SECRET))
	if err != nil {
		return "", errors.New("something went wrong")
	}

	return token, nil
}
