package matching

import (
	"sync"

	"github.com/exchange/internal/common/decimal"
)

// orderNodePool reuses orderNode allocations to reduce GC pressure.
var orderNodePool = sync.Pool{
	New: func() interface{} {
		return &orderNode{}
	},
}

// matchResultPool reuses MatchResult allocations.
var matchResultPool = sync.Pool{
	New: func() interface{} {
		return &MatchResult{}
	},
}

// GetMatchResult returns a pooled MatchResult. Call ReleaseMatchResult after use.
func GetMatchResult() *MatchResult {
	return matchResultPool.Get().(*MatchResult)
}

// ReleaseMatchResult returns a MatchResult to the pool.
func ReleaseMatchResult(m *MatchResult) {
	m.TakerOrderID = ""
	m.MakerOrderID = ""
	m.TakerUserID = ""
	m.MakerUserID = ""
	m.Symbol = ""
	m.Price = decimal.Zero
	m.Quantity = decimal.Zero
	m.QuoteQty = decimal.Zero
	m.TakerSide = ""
	m.Timestamp = 0
	matchResultPool.Put(m)
}
