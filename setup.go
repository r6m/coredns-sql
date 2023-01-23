package sql

import (
	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlite"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"
)

func init() {
	caddy.RegisterPlugin(pluginName, caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	sql := &SQL{}

	var (
		migrate bool
		config  gorm.Config
	)

	c.Next()
	if !c.NextArg() {
		return plugin.Error(pluginName, c.ArgErr())
	}
	dialect := c.Val()

	if !c.NextArg() {
		return plugin.Error(pluginName, c.ArgErr())
	}
	dsn := c.Val()

	var dialector gorm.Dialector
	switch dialect {
	case "mysql":
		dialector = mysql.Open(dsn)
	case "postgres", "postgresql":
		dialector = postgres.Open(dsn)
	case "sqlite", "sqlite3":
		dialector = sqlite.Open(dsn)
	case "sqlserver":
		dialector = sqlserver.Open(dsn)
	}

	for c.NextBlock() {
		x := c.Val()
		switch x {
		case "debug":
			args := c.RemainingArgs()
			if len(args) > 0 && args[0] == "db" {
				config.Logger = logger.Default.LogMode(logger.Info)
			}
			sql.Debug = true
		case "auto_migrate":
			migrate = true
		case "table_prefix":
			args := c.RemainingArgs()
			if len(args) == 0 {
				return plugin.Error(pluginName, c.ArgErr())
			}

			config.NamingStrategy = schema.NamingStrategy{
				TablePrefix: args[0],
			}

		default:
			return plugin.Error(pluginName, c.Errf("unexpected '%v' command", x))
		}
	}

	if c.NextArg() {
		return plugin.Error(pluginName, c.ArgErr())
	}

	db, err := gorm.Open(dialector, &config)
	if err != nil {
		return err
	}

	sql.DB = db

	if migrate {
		if err := sql.Migrate(); err != nil {
			return err
		}
	}

	dnsserver.
		GetConfig(c).
		AddPlugin(func(next plugin.Handler) plugin.Handler {
			sql.Next = next
			return sql
		})

	return nil
}

func (sql *SQL) Migrate() error {
	return sql.AutoMigrate(&Zone{}, &Record{})
}
