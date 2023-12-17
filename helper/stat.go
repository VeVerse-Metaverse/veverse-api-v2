package helper

import (
	"fmt"
	"veverse-api/database"
)

func LogPgxStat(msg string) {
	stat := database.DB.Stat()
	fmt.Printf("pgxstat (%s):\n{\n\tAcquireCount: %d,\n\tAcquireDuration: %d,\n\tAcquiredConns: %d,\n\tCanceledAcquireCount: %d,\n\tConstructingConns: %d,\n\tEmptyAquireCount: %d,\n\tIdleConns: %d,\n\tMaxConns: %d,\n\tTotalConns:%d\n}\n", msg, stat.AcquireCount(), stat.AcquireDuration(), stat.AcquiredConns(), stat.CanceledAcquireCount(), stat.ConstructingConns(), stat.EmptyAcquireCount(), stat.IdleConns(), stat.MaxConns(), stat.TotalConns())
}
