package database

import (
	"context"
	vContext "dev.hackerman.me/artheon/veverse-shared/context"
	"dev.hackerman.me/artheon/veverse-shared/model"
	"fmt"
	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/ClickHouse/clickhouse-go/v2/lib/driver"
	"github.com/gofiber/fiber/v2"
	"github.com/gofrs/uuid"
	"github.com/sirupsen/logrus"
	"net"
	"os"
	"strconv"
	"time"
)

var Clickhouse driver.Conn

func SetupClickhouse() error {
	clickhouseHost := os.Getenv("CLICKHOUSE_HOST")
	clickhousePort := os.Getenv("CLICKHOUSE_PORT")
	clickhouseUser := os.Getenv("CLICKHOUSE_USER")
	clickhousePass := os.Getenv("CLICKHOUSE_PASS")
	clickhouseName := os.Getenv("CLICKHOUSE_NAME")

	clickhousePortNum, err := strconv.Atoi(clickhousePort)
	if err != nil {
		return fmt.Errorf("failed to convert clickhouse port to int: %w", err)
	}

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{fmt.Sprintf("%s:%d", clickhouseHost, clickhousePortNum)},
		Auth: clickhouse.Auth{
			Database: clickhouseName,
			Username: clickhouseUser,
			Password: clickhousePass,
		},
		Debug: true,
		Debugf: func(format string, v ...any) {
			fmt.Printf(format, v)
		},
		Settings: clickhouse.Settings{
			"max_execution_time": 60,
		},
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
		DialTimeout:      time.Duration(10) * time.Second,
		MaxOpenConns:     5,
		MaxIdleConns:     5,
		ConnMaxLifetime:  time.Duration(60) * time.Second,
		ConnOpenStrategy: clickhouse.ConnOpenInOrder,
		BlockBufferSize:  10,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to clickhouse: %w", err)
	}

	Clickhouse = conn

	return conn.Ping(context.Background())
}

func NewClickhouseMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.SetUserContext(context.WithValue(c.UserContext(), vContext.Clickhouse, Clickhouse))
		return c.Next()
	}
}

func CreateRequestRecord(ctx context.Context, request model.HttpRequestMetadata) error {
	if Clickhouse == nil {
		return fmt.Errorf("clickhouse is not connected")
	}

	q := "INSERT INTO request (ipv4, ipv6, userid, method, path, headers, body, status, source) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)"

	if err := Clickhouse.Exec(ctx, q, request.IPv4, request.IPv6, request.UserId, request.Method, request.Path, request.Headers, request.Body, request.Status, request.Source); err != nil {
		return fmt.Errorf("failed to create request record: %w", err)
	}

	return nil
}

func ReportRequestEvent(c *fiber.Ctx, requesterId uuid.UUID, status int) error {
	request := model.HttpRequestMetadata{}
	ip := net.ParseIP(c.IP())
	ipv4 := ip.To4()
	ipv6 := ip.To16()
	if ipv4 != nil {
		request.IPv4 = c.IP()
	}
	if ipv6 != nil {
		request.IPv6 = c.IP()
	}
	request.UserId = requesterId
	request.Method = c.Method()
	request.Path = c.Path()
	request.Headers = make(map[string]string)
	c.Request().Header.VisitAll(func(key, value []byte) {
		if string(key) == "Authorization" || string(key) == "Cookie" {
			return
		}

		if string(key) == "X-Forwarded-For" {
			ip := net.ParseIP(string(value))
			ipv4 := ip.To4()
			ipv6 := ip.To16()
			if ipv4 != nil {
				request.IPv4 = string(value)
			}
			if ipv6 != nil {
				request.IPv6 = string(value)
			}
		}

		if string(key) == "X-Real-Ip" {
			ip := net.ParseIP(string(value))
			ipv4 := ip.To4()
			ipv6 := ip.To16()
			if ipv4 != nil {
				request.IPv4 = string(value)
			}
			if ipv6 != nil {
				request.IPv6 = string(value)
			}
		}

		request.Headers[string(key)] = string(value)
	})
	request.Body = string(c.Body())
	request.Status = uint16(status)
	request.Source = "APIv2"

	err := CreateRequestRecord(c.UserContext(), request)
	if err != nil {
		logrus.Errorf("failed to create request record: %v", err)
	}

	return nil
}
