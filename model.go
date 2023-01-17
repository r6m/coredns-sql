package sql

type Zone struct {
	ID     uint
	Name   string
	Master string
	Type   string
}

type Record struct {
	ID       uint
	ZoneID   int64
	Name     string
	Type     string
	Content  string
	TTL      int32
	Prio     int
	Disabled bool
}
