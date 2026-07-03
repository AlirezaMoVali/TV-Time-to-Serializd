package service

import "github.com/alireza/tvtime2serializd/internal/tvtime"

func applyGatherProgress(state *migrateState, p tvtime.GatherProgress) {
	state.progress.GatherShows.Status = StepRunning
	state.progress.GatherShows.Phase = string(p.Phase)
	state.progress.GatherShows.Done = p.Done
	state.progress.GatherShows.Total = p.Total
	remaining := 0
	if p.Total > 0 {
		remaining = p.Total - p.Done
	}
	state.progress.GatherShows.Remaining = remaining
	if p.Detail != "" {
		state.progress.CurrentActivity = p.Detail
	}
	if p.ShowName != "" {
		state.progress.CurrentShow = p.ShowName
		state.progress.ActiveShows = []string{p.ShowName}
	}
}
