package model

import (
	"bytes"
	sm "dev.hackerman.me/artheon/veverse-shared/model"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofrs/uuid"
	"github.com/jackc/pgx/v5"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
	"veverse-api/aws/ses"
	"veverse-api/database"
	"veverse-api/reflect"
)

const (
	buildJobSingular = "buildJob"
)

type BuildJob struct {
	Identifier
	UserId        *uuid.UUID `json:"userId,omitempty"`
	WorkerId      *uuid.UUID `json:"workerId,omitempty"`
	PackageId     *uuid.UUID `json:"modId,omitempty"`
	Configuration string     `json:"configuration,omitempty"`
	Status        string     `json:"status,omitempty"`
	Map           string     `json:"map,omitempty"`
	Platform      string     `json:"platform,omitempty"`
	Server        bool       `json:"server,omitempty"`
	ReleaseName   string     `json:"releaseName,omitempty"`
	Version       int64      `json:"variant,omitempty"`
	CreatedAt     time.Time  `json:"createdAt,omitempty"`
}

type BuildJobMetadata struct {
	UserId        *uuid.UUID `json:"userId,omitempty"`
	WorkerId      *uuid.UUID `json:"workerId,omitempty"`
	PackageId     uuid.UUID  `json:"modId,omitempty"`
	Configuration string     `json:"configuration,omitempty"`
	Status        string     `json:"status,omitempty"`
	Map           string     `json:"map,omitempty"`
	Platform      string     `json:"platform,omitempty"`
	Server        bool       `json:"server,omitempty"`
	TargetRelease string     `json:"releaseName,omitempty"`
	Version       int64      `json:"variant,omitempty"`
}

type ScheduleRequestMetadata struct {
	Identifier
}

func ScheduleBuildJobsForPackage(c *fiber.Ctx, requester *sm.User, entityId uuid.UUID) (err error) {
	db := database.DB
	q := "SELECT DISTINCT s.map FROM spaces s LEFT JOIN mods m ON m.id = s.mod_id WHERE m.id = $1"
	rows, err := db.Query(c.UserContext(), q, entityId)
	if err != nil {
		return fmt.Errorf("failed to query %s @ %s: %v", buildJobSingular, reflect.FunctionName(), err)
	}

	defer func() {
		rows.Close()
		database.LogPgxStat("ScheduleBuildJobsForPackage")
	}()

	var maps []string
	for rows.Next() {
		var m string
		err = rows.Scan(&m)
		if err != nil {
			return err
		}
		maps = append(maps, m)
	}

	// Assign configuration matching the release type
	configuration := "Development"
	env := os.Getenv("ENVIRONMENT")
	if env == "test" {
		configuration = "Shipping"
	} else if env == "prod" {
		configuration = "Shipping"
	}

	job := BuildJobMetadata{
		UserId:        &requester.Id,
		WorkerId:      nil, // Assigned later
		PackageId:     entityId,
		Configuration: configuration,
		Status:        "pending",
		Map:           strings.Join(maps, "+"),
		Platform:      "Linux",
		Server:        true,
		TargetRelease: "1.0.0",
		Version:       0,
	}

	// Add linux server job
	if err = AddPendingBuildJob(c, job); err != nil {
		return err
	}

	// Add client jobs
	job.Server = false
	clientPlatforms := []string{"Win64", "Linux", "Mac", "IOS", "Android"}
	for _, p := range clientPlatforms {
		job.Platform = p
		if err = AddPendingBuildJob(c, job); err != nil {
			return err
		}
	}

	q = "SELECT m.name FROM mods m WHERE m.id = $1"
	row := db.QueryRow(c.UserContext(), q, entityId /*$1*/)
	var packageName string
	err = row.Scan(&packageName)
	if err != nil {
		return fmt.Errorf("failed to get a package name: %s", err.Error())
	}

	q = "SELECT name, email, allow_emails FROM users u WHERE u.id = $1"
	row = db.QueryRow(c.UserContext(), q, requester.Id /*$1*/)
	var (
		name        string
		email       string
		allowEmails bool
	)
	err = row.Scan(&name, &email, &allowEmails)
	if err != nil {
		return fmt.Errorf("failed to get a requester email: %s", err.Error())
	}

	if allowEmails && email != "" {
		htmlTemplate := fmt.Sprintf(`<!DOCTYPE HTML PUBLIC "-//W3C//DTD XHTML 1.0 Transitional //EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:v="urn:schemas-microsoft-com:vml" xmlns:o="urn:schemas-microsoft-com:office:office">
<head></head><body>Your VeVerse package %s has been scheduled for processing</body></html>`, packageName)
		if err = ses.Send("VeVerse SDK - Package Processing", fmt.Sprintf("Your VeVerse package %s has been scheduled for processing", packageName), htmlTemplate, []string{email}, []string{}, []string{}, "builder@veverse.com"); err != nil {
			return fmt.Errorf("failed to send build job scheduled email: %s", err.Error())
		}
	}

	res, err := http.Post("https://discord.com/api/webhooks/xxxxxxxxxxxxxxxxxxx/xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", "text/plain", bytes.NewBuffer([]byte(fmt.Sprintf("VeVerse package %s has been scheduled for processing by %s", packageName, name))))
	if err != nil {
		fmt.Printf("failed to trigger discord hook: %s\n", err.Error())
	}

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Printf("server: could not read response body: %s\n", err)
	}
	fmt.Printf("discord hook: response body: %s\n", resBody)

	return nil
}

func NotifyBuildJobCompleted(c *fiber.Ctx, entityId uuid.UUID) (err error) {
	db := database.DB

	q := "SELECT m.name, u.name, u.email, u.allow_emails FROM mods m LEFT JOIN entities e on m.id = e.id LEFT JOIN accessibles a ON e.id = a.entity_id LEFT JOIN users u ON a.user_id = u.id AND a.is_owner WHERE m.id = $1"
	row := db.QueryRow(c.UserContext(), q, entityId /*$1*/)
	var (
		packageName string
		name        string
		email       string
		allowEmails bool
	)
	err = row.Scan(&packageName, &name, &email, &allowEmails)
	if err != nil {
		return fmt.Errorf("failed to get a package name: %s", err.Error())
	}

	if allowEmails && email != "" {
		htmlTemplate := fmt.Sprintf(`<!DOCTYPE HTML PUBLIC "-//W3C//DTD XHTML 1.0 Transitional //EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:v="urn:schemas-microsoft-com:vml" xmlns:o="urn:schemas-microsoft-com:office:office">
<head></head><body>Your VeVerse package %s processing has been completed</body></html>`, packageName)
		if err = ses.Send("VeVerse SDK - Package Processing", fmt.Sprintf("Your VeVerse package %s processing has been completed", packageName), htmlTemplate, []string{email}, []string{}, []string{}, "builder@veverse.com"); err != nil {
			return fmt.Errorf("failed to send build job completed email: %s", err.Error())
		}
	}

	res, err := http.Post("https://discord.com/api/webhooks/xxxxxxxxxxxxxxxxxxx/xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx", "text/plain", bytes.NewBuffer([]byte(fmt.Sprintf("VeVerse package %s uploaded by %s has been completed", packageName, name))))
	if err != nil {
		fmt.Printf("failed to trigger discord hook: %s\n", err.Error())
	}

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		fmt.Printf("server: could not read response body: %s\n", err)
	}
	fmt.Printf("discord hook: response body: %s\n", resBody)

	return err
}

func AddPendingBuildJob(c *fiber.Ctx, metadata BuildJobMetadata) (err error) {
	db := database.DB
	ctx := c.UserContext()

	if metadata.UserId == nil || metadata.UserId.IsNil() {
		return fmt.Errorf("no build package id")
	}

	if metadata.PackageId.IsNil() {
		return fmt.Errorf("no build package id")
	}

	if metadata.Configuration == "" {
		metadata.Configuration = "Shipping"
	}

	if metadata.Platform == "" {
		return fmt.Errorf("no build package platform")
	}

	if metadata.Map == "" {
		return fmt.Errorf("no build package maps")
	}

	if metadata.TargetRelease == "" {
		return fmt.Errorf("no build package target release")
	}

	var (
		q  string // query string
		tx pgx.Tx // pgx transaction
	)

	tx, err = db.Begin(ctx)
	if err != nil {
		return err
	}

	q = `INSERT INTO build_jobs AS j (id, mod_id, status, user_id, worker_id, configuration, platform, server, map, release_name, version, created_at)
VALUES (gen_random_uuid(), $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, now())`
	_, err = tx.Exec(ctx, q, metadata.PackageId, "pending", metadata.UserId, nil, metadata.Configuration, metadata.Platform, metadata.Server, metadata.Map, metadata.TargetRelease, metadata.Version)
	if err != nil {
		_ = tx.Rollback(ctx)
		return err
	}

	return tx.Commit(ctx)
}
