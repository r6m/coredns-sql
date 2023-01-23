package sql

import (
	"testing"

	"github.com/coredns/caddy"
	_ "gorm.io/driver/sqlite"
)

func TestSetupSql(t *testing.T) {
	c := caddy.NewTestController("dns", `sql sqlite3 :memory:`)
	if err := setup(c); err != nil {
		t.Fatalf("Expected no errors, but got: %v", err)
	}

	c = caddy.NewTestController("dns", `sql sqlite3 :memory: {
}`)
	if err := setup(c); err != nil {
		t.Fatalf("Expected no errors, but got: %v", err)
	}

	c = caddy.NewTestController("dns", `sql sqlite3 :memory: {
debug db
auto_migrate
}`)
	if err := setup(c); err != nil {
		t.Fatalf("Expected no errors, but got: %v", err)
	}

	c = caddy.NewTestController("dns", `sql sqlite3 :memory: {
debug
table_prefix dns_
auto_migrate
}`)
	if err := setup(c); err != nil {
		t.Fatalf("Expected no errors, but got: %v", err)
	}

	c = caddy.NewTestController("dns", `sql sqlite`)
	if err := setup(c); err == nil {
		t.Fatalf("Expected errors, but got: %v", err)
	}

	c = caddy.NewTestController("dns", `sql sqlite3 :memory: {
unknown
}`)
	if err := setup(c); err == nil {
		t.Fatalf("Expected errors, but got: %v", err)
	}

	c = caddy.NewTestController("dns", `sql sqlite3 :memory: {
debug
unknown
}`)
	if err := setup(c); err == nil {
		t.Fatalf("Expected errors, but got: %v", err)
	}

	c = caddy.NewTestController("dns", `sql sqlite3 :memory: {
debug
} invalid`)
	if err := setup(c); err == nil {
		t.Fatalf("Expected errors, but got: %v", err)
	}

	c = caddy.NewTestController("dns", `sql sqlite3 :memory: {
auto_migrate invalid
}`)
	if err := setup(c); err == nil {
		t.Fatalf("Expected errors, but got: %v", err)
	}
}
