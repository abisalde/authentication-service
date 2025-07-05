package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	"entgo.io/ent/dialect/sql/schema"
	"github.com/abisalde/authentication-service/internal/configs"
	"github.com/abisalde/authentication-service/internal/database/ent"
	_ "github.com/go-sql-driver/mysql"
)

var (
	client     *ent.Client
	clientOnce sync.Once
	initErr    error
)

type Database struct {
	Client *ent.Client
	config *configs.Config
	SQLDB  *sql.DB
}

func Connect(cfg *configs.Config) (*Database, error) {
	clientOnce.Do(func() {

		sqlDB, err := initDatabase(cfg)
		if err != nil {
			log.Fatalf("🛑 Database initialization failed %v", err)
			return
		}

		drv := entsql.OpenDB(dialect.MySQL, sqlDB)
		client = ent.NewClient(ent.Driver(drv), ent.Debug(), ent.Log(log.Print))

		if cfg.DB.Migrate {
			if err := migrate(context.Background(), client); err != nil {
				initErr = fmt.Errorf("🛠️ Database migration failed: %w", err)
				_ = client.Close()
				return
			}
		}
	})

	if initErr != nil {
		return nil, initErr
	}

	return &Database{
		Client: client,
		config: cfg,
		SQLDB:  &sql.DB{},
	}, nil
}

func (db *Database) Close() error {
	if db.Client == nil {
		return nil
	}

	if err := db.Client.Close(); err != nil {
		return fmt.Errorf("failed to close database connection: %w", err)
	}

	client = nil
	clientOnce = sync.Once{}
	return nil
}

func migrate(ctx context.Context, client *ent.Client) error {
	return client.Schema.Create(
		ctx,
		schema.WithDropIndex(true),
		schema.WithDropColumn(true),
		schema.WithForeignKeys(true),
	)
}

func initDatabase(cfg *configs.Config) (*sql.DB, error) {

	sqlDB, err := sql.Open(dialect.MySQL, cfg.SQL_DSB())
	if err != nil {
		return nil, fmt.Errorf("❌ Failed to open database connection: %w", err)
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("⚙️ Database ping failed: %w", err)
	}

	return sqlDB, nil

}

func (db *Database) HealthCheck(ctx context.Context) error {
	sqlDB := db.SQLDB
	if sqlDB == nil {
		return fmt.Errorf("sql.DB is not initialized")
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
	}
	return sqlDB.PingContext(ctx)
}
