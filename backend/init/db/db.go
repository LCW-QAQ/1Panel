package db

import (
	"fmt"
	"log"
	"os"
	"path"
	"time"

	"github.com/1Panel-dev/1Panel/backend/global"
	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func Init() {
	var dsn string
	if global.CONF.System.DbType != "" {
		dsn = global.CONF.System.Dsn
	} else {
		dsn = getDbFilePath(global.CONF.System.DbFile)
	}
	dialector := buildDialector(global.CONF.System.DbType, dsn)

	var logLevel logger.LogLevel
	switch global.CONF.System.OrmLogLevel {
	case "Info":
		logLevel = logger.Info
	case "Warn":
		logLevel = logger.Warn
	case "Error":
		logLevel = logger.Error
	case "Silent":
	default:
		logLevel = logger.Silent
	}

	newLogger := logger.New(
		log.New(os.Stdout, "\r\n", log.LstdFlags),
		logger.Config{
			SlowThreshold:             time.Second,
			LogLevel:                  logLevel,
			IgnoreRecordNotFoundError: true,
			Colorful:                  false,
		},
	)

	global.DB = createDBWithLogger(dialector, newLogger)
	global.LOG.Info("init db successfully")

	initMonitorDB(newLogger)
}

func buildDialector(dbType string, dsn string) gorm.Dialector {
	if dbType == "mysql" {
		return mysql.Open(dsn)
	} else {
		return sqlite.Open(dsn)
	}
}

func getDbFilePath(dbFile string) string {
	if _, err := os.Stat(global.CONF.System.DbPath); err != nil {
		if err := os.MkdirAll(global.CONF.System.DbPath, os.ModePerm); err != nil {
			panic(fmt.Errorf("init db dir failed, err: %v", err))
		}
	}
	fullPath := path.Join(global.CONF.System.DbPath, dbFile)
	if _, err := os.Stat(fullPath); err != nil {
		f, err := os.Create(fullPath)
		if err != nil {
			panic(fmt.Errorf("init db file failed, err: %v", err))
		}
		_ = f.Close()
	}
	return fullPath
}

func createDBWithLogger(dialector gorm.Dialector, loggerInstance logger.Interface) *gorm.DB {
	db, err := gorm.Open(dialector, &gorm.Config{
		DisableForeignKeyConstraintWhenMigrating: true,
		Logger:                                   loggerInstance,
	})
	if err != nil {
		panic(err)
	}

	if _, ok := dialector.(sqlite.Dialector); ok {
		_ = db.Exec("PRAGMA journal_mode = WAL;")
	}

	sqlDB, dbError := db.DB()
	if dbError != nil {
		panic(dbError)
	}
	sqlDB.SetConnMaxIdleTime(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	return db
}

func initMonitorDB(newLogger logger.Interface) {
	if global.CONF.System.DbType != "" {
		global.MonitorDB = global.DB
		return
	}
	fullPath := getDbFilePath("monitor.db")
	global.MonitorDB = createDBWithLogger(sqlite.Open(fullPath), newLogger)
	global.LOG.Info("init monitor db successfully")
}
