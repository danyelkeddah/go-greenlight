package main

import (
	"context"
	"database/sql"
	"expvar"
	"flag"
	"fmt"
	"github.com/danyelkeddah/go-greenlight/internal/data"
	"github.com/danyelkeddah/go-greenlight/internal/jsonlog"
	"github.com/danyelkeddah/go-greenlight/internal/mailer"
	"github.com/danyelkeddah/go-greenlight/internal/vcs"
	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	_ "github.com/lib/pq"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"
)

var (
	version = vcs.Version()
)

type config struct {
	port int
	env  string

	db struct {
		dsn                string
		maxOpenConnections int
		maxIdleConnections int
		maxIdleTime        string
	}
	// Add a new limiter struct containing fields for the requests-per-second and burst values,
	// and a boolean field which we can use to enable/disable rate limiting altogether
	limiter struct {
		rps     float64
		burst   int
		enabled bool
	}
	smtp struct {
		host     string
		port     int
		username string
		password string
		sender   string
	}
	cors struct {
		trustedOrigins []string
	}
}

type application struct {
	config config
	logger *jsonlog.Logger
	models data.Models
	mailer mailer.Mailer
	wg     sync.WaitGroup
}

func main() {
	var cfg config                                                                                 // create config with empty values
	flag.IntVar(&cfg.port, "port", 4000, "API server port")                                        // parse port from command line
	flag.StringVar(&cfg.env, "env", "development", "Environment (development|staging|production)") // parse env from command line
	flag.StringVar(&cfg.db.dsn, "db-dsn", os.Getenv("GREENLIGHT_DB_DSN"), "PostgreSQL DSN")
	flag.IntVar(&cfg.db.maxOpenConnections, "db-max-open-conns", 25, "PostgreSQL max open connections")
	flag.IntVar(&cfg.db.maxIdleConnections, "db-max-idle-conns", 25, "PostgreSQL max idle connections")
	flag.StringVar(&cfg.db.maxIdleTime, "db-max-idle-time", "15m", "PostgreSQL max connection idle time")
	flag.Float64Var(&cfg.limiter.rps, "limiter-rps", 2, "Rate limiter maximum requests per second")
	flag.IntVar(&cfg.limiter.burst, "limiter-burst", 4, "Rate limiter maximum burst")
	flag.BoolVar(&cfg.limiter.enabled, "limiter-enabled", true, "Enable rate limiter")
	flag.StringVar(&cfg.smtp.host, "smtp-host", "127.0.0.1", "SMTP host")
	flag.IntVar(&cfg.smtp.port, "smtp-port", 2525, "SMTP port")
	flag.StringVar(&cfg.smtp.username, "smtp-username", "greenlight", "SMTP username")
	flag.StringVar(&cfg.smtp.password, "smtp-password", "", "SMTP password")
	flag.StringVar(&cfg.smtp.sender, "smtp-sender", "Greenlight <no-reply@greenlight.danyel.dev>", "SMTP sender")
	flag.Func("cors-trusted-origins", "Trusted CORS origins (spsace separated)", func(s string) error {
		cfg.cors.trustedOrigins = strings.Fields(s)
		return nil
	})
	displayVersion := flag.Bool("version", false, "Display version and exit")

	flag.Parse()
	if *displayVersion {
		fmt.Printf("Version:\t%s\n", version)
		os.Exit(0)
	}

	logger := jsonlog.New(os.Stdout, jsonlog.LevelInfo)
	db, err := openDB(cfg)

	if err != nil {
		logger.PrintFatal(err, nil)
	}

	defer db.Close()

	logger.PrintInfo("database connection pool established", nil)

	migrationDriver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		logger.PrintFatal(err, nil)
	}

	pwd, _ := os.Getwd()

	migrator, err := migrate.NewWithDatabaseInstance(fmt.Sprintf("file:///%s/migrations", pwd), "postgres", migrationDriver)
	if err != nil {
		logger.PrintFatal(err, nil)
	}
	err = migrator.Up()
	if err != nil && err != migrate.ErrNoChange {
		logger.PrintFatal(err, nil)
	}
	logger.PrintInfo("database migrations applied", nil)

	expvar.NewString("version").Set(version)

	expvar.Publish("goroutines", expvar.Func(func() any {
		return runtime.NumGoroutine()
	}))

	expvar.Publish("database", expvar.Func(func() any {
		return db.Stats()
	}))

	expvar.Publish("timestamp", expvar.Func(func() any {
		return time.Now().Unix()
	}))

	app := &application{ // app has config and logger
		config: cfg,
		logger: logger,
		models: data.NewModels(db),
		mailer: mailer.New(cfg.smtp.host, cfg.smtp.port, cfg.smtp.username, cfg.smtp.password, cfg.smtp.sender),
	}

	err = app.serve()
	if err != nil {
		logger.PrintFatal(err, nil)
	}

}

func openDB(cfg config) (*sql.DB, error) {
	// Use sql.Open() to create an empty connection pool
	db, err := sql.Open("postgres", cfg.db.dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(cfg.db.maxOpenConnections)
	db.SetMaxIdleConns(cfg.db.maxIdleConnections)
	duration, err := time.ParseDuration(cfg.db.maxIdleTime)
	if err != nil {
		return nil, err
	}
	db.SetConnMaxIdleTime(duration)

	// Create a context with a 5-second timeout deadline.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Use PingContext() to establish a new connection to the database, passing in the
	// context we created above as a parameter. If the connection couldn't be
	// established successfully within the 5 seconds deadline, then this will return an error.
	err = db.PingContext(ctx)
	if err != nil {
		return nil, err
	}

	// Return the sql.DB connection pool.
	return db, nil
}
