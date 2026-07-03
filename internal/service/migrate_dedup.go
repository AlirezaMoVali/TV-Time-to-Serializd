package service

func mergeTrackedShowIDs(library map[int]struct{}, previouslyImported map[int]struct{}) map[int]struct{} {
	out := make(map[int]struct{}, len(library)+len(previouslyImported))
	for id := range library {
		out[id] = struct{}{}
	}
	for id := range previouslyImported {
		out[id] = struct{}{}
	}
	return out
}

func filterPendingImports(pending []pendingImport, tracked map[int]struct{}) ([]pendingImport, int) {
	if len(tracked) == 0 {
		return pending, 0
	}
	out := make([]pendingImport, 0, len(pending))
	skipped := 0
	for _, item := range pending {
		if _, ok := tracked[item.tmdbID]; ok {
			skipped++
			continue
		}
		out = append(out, item)
	}
	return out, skipped
}
