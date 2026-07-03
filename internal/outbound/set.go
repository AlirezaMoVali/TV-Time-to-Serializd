package outbound

// Set holds per-service outbound gates shared across migrations and exports.
type Set struct {
	TVTime    *Gate
	Serializd *Gate
	Wikidata  *Gate
	TMDB      *Gate
}

// NewSetFromEnv builds gates with defaults tuned for multi-migration throughput
// without exceeding common provider limits.
//
// Defaults: TVTime=24, Serializd=12, Wikidata=8, TMDB=8
func NewSetFromEnv() *Set {
	return &Set{
		TVTime:    NewGate(envInt("OUTBOUND_TVTIME_MAX", 24)),
		Serializd: NewGate(envInt("OUTBOUND_SERIALIZD_MAX", 12)),
		Wikidata:  NewGate(envInt("OUTBOUND_WIKIDATA_MAX", 8)),
		TMDB:      NewGate(envInt("OUTBOUND_TMDB_MAX", 8)),
	}
}
