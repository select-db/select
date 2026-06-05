package datasource

import (
	"time"

	"github.com/selectDb/toolkit/cache"
)

var pingCache = cache.New(cache.Options{
	MaxEntries: 20_000,
	TTL:        20 * time.Second,
})
