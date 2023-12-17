package model

import (
	sm "dev.hackerman.me/artheon/veverse-shared/model"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofrs/uuid"
	"veverse-api/aws/ses"
)

type EmailMetadata struct {
	Subject string
	Html    string
	Text    string
	To      string
	Sender  string
}

var JobStatusMap = map[string]string{
	"unclaimed":  "Scheduled",  // Job is scheduled and not claimed by any worker
	"claimed":    "Processing", // Job is claimed by a worker
	"processing": "Processing", // Job is currently processed by a worker
	"uploading":  "Processing", // Job is currently uploading its results to the cloud storage
	"completed":  "Completed",  // Job has been completed successfully
	"error":      "Failed",     // Job failed with an error message
	"cancelled":  "Cancelled",  // Job has been cancelled
}

func SendPackageJobStatusEmail(c *fiber.Ctx, user *sm.User, entityId uuid.UUID, status string) (err error) {
	if user == nil {
		return fmt.Errorf("user is nil")
	}

	if !SupportedJobStatuses[status] {
		return fmt.Errorf("unsupported job status: %s", status)
	}

	// Apply user-friendly status message
	var ok bool
	if status, ok = JobStatusMap[status]; !ok {
		status = "Unknown"
	}

	var email string
	if user.Email != nil {
		email = *user.Email
	} else {
		return fmt.Errorf("user has no email")
	}

	var entity *Package
	if user.IsAdmin {
		entity, err = GetPackageForAdmin(c.Context(), user, entityId)
	} else {
		entity, err = GetPackageForAdmin(c.Context(), user, entityId)
	}
	if err != nil || entity == nil {
		return fmt.Errorf("failed to get package: %v", err)
	}

	if !user.AllowEmails {
		return fmt.Errorf("user disabled sending emails")
	} else {
		htmlTemplate := fmt.Sprintf(`<!DOCTYPE HTML PUBLIC "-//W3C//DTD XHTML 1.0 Transitional //EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:v="urn:schemas-microsoft-com:vml" xmlns:o="urn:schemas-microsoft-com:office:office">
<head></head><body>Your Metaverse package [%s] is now [%s]</body></html>`, entity.Name, status)
		if err = ses.Send("Metaverse SDK - Package Processing", fmt.Sprintf("Your Metaverse package [%s] is now [%s]", entity.Name, status), htmlTemplate, []string{email}, []string{}, []string{}, "builder@veverse.com"); err != nil {
			return fmt.Errorf("failed to send build job scheduled email: %v", err)
		}
	}

	return nil
}

func SendPackageJobLogEmail(c *fiber.Ctx, user *sm.User, entityId uuid.UUID, warnings []string, errors []string) (err error) {
	if user == nil {
		return fmt.Errorf("user is nil")
	}

	var email string
	if user.Email != nil {
		email = *user.Email
	} else {
		return fmt.Errorf("user has no email")
	}

	var entity *Package
	if user.IsAdmin {
		entity, err = GetPackageForAdmin(c.Context(), user, entityId)
	} else {
		entity, err = GetPackageForAdmin(c.Context(), user, entityId)
	}
	if err != nil || entity == nil {
		return fmt.Errorf("failed to get package: %v", err)
	}

	if !user.AllowEmails {
		return fmt.Errorf("user disabled sending emails")
	} else {
		//var textWarnings, htmlWarnings string
		//for _, warning := range warnings {
		//	textWarnings += fmt.Sprintf("- %s\n", warning)
		//	htmlWarnings += fmt.Sprintf("<li>%s</li>", warning)
		//}

		var textErrors, htmlErrors string
		for _, e := range errors {
			textErrors += fmt.Sprintf("- %s\n", e)
			htmlErrors += fmt.Sprintf("<li>%s</li>", e)
		}

		// Text template for the email.
		textTemplate := fmt.Sprintf("Your Metaverse package [%s] errors:\n%s", entity.Name, textErrors)
		//textTemplate := fmt.Sprintf("Your Metaverse package [%s] errors:\n%s\nwarnings:\n%s", entity.Name, textErrors, textWarnings)

		// HTML template for the email.
		htmlTemplate := fmt.Sprintf(`<!DOCTYPE HTML PUBLIC "-//W3C//DTD XHTML 1.0 Transitional //EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:v="urn:schemas-microsoft-com:vml" xmlns:o="urn:schemas-microsoft-com:office:office">
<head></head><body>Your Metaverse package [%s] processing log: <h3>Errors</h3><ul>%s</ul></body></html>`, entity.Name, htmlErrors)
		//		htmlTemplate := fmt.Sprintf(`<!DOCTYPE HTML PUBLIC "-//W3C//DTD XHTML 1.0 Transitional //EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">
		//<html xmlns="http://www.w3.org/1999/xhtml" xmlns:v="urn:schemas-microsoft-com:vml" xmlns:o="urn:schemas-microsoft-com:office:office">
		//<head></head><body>Your Metaverse package [%s] processing log: <h3>Errors</h3><ul>%s</ul><h3>Warnings</h3><ul>%s</ul></body></html>`, entity.Name, htmlErrors, htmlWarnings)
		if err = ses.Send("Metaverse SDK - Package Processing", textTemplate, htmlTemplate, []string{email}, []string{}, []string{}, "builder@veverse.com"); err != nil {
			return fmt.Errorf("failed to send build job scheduled email: %v", err)
		}
	}

	return nil
}

func SendRestoreLinkEmail(user *User, restoreLink string) (err error) {
	if user == nil {
		return fmt.Errorf("user is nil")
	}

	if !user.AllowEmails {
		return fmt.Errorf("user disabled sending emails")
	}

	var email string
	if user.Email != nil {
		email = *user.Email
	} else {
		return fmt.Errorf("user has no email")
	}

	htmlTemplate := fmt.Sprintf(`<!DOCTYPE HTML PUBLIC "-//W3C//DTD XHTML 1.0 Transitional //EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd">
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:v="urn:schemas-microsoft-com:vml" xmlns:o="urn:schemas-microsoft-com:office:office">
<head></head><body>Restore password - [%s]</body></html>`, restoreLink)
	if err = ses.Send("Restore password link", fmt.Sprintf("Restore password - [%s]", restoreLink), htmlTemplate, []string{email}, []string{}, []string{}, "no-reply@le7el.com"); err != nil {
		return fmt.Errorf("failed to send restore password link email")
	}

	return nil
}
