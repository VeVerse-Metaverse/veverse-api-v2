package ses

import (
	"fmt"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ses"
	"log"
	"os"
)

var EmailService *ses.SES

var awsRegion = os.Getenv("AWS_SES_REGION")
var awsSesBounceAddress = os.Getenv("AWS_SES_BOUNCE_ADDRESS")
var awsSesSenderAddress = os.Getenv("AWS_SES_SENDER_ADDRESS")

func Setup() error {
	if awsRegion == "" {
		log.Fatalf("required AWS env not provided (AWS_SES_REGION)")
	}

	if awsSesBounceAddress == "" {
		log.Fatalf("required AWS env not provided (AWS_SES_BOUNCE_ADDRESS)")
	}

	if awsSesSenderAddress == "" {
		log.Fatalf("required AWS env not provided (AWS_SES_SENDER_ADDRESS)")
	}

	awsSession, err := session.NewSession(&aws.Config{
		Credentials: credentials.NewEnvCredentials(),
		Region:      aws.String(awsRegion),
	})

	if err != nil {
		log.Fatalf("failed to initialize a new AWS session: %v", err)
	}

	EmailService = ses.New(awsSession)
	if EmailService == nil {
		return fmt.Errorf("failed to create a SES client")
	}

	return nil
}

func Send(subject string, text string, html string, toAddresses []string, ccAddresses []string, bccAddresses []string, senderEmail string) error {
	if senderEmail == "" {
		senderEmail = awsSesSenderAddress
	}

	input := ses.SendEmailInput{
		Destination: &ses.Destination{
			BccAddresses: aws.StringSlice(bccAddresses),
			CcAddresses:  aws.StringSlice(ccAddresses),
			ToAddresses:  aws.StringSlice(toAddresses),
		},
		Message: &ses.Message{
			Body: &ses.Body{
				Html: &ses.Content{
					Charset: aws.String("utf8"),
					Data:    aws.String(html),
				},
				Text: &ses.Content{
					Charset: aws.String("utf8"),
					Data:    aws.String(text),
				},
			},
			Subject: &ses.Content{
				Charset: aws.String("utf8"),
				Data:    aws.String(subject),
			},
		},
		ReplyToAddresses: aws.StringSlice([]string{senderEmail}),
		ReturnPath:       aws.String(awsSesBounceAddress),
		Source:           aws.String(awsSesSenderAddress),
	}

	result, err := EmailService.SendEmail(&input)
	if err != nil {
		if awsErr, ok := err.(awserr.Error); ok {
			switch awsErr.Code() {
			case ses.ErrCodeMessageRejected:
				fmt.Println(ses.ErrCodeMessageRejected, awsErr.Error())
			case ses.ErrCodeMailFromDomainNotVerifiedException:
				fmt.Println(ses.ErrCodeMailFromDomainNotVerifiedException, awsErr.Error())
			case ses.ErrCodeConfigurationSetDoesNotExistException:
				fmt.Println(ses.ErrCodeConfigurationSetDoesNotExistException, awsErr.Error())
			default:
				fmt.Println(awsErr.Error())
			}
		} else {
			fmt.Println(err.Error())
		}
		return err
	}

	fmt.Println(result)

	return nil
}
