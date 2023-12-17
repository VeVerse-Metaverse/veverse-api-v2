package database

import (
	"context"
	vContext "dev.hackerman.me/artheon/veverse-shared/context"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
)

// DB gorm connector
var DB *pgxpool.Pool

// Setup connect to db
func Setup() error {
	// urlExample := "postgres://username:password@localhost:5432/database_name"
	databaseHost := os.Getenv("DATABASE_HOST")
	databasePort := os.Getenv("DATABASE_PORT")
	databaseUser := os.Getenv("DATABASE_USER")
	databasePass := os.Getenv("DATABASE_PASS")
	databaseName := os.Getenv("DATABASE_NAME")

	databaseUrl := fmt.Sprintf("postgres://%s:%s@%s:%s/%s", databaseUser, databasePass, databaseHost, databasePort, databaseName)

	dbconfig, err := pgxpool.ParseConfig(databaseUrl)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "unable to connect to database: %v\n", err)
		return err
	}

	pool, err := pgxpool.NewWithConfig(context.Background(), dbconfig)

	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "unable to connect to database: %v\n", err)
		return err
	}

	DB = pool
	//defer pool.Close()

	return nil
}

func NewMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		c.SetUserContext(context.WithValue(c.UserContext(), vContext.Database, DB))

		return c.Next()
	}
}

func LogPgxStat(msg string) {
	stat := DB.Stat()
	fmt.Printf("pgxstat (%s):\n{\n\tAcquireCount: %d,\n\tAcquireDuration: %d,\n\tAcquiredConns: %d,\n\tCanceledAcquireCount: %d,\n\tConstructingConns: %d,\n\tEmptyAquireCount: %d,\n\tIdleConns: %d,\n\tMaxConns: %d,\n\tTotalConns:%d\n}\n", msg, stat.AcquireCount(), stat.AcquireDuration(), stat.AcquiredConns(), stat.CanceledAcquireCount(), stat.ConstructingConns(), stat.EmptyAcquireCount(), stat.IdleConns(), stat.MaxConns(), stat.TotalConns())
}
