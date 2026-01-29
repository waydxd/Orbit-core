package database

import (
	"context"
	"fmt"
	"sync"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB wraps the database connection pool
type DB struct {
	Pool *pgxpool.Pool
}

var (
	pool *pgxpool.Pool
	once sync.Once
)

// Connect establishes a connection pool to PostgreSQL
func Connect(connectionString string) (*DB, error) {
	var err error
	once.Do(func() {
		config, cfgErr := pgxpool.ParseConfig(connectionString)
		if cfgErr != nil {
			err = fmt.Errorf("failed to parse config: %w", cfgErr)
			return
		}

		// Set connection pool settings
		// You might want to parameterize these later via config
		config.MaxConns = 25
		config.MinConns = 5

		pool, cfgErr = pgxpool.NewWithConfig(context.Background(), config)
		if cfgErr != nil {
			err = fmt.Errorf("failed to create connection pool: %w", cfgErr)
			return
		}

		if pingErr := pool.Ping(context.Background()); pingErr != nil {
			err = fmt.Errorf("failed to ping database: %w", pingErr)
			return
		}
	})

	if err != nil {
		return nil, err
	}

	return &DB{Pool: pool}, nil
}

// Close closes the database connection pool
func (db *DB) Close() {
	if db.Pool != nil {
		db.Pool.Close()
	}
}
