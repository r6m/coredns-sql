package sql

import (
	"log"

	"github.com/coredns/caddy"
	"github.com/coredns/coredns/core/dnsserver"
	"github.com/coredns/coredns/plugin"
	"github.com/jinzhu/gorm"
)

func init() {
	caddy.RegisterPlugin(pluginName, caddy.Plugin{
		ServerType: "dns",
		Action:     setup,
	})
}

func setup(c *caddy.Controller) error {
	backend := &SQL{}
	c.Next()
	if !c.NextArg() {
		return plugin.Error(pluginName, c.ArgErr())
	}
	dialect := c.Val()

	if !c.NextArg() {
		return plugin.Error(pluginName, c.ArgErr())
	}
	arg := c.Val()

	db, err := gorm.Open(dialect, arg)
	if err != nil {
		return err
	}
	backend.DB = db

	for c.NextBlock() {
		x := c.Val()
		switch x {
		case "debug":
			args := c.RemainingArgs()
			for _, v := range args {
				switch v {
				case "db":
					backend.DB = backend.DB.Debug()
				}
			}
			backend.Debug = true
			log.Println(pluginName, "enable log", args)
		case "auto-migrate":
			// currently only use records table
			if err := backend.AutoMigrate(); err != nil {
				return err
			}
		default:
			return plugin.Error(pluginName, c.Errf("unexpected '%v' command", x))
		}
	}

	if c.NextArg() {
		return plugin.Error(pluginName, c.ArgErr())
	}

	dnsserver.
		GetConfig(c).
		AddPlugin(func(next plugin.Handler) plugin.Handler {
			backend.Next = next
			return backend
		})

	return nil
}

func (sql SQL) AutoMigrate() error {
	return sql.DB.AutoMigrate(&Record{}, &Zone{}).Error
}
