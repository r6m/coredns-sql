package sql

import (
	"net"
	"strconv"
	"strings"

	"github.com/coredns/coredns/plugin"
	"github.com/coredns/coredns/request"
	"github.com/jinzhu/gorm"
	"github.com/miekg/dns"
	"golang.org/x/net/context"
)

const pluginName = "sql"

type SQL struct {
	*gorm.DB
	Debug bool
	Next  plugin.Handler
}

func (sql *SQL) Name() string { return pluginName }
func (sql *SQL) ServeDNS(ctx context.Context, w dns.ResponseWriter, r *dns.Msg) (int, error) {
	state := request.Request{W: w, Req: r}

	a := new(dns.Msg)
	a.SetReply(r)
	a.Compress = true
	a.Authoritative = true

	records := make([]*Record, 0)
	query := Record{Name: state.QName(), Type: state.Type(), Disabled: false}
	if query.Name != "." {
		// remove last dot
		query.Name = query.Name[:len(query.Name)-1]
	}

	switch state.QType() {
	case dns.TypeANY:
		query.Type = ""
	}

	if err := sql.Where(query).Find(&records).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			query.Type = "SOA"
			if sql.Where(query).Find(&records).Error == nil {
				rr := new(dns.SOA)
				rr.Hdr = dns.RR_Header{Name: state.QName(), Rrtype: dns.TypeSOA, Class: state.QClass()}
				if ParseSOA(rr, records[0].Content) {
					a.Extra = append(a.Extra, rr)
				}
			}
		} else {
			return dns.RcodeServerFailure, err
		}
	} else {
		if len(records) == 0 {
			records, err = sql.SearchWildcard(state.QName(), state.QType())
			if err != nil {
				return dns.RcodeServerFailure, err
			}
		}
		for _, v := range records {
			typ := dns.StringToType[v.Type]
			hrd := dns.RR_Header{Name: state.QName(), Rrtype: typ, Class: state.QClass(), Ttl: uint32(v.TTL)}
			if !strings.HasSuffix(hrd.Name, ".") {
				hrd.Name += "."
			}
			rr := dns.TypeToRR[typ]()

			// todo support more type
			// this is enough for most query
			switch rr := rr.(type) {
			case *dns.SOA:
				rr.Hdr = hrd
				if !ParseSOA(rr, v.Content) {
					rr = nil
				}
			case *dns.A:
				rr.Hdr = hrd
				rr.A = net.ParseIP(v.Content)
			case *dns.AAAA:
				rr.Hdr = hrd
				rr.AAAA = net.ParseIP(v.Content)
			case *dns.TXT:
				rr.Hdr = hrd
				rr.Txt = []string{v.Content}
			case *dns.NS:
				rr.Hdr = hrd
				rr.Ns = v.Content
			case *dns.PTR:
				rr.Hdr = hrd
				// pdns don't need the dot but when we answer, we need it
				if strings.HasSuffix(v.Content, ".") {
					rr.Ptr = v.Content
				} else {
					rr.Ptr = v.Content + "."
				}
			case *dns.MX:
				rr.Hdr = hrd
				rr.Mx = v.Content
				rr.Preference = uint16(v.Prio)
			case *dns.SRV:
				rr.Hdr = hrd
				rr.Priority = uint16(v.Prio)
				words := strings.Fields(v.Content)
				if i, err := strconv.Atoi(words[0]); err == nil {
					rr.Weight = uint16(i)
				}
				if i, err := strconv.Atoi(words[1]); err == nil {
					rr.Port = uint16(i)
				}
				rr.Target = words[2]
			case *dns.CNAME:
				rr.Hdr = hrd
				rr.Target = v.Content
			default:
				// drop unsupported
			}

			if rr == nil {
				// invalid record
			} else {
				a.Answer = append(a.Answer, rr)
			}
		}
	}
	if len(a.Answer) == 0 {
		return plugin.NextOrFailure(sql.Name(), sql.Next, ctx, w, r)
	}

	return 0, w.WriteMsg(a)
}

func (sql *SQL) SearchWildcard(qname string, qtype uint16) (redords []*Record, err error) {
	// find domain, then find matched sub domain
	name := qname
	qnameNoDot := qname[:len(qname)-1]
	typ := dns.TypeToString[qtype]
	name = qnameNoDot
NEXT_ZONE:
	if i := strings.IndexRune(name, '.'); i > 0 && strings.Count(name, ".") > 1 {
		name = name[i+1:]
	} else {
		return
	}

	zone := new(Zone)
	if err := sql.Limit(1).Find(zone, "name = ?", name).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			goto NEXT_ZONE
		}
		return nil, err
	}

	if err := sql.Find(&redords, "zone_id = ? and ( ? = 'ANY' or type = ? ) and name like '%*%'", zone.ID, typ, typ).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	// filter
	matched := make([]*Record, 0)
	for _, v := range redords {
		if WildcardMatch(qnameNoDot, v.Name) {
			matched = append(matched, v)
		}
	}
	redords = matched
	return
}

func ParseSOA(rr *dns.SOA, line string) bool {
	splites := strings.Split(line, " ")
	if len(splites) < 7 {
		return false
	}
	rr.Ns = splites[0]
	rr.Mbox = splites[1]
	if i, err := strconv.Atoi(splites[2]); err != nil {
		return false
	} else {
		rr.Serial = uint32(i)
	}
	if i, err := strconv.Atoi(splites[3]); err != nil {
		return false
	} else {
		rr.Refresh = uint32(i)
	}
	if i, err := strconv.Atoi(splites[4]); err != nil {
		return false
	} else {
		rr.Retry = uint32(i)
	}
	if i, err := strconv.Atoi(splites[5]); err != nil {
		return false
	} else {
		rr.Expire = uint32(i)
	}
	if i, err := strconv.Atoi(splites[6]); err != nil {
		return false
	} else {
		rr.Minttl = uint32(i)
	}
	return true
}

// Dummy wildcard match
func WildcardMatch(s1, s2 string) bool {
	if s1 == "." || s2 == "." {
		return true
	}

	l1 := dns.SplitDomainName(s1)
	l2 := dns.SplitDomainName(s2)

	if len(l1) != len(l2) {
		return false
	}

	for i := range l1 {
		if !equal(l1[i], l2[i]) {
			return false
		}
	}

	return true
}

func equal(a, b string) bool {
	if b == "*" || a == "*" {
		return true
	}
	// might be lifted into API function.
	la := len(a)
	lb := len(b)
	if la != lb {
		return false
	}

	for i := la - 1; i >= 0; i-- {
		ai := a[i]
		bi := b[i]
		if ai >= 'A' && ai <= 'Z' {
			ai |= 'a' - 'A'
		}
		if bi >= 'A' && bi <= 'Z' {
			bi |= 'a' - 'A'
		}
		if ai != bi {
			return false
		}
	}
	return true
}
