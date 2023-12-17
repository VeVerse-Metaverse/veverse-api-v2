package handler

import (
	sc "dev.hackerman.me/artheon/veverse-shared/context"
	st "dev.hackerman.me/artheon/veverse-shared/telegram"
	"fmt"
	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/exp/rand"
	"strconv"
	"strings"
	"time"
	"veverse-api/database"
	"veverse-api/helper"
	"veverse-api/model"
	"veverse-api/validation"

	"github.com/dgrijalva/jwt-go"
	"github.com/gofiber/fiber/v2"
)

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

type LoginInput struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginWeb3Input struct {
	Address   string `json:"address"`
	Signature string `json:"signature"`
	Message   string `json:"message"`
	Timestamp int64  `json:"timestamp"`
}

type RestoreTokenInput struct {
	Token string `json:"token" validate:"required"`
}

type RestorePasswordInput struct {
	RestoreTokenInput
	Password       string `json:"password" validate:"required,gte=6,hasUpper,hasLower,hasNumber,containsany=!@#$%^&*()+=<>~"`
	RepeatPassword string `json:"repeatPassword" validate:"required,eqfield=Password"`
}

type RestoreFormInput struct {
	Email string `json:"email" validate:"required,email"`
}

// Login godoc
// @Summary      Login
// @Description  Login using email and password.
// @Tags         accounts
// @Accept       json
// @Produce      json
// @Param        request body handler.LoginInput true "Request JSON"
// @Success      200  {object}  model.User
// @Failure      400  {object}  error
// @Failure      404  {object}  error
// @Failure      500  {object}  error
// @Router       /auth/login [post]
func Login(c *fiber.Ctx) error {
	var input LoginInput
	if err := c.BodyParser(&input); err != nil {
		if err1 := st.SendTelegramMessage("failed to parse body"); err1 != nil {
			logrus.Errorf("failed to send telegram message: %v", err1)
		}

		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "no credentials", "data": nil})
	}

	logrus.Infof(">>> Logging in: email: %s", input.Email)

	var err error

	email := input.Email
	pass := input.Password

	if len(email) == 0 || len(pass) == 0 {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "no credentials", "data": nil})
	}

	if !strings.Contains(email, "@") {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "invalid email", "data": nil})
	}

	if len(pass) < 6 {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "password too short", "data": nil})
	}

	db, ok := c.UserContext().Value(sc.Database).(*pgxpool.Pool)
	if !ok || db == nil {
		logrus.Errorf("failed to get database from context")

		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "something went wrong", "data": nil})
	}

	logrus.Infof(">>> Logging in: database %v", db)

	// Try to find a user by email
	q := `SELECT u.id, u.hash, u.is_admin FROM users u WHERE u.email = $1`

	row := db.QueryRow(c.UserContext(), q, email)

	logrus.Infof(">>> Logging in: query: %s", q)

	var user model.User
	err = row.Scan(&user.Id, &user.PasswordHash, &user.IsAdmin)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "no user", "data": nil})
	}

	logrus.Infof(">>> Logging in: user %s", user.Id)

	var hash string
	hash, err = HashPassword(pass)

	logrus.Infof(">>> Logging in: hash %v", hash)

	if err == nil {
		logrus.Infof(">>> Logging in: patch")

		// todo: Considered a hack, patches existing users with the password hash. Instead of that we should make correct user registration and password reset functions.
		if nil == user.PasswordHash {
			row = db.QueryRow(c.UserContext(), `UPDATE users u SET hash = $1 WHERE u.id = $2`, hash, user.Id)
			if err = row.Scan(); err != nil {
				if err.Error() != "no rows in result set" {
					fmt.Printf(err.Error())
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "something went wrong", "data": nil})
				}
			}
			user.PasswordHash = &hash
		}
	}

	if user.PasswordHash == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "no password", "data": nil})
	}

	logrus.Infof(">>> Logging in: check hash")

	if !CheckPasswordHash(pass, *user.PasswordHash) {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "invalid password", "data": nil})
	}

	logrus.Infof(">>> Logging in: hash ok")

	token := jwt.New(jwt.SigningMethodHS256)

	claims := token.Claims.(jwt.MapClaims)
	claims["id"] = user.Id
	claims["is_admin"] = user.IsAdmin

	var exp int
	exp, err = strconv.Atoi(model.AUTH_EXPIRATION)
	if err != nil {
		exp = 7 * 24 // Default to 7 days
	}
	claims["exp"] = time.Now().Add(time.Duration(exp) * time.Hour).Unix()

	var t string
	t, err = token.SignedString([]byte(model.AUTH_SECRET))
	if err != nil {
		return c.SendStatus(fiber.StatusInternalServerError)
	}

	return c.JSON(fiber.Map{"status": "ok", "message": "ok", "data": t})
}

// LoginWeb3 godoc
// @Summary      LoginWeb3
// @Description  Login using web3.
// @Tags         accounts
// @Accept       json
// @Produce      json
// @Param        request body handler.LoginWeb3Input true "Request JSON"
// @Success      200  {object}  model.User
// @Failure      400  {object}  error
// @Failure      404  {object}  error
// @Failure      500  {object}  error
// @Router       /auth/login/web3 [post]
func LoginWeb3(c *fiber.Ctx) error {
	var web3Input LoginWeb3Input
	if err := c.BodyParser(&web3Input); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "error", "message": "no credentials", "data": nil})
	}

	err, nonce, user := model.GetUserNonce(c.UserContext(), web3Input.Address)
	if err != nil && err.Error() != "no rows in result set" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "", "data": nil})
	}

	i := strings.Index(web3Input.Message, "Nonce:")

	var nonceFromMsg int
	if i != -1 {
		i += 7
		nonceFromMsg, _ = strconv.Atoi(web3Input.Message[i:])
	}

	if web3Input.Timestamp < (time.Now().UnixMilli() - time.Minute.Milliseconds()*10) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "unverified signature", "data": nil})
	}

	if nonce != nil && nonceFromMsg != *nonce {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "unverified signature", "data": nil})
	}

	correct := verifySignature(web3Input.Address, web3Input.Signature, []byte(web3Input.Message))

	if correct {
		if user.Id != nil {
			db := database.DB
			helper.LogPgxStat("login before")
			row := db.QueryRow(
				c.UserContext(),
				`UPDATE users u SET nonce = $1 WHERE u.eth_address = $2`,
				10000+rand.Intn(40000-10000),
				web3Input.Address,
			)

			if err = row.Scan(); err != nil {
				if err.Error() != "no rows in result set" {
					fmt.Printf(err.Error())
					return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "something went wrong", "data": nil})
				}
			}

			token := jwt.New(jwt.SigningMethodHS256)

			claims := token.Claims.(jwt.MapClaims)
			claims["id"] = user.Id
			claims["is_admin"] = user.IsAdmin
			exp, err1 := strconv.Atoi(model.AUTH_EXPIRATION)
			if err1 != nil {
				exp = 72 // Default to 3 days
			}
			claims["exp"] = time.Now().Add(time.Duration(exp) * time.Hour).Unix()

			t, err3 := token.SignedString([]byte(model.AUTH_SECRET))
			if err3 != nil {
				return c.SendStatus(fiber.StatusInternalServerError)
			}

			helper.LogPgxStat("login after")

			return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "data": fiber.Map{"verified": true, "token": t}})
		} else {
			return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "data": fiber.Map{"verified": true}})
		}
	} else {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"status": "ok", "message": "unverified signature", "data": fiber.Map{"verified": false}})
	}
}

func CheckRestoreToken(c *fiber.Ctx) error {

	var input RestoreTokenInput
	err := c.QueryParser(&input)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	err = validation.Validator.Struct(input)
	if err != nil {
		errors := model.GetErrors(err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "validation error", "data": errors})
	}

	var claims map[string]interface{}
	claims, err = helper.GetRequesterTokenData(input.Token)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "restore password token is invalid", "data": nil})
	}

	db := database.DB

	//Try to find a user by email
	q := `SELECT u.id FROM users u WHERE u.email = $1`
	row := db.QueryRow(c.UserContext(), q, claims["id"])

	var (
		hasActiveToken bool
		user           model.User
	)

	err = row.Scan(&user.Id)

	hasActiveToken, err = model.CheckRestoreToken(c.UserContext(), user.Id, 24)

	if !hasActiveToken {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "err", "message": "restore password token was expired", "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "data": fiber.Map{"expired": false}})
}

func RestorePassword(c *fiber.Ctx) error {

	var input RestorePasswordInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "restore password data error", "data": nil})
	}

	err := validation.Validator.Struct(input)
	if err != nil {
		errors := model.GetErrors(err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "validation error", "data": errors})
	}

	var claims map[string]interface{}
	claims, err = helper.GetRequesterTokenData(input.Token)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "invalid restore password token", "data": nil})
	}

	db := database.DB

	//Try to find a user by email
	q := `SELECT u.id FROM users u WHERE u.email = $1`
	row := db.QueryRow(c.UserContext(), q, claims["id"])

	var (
		hasActiveToken bool
		user           model.User
	)

	err = row.Scan(&user.Id)

	hasActiveToken, err = model.CheckRestoreToken(c.UserContext(), user.Id, 24)

	if !hasActiveToken {
		return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": "restore password token expired", "data": fiber.Map{"expired": true}})
	}

	var hash string
	hash, err = HashPassword(input.Password)
	if err == nil {
		row = db.QueryRow(c.UserContext(), `UPDATE users u SET hash = $1 WHERE u.id = $2`, hash, user.Id)
		if err = row.Scan(); err != nil {
			if err.Error() != "no rows in result set" {
				logrus.Errorf("restore password err: %v", err)
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "restore password error", "data": nil})
			}
		}

		user.PasswordHash = &hash

		err = model.SetRestoreTokensToInvalid(c.UserContext(), user)
		if err != nil || err.Error() != "no rows in result set" {
			logrus.Errorf("set restore tokens to invalid err: %v", err)
		}
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": "password has been successfully changed!", "data": fiber.Map{"restored": true}})
}

func SendRecoveryLink(c *fiber.Ctx) error {

	var input RestoreFormInput
	if err := c.BodyParser(&input); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "", "data": nil})
	}

	email := input.Email
	err := validation.Validator.Struct(input)
	if err != nil {
		errors := model.GetErrors(err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "login validation error", "data": errors})
	}

	db := database.DB

	// Try to find a user by email
	q := `SELECT u.id, u.email, u.allow_emails FROM users u WHERE u.email = $1`
	row := db.QueryRow(c.UserContext(), q, email)

	var user model.User
	err = row.Scan(&user.Id, &user.Email, &user.AllowEmails)
	if user.Email == nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "User account was not found", "data": nil})
	}

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no user", "data": nil})
	}

	var (
		hasActiveToken bool
	)

	hasActiveToken, err = model.CheckRestoreToken(c.UserContext(), user.Id, 24)

	if hasActiveToken {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "The restore link was sent earlier. Please check your email.",
			"data":    fiber.Map{"hasActiveToken": true}})
	}

	var token string
	token, err = generateNonAuthToken(user.Email, model.ACTIVATION_SECRET_KEY, 24)
	if err != nil {
		logrus.Errorf("generate recovery link err %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"status":  "error",
			"message": "generate recovery link err",
			"data":    nil,
		})
	}

	err = model.AddRestoreToken(c.UserContext(), user.Id, token)
	if err != nil {
		logrus.Errorf("generate recovery link err %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"status":  "error",
			"message": "generate recovery link err",
			"data":    nil,
		})
	}

	restoreLink := fmt.Sprintf("%s/#/auth/restore/%s", model.WEBAPP_ADDRESS, token)

	err = model.SendRestoreLinkEmail(&user, restoreLink)
	if err != nil {
		logrus.Errorf("failed to send restore password link email: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "failed to send restore password link email", "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": fmt.Sprintf("A reset email has been sent to %v", input.Email), "data": nil})
}

func verifySignature(from, sigHex string, expectedMsg []byte) bool {
	sig := hexutil.MustDecode(sigHex)

	expectedMsg = accounts.TextHash(expectedMsg)
	sig[crypto.RecoveryIDOffset] -= 27

	recovered, err := crypto.SigToPub(expectedMsg, sig)
	if err != nil {
		return false
	}

	recoveredAddr := crypto.PubkeyToAddress(*recovered)

	return from == recoveredAddr.Hex()

}

func generateNonAuthToken(email *string, secret string, exp int) (tokenString string, err error) {
	token := jwt.New(jwt.SigningMethodHS256)

	claims := token.Claims.(jwt.MapClaims)
	claims["id"] = email

	claims["exp"] = time.Now().Add(time.Duration(exp) * time.Hour).Unix()

	var t string
	t, err = token.SignedString([]byte(secret))
	if err != nil {
		return "", err
	}

	return t, nil
}
