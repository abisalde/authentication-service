package database

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/url"
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
	var (
		sqlDB    *sql.DB
		dbClient *ent.Client
	)

	clientOnce.Do(func() {
		var err error
		sqlDB, err = initDatabase(cfg)
		if err != nil {
			initErr = fmt.Errorf("🛑 Database initialization failed: %w", err)
			return
		}
		env := cfg.Env.CurrentEnv
		isDev := env != "production"

		drv := entsql.OpenDB(dialect.MySQL, sqlDB)
		dbClient = ent.NewClient(ent.Driver(drv), ent.Debug(), ent.Log(log.Print))

		if cfg.DB.Migrate {
			if err := migrate(context.Background(), dbClient, isDev); err != nil {
				initErr = fmt.Errorf("🛠️ Database migration failed: %w", err)
				_ = dbClient.Close()
				_ = sqlDB.Close()
				return
			}
		}
	})

	if initErr != nil {
		return nil, initErr
	}

	return &Database{
		Client: dbClient,
		config: cfg,
		SQLDB:  sqlDB,
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

func migrate(ctx context.Context, client *ent.Client, isDev bool) error {
	if isDev {
		return client.Schema.Create(
			ctx,
			schema.WithDropIndex(true),
			schema.WithDropColumn(true),
			schema.WithForeignKeys(true),
		)
	}

	return client.Schema.Create(
		ctx,
		schema.WithDropIndex(false),
		schema.WithDropColumn(false),
		schema.WithForeignKeys(true),
	)
}

func initDatabase(cfg *configs.Config) (*sql.DB, error) {

	dbUrl := url.QueryEscape(cfg.SQL_DSB())

	sqlDB, err := sql.Open(dialect.MySQL, dbUrl)
	if err != nil {
		return nil, fmt.Errorf("❌ Failed to open database connection: %w", err)
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := sqlDB.PingContext(ctx); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("⚙️ Database ping failed: %w", err)
	}

	return sqlDB, nil

}

func (db *Database) HealthCheck(ctx context.Context) error {
	if db.SQLDB == nil {
		return fmt.Errorf("🎛️ Database connection not initialized")
	}

	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
	}

	if err := db.SQLDB.PingContext(ctx); err != nil {
		return fmt.Errorf("🕹️ Database ping failed: %w", err)
	}

	_, err := db.SQLDB.ExecContext(ctx, "SELECT 1 FROM users LIMIT 1")
	if err != nil {
		return fmt.Errorf("🩸 Database schema verification failed: %w", err)
	}

	return nil
}
