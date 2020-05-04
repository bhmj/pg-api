package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/bhmj/pg-api/internal/pkg/config"
	//	_ "github.com/jackc/pgx"
	_ "github.com/lib/pq"
)

//Database constants
const (
	databaseConnectionTimeout = 10
	databaseCheckMaxAttempts  = 120
	databaseCheckSleepTime    = 5 * time.Second
)

// SetupDatabase opens database
func SetupDatabase(conf config.Database) (*sql.DB, error) {
	connStr := conf.ConnString
	if connStr == "" {
		connStr = fmt.Sprintf(
			"host=%s port=%d dbname=%s user=%s password=%s sslmode=disable connect_timeout=%d",
			conf.Host,
			conf.Port,
			conf.Name,
			conf.User,
			conf.Password,
			databaseConnectionTimeout,
		)
	}
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}

	db.SetMaxOpenConns(conf.MaxConn)
	db.SetMaxIdleConns(conf.MaxConn)
	db.SetConnMaxLifetime(-1) // forever

	return db, nil
}
