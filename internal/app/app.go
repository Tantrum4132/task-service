package app

import (
	"database/sql"
	"fmt"

	"github.com/Tantrum4132/task-service/internal/config"
	"github.com/Tantrum4132/task-service/internal/logger"

	_ "github.com/go-sql-driver/mysql"
	redisclient "github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

type Container struct {
	Config       *config.Config
	Logger       *zap.Logger
	DB           *sql.DB
	Redis        redisclient.Cmdable
	Repositories *Repositories
}

func NewContainer(configPath string) (*Container, error) {
	cfg, err := config.Load(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	log, err := logger.NewLogger(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to init logger: %w", err)
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true",
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Name,
	)

	db, err := sql.Open(cfg.Database.Dialect, dsn)
	if err != nil {
		log.Error("failed to open database connection", zap.Error(err))
		return nil, fmt.Errorf("failed to open db: %w", err)
	}

	db.SetMaxOpenConns(cfg.Database.MaxOpenConnections)
	db.SetMaxIdleConns(cfg.Database.MaxIdleConnections)
	db.SetConnMaxLifetime(cfg.Database.ConnectionLifetime)

	if err := db.Ping(); err != nil {
		log.Error("failed to ping database", zap.Error(err))
		return nil, fmt.Errorf("failed to ping db: %w", err)
	}

	var rdb redisclient.Cmdable
	if cfg.Redis.Enabled {
		rdb = redisclient.NewClient(&redisclient.Options{
			Addr:     cfg.Redis.Addr,
			Password: cfg.Redis.Password,
			DB:       cfg.Redis.DB,
		})
		log.Info("redis client initialized", zap.String("addr", cfg.Redis.Addr))
	}

	repos := NewRepositories(db, rdb, cfg)

	log.Info("application container initialized successfully",
		zap.String("env", cfg.Environment),
		zap.String("log_level", cfg.Logging.Level),
	)

	return &Container{
		Config:       cfg,
		Logger:       log,
		DB:           db,
		Redis:        rdb,
		Repositories: repos,
	}, nil
}

func (c *Container) Close() {
	if c.Logger != nil {
		_ = c.Logger.Sync()
	}
	if c.DB != nil {
		_ = c.DB.Close()
	}
}
