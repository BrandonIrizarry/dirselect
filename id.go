package dirselect

import "sync/atomic"

var lastID int64

func nextID() int {
	return int(atomic.AddInt64(&lastID, 1))
}
