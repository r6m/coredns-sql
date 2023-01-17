package sql_test

import (
	"testing"

	sql "github.com/r6m/coredns-sql"

	"github.com/coredns/coredns/plugin/pkg/dnstest"
	"github.com/coredns/coredns/plugin/test"

	"github.com/jinzhu/gorm"
	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

func TestPowerDNSSQL(t *testing.T) {
	db, err := gorm.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatal(err)
	}

	p := sql.SQL{DB: db.Debug()}
	if err := p.AutoMigrate(); err != nil {
		t.Fatal(err)
	}

	p.DB.Create(&sql.Record{
		Name:    "example.org",
		Type:    "A",
		Content: "192.168.1.1",
		TTL:     3600,
	})

	tests := []struct {
		qname         string
		qtype         uint16
		expectedCode  int
		expectedReply []string // ownernames for the records in the additional section.
		expectedErr   error
	}{
		{
			qname:         "example.org.",
			qtype:         dns.TypeA,
			expectedCode:  dns.RcodeSuccess,
			expectedReply: []string{"example.org."},
			expectedErr:   nil,
		},
	}

	ctx := context.TODO()

	for i, tc := range tests {
		req := new(dns.Msg)
		req.SetQuestion(dns.Fqdn(tc.qname), tc.qtype)

		rec := dnstest.NewRecorder(&test.ResponseWriter{})
		code, err := p.ServeDNS(ctx, rec, req)

		if err != tc.expectedErr {
			t.Errorf("Test %d: Expected error %v, but got %v", i, tc.expectedErr, err)
		}
		if code != tc.expectedCode {
			t.Errorf("Test %d: Expected status code %d, but got %d", i, tc.expectedCode, code)
		}
		if len(tc.expectedReply) != len(rec.Msg.Answer) {
			t.Errorf("Test %d: Expected status len %d, but got %d", i, len(tc.expectedReply), len(rec.Msg.Answer))
		}

		for i, expected := range tc.expectedReply {
			actual := rec.Msg.Answer[i]
			if actual.Header().Name != expected {
				t.Errorf("Test %d: Expected answer %s, but got %s", i, expected, actual)
			}
			if actual.Header().Rrtype != tc.qtype {
				t.Errorf("Test %d: Expected answer type %s, but got %s", i, dns.TypeToString[tc.qtype], dns.TypeToString[actual.Header().Rrtype])
			}

			if actual.Header().Rrtype == tc.qtype {
				answer := actual.(*dns.A)
				if answer.A.String() != "192.168.1.1" {
					t.Errorf("Test %d: Expected answer content %s, but got %s", i, "192.168.1.1", answer.A.String())
				}
			}
		}
	}
}

func TestWildcardMatch(t *testing.T) {

	tests := []struct {
		pattern  string
		name     string
		expected bool
	}{
		{"*.example.org.", "example.org.", false},
		{"a.example.org.", "a.example.org.", true},
		{"*.example.org.", "a.example.org.", true},
		{"*.example.org.", "abcd.example.org.", true},
	}

	for i, tc := range tests {
		act := sql.WildcardMatch(tc.name, tc.pattern)
		if tc.expected != act {
			t.Errorf("Test %d: Expected  %v, but got %v", i, tc.expected, act)
		}
	}
}
