package tvtime

type GatherPhase string

const (
	GatherPhaseFollows  GatherPhase = "follows"
	GatherPhaseWatches  GatherPhase = "watches"
	GatherPhaseEpisodes GatherPhase = "episodes"
	GatherPhaseDump     GatherPhase = "dump"
)

type GatherProgress struct {
	Phase    GatherPhase
	Done     int
	Total    int
	Detail   string
	ShowName string
}

type GatherProgressFunc func(GatherProgress)
