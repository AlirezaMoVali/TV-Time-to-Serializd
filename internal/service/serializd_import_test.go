package service

import (
	"slices"
	"testing"

	"github.com/alireza/tvtime2serializd/internal/serializd"
	"github.com/alireza/tvtime2serializd/internal/tvtime"
)

type importStub struct {
	show *serializd.Show

	watchlist         []int
	currentlyWatching bool
	watchedSeasons    []int
	dropped           bool
	episodeLogs       []episodeLogCall
}

type episodeLogCall struct {
	seasonID int
	numbers  []int
}

func (s *importStub) GetShow(string, int) (*serializd.Show, error) {
	return s.show, nil
}

func (s *importStub) AddWatchlist(_ string, _ int, seasonIDs []int) error {
	s.watchlist = append([]int(nil), seasonIDs...)
	return nil
}

func (s *importStub) AddCurrentlyWatching(string, int) (*serializd.MessageResponse, error) {
	s.currentlyWatching = true
	return &serializd.MessageResponse{}, nil
}

func (s *importStub) AddWatched(_ string, _ int, seasonIDs []int, _ bool) (*serializd.MessageResponse, error) {
	s.watchedSeasons = append([]int(nil), seasonIDs...)
	return &serializd.MessageResponse{}, nil
}

func (s *importStub) AddDropped(string, int) (*serializd.MessageResponse, error) {
	s.dropped = true
	return &serializd.MessageResponse{}, nil
}

func (s *importStub) AddEpisodeLog(_ string, _, seasonID int, episodeNumbers []int, _ bool) error {
	s.episodeLogs = append(s.episodeLogs, episodeLogCall{
		seasonID: seasonID,
		numbers:  append([]int(nil), episodeNumbers...),
	})
	return nil
}

func testShow() *serializd.Show {
	return &serializd.Show{
		Seasons: []serializd.Season{
			{SeasonID: 1, SeasonNumber: 1, Name: "Season 1"},
			{SeasonID: 2, SeasonNumber: 2, Name: "Season 2"},
			{SeasonID: 99, SeasonNumber: 0, Name: "Specials"},
		},
	}
}

func TestImportTVTimeShow_NotStartedYet(t *testing.T) {
	stub := &importStub{show: testShow()}
	show := tvtime.ExportShow{Status: "not_started_yet"}

	if err := ImportTVTimeShow(stub, "token", 123, show); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(stub.watchlist, []int{1, 2}) {
		t.Fatalf("watchlist = %v, want [1 2]", stub.watchlist)
	}
	if stub.currentlyWatching || stub.dropped || len(stub.watchedSeasons) > 0 || len(stub.episodeLogs) > 0 {
		t.Fatalf("unexpected side effects: %+v", stub)
	}
}

func TestImportTVTimeShow_UpToDate(t *testing.T) {
	stub := &importStub{show: testShow()}
	show := tvtime.ExportShow{
		Status: "up_to_date",
		Seasons: []tvtime.ExportSeason{
			{Number: 0, IsSpecials: true, Episodes: []tvtime.ExportEpisode{
				{Number: 1, IsWatched: true},
			}},
		},
	}

	if err := ImportTVTimeShow(stub, "token", 123, show); err != nil {
		t.Fatal(err)
	}
	if len(stub.watchlist) != 0 {
		t.Fatalf("watchlist = %v, want none", stub.watchlist)
	}
	if !slices.Equal(stub.watchedSeasons, []int{1, 2}) {
		t.Fatalf("watched seasons = %v, want [1 2]", stub.watchedSeasons)
	}
	if len(stub.episodeLogs) != 1 || stub.episodeLogs[0].seasonID != 99 {
		t.Fatalf("special episode logs = %+v, want specials only", stub.episodeLogs)
	}
}

func TestImportTVTimeShow_Continuing(t *testing.T) {
	stub := &importStub{show: testShow()}
	show := tvtime.ExportShow{
		Status: "continuing",
		Seasons: []tvtime.ExportSeason{
			{Number: 1, Episodes: []tvtime.ExportEpisode{
				{Number: 1, IsWatched: true},
				{Number: 2, IsWatched: false},
			}},
			{Number: 0, IsSpecials: true, Episodes: []tvtime.ExportEpisode{
				{Number: 1, IsWatched: true},
			}},
		},
	}

	if err := ImportTVTimeShow(stub, "token", 123, show); err != nil {
		t.Fatal(err)
	}
	if !stub.currentlyWatching {
		t.Fatal("expected currently watching")
	}
	if !slices.Equal(stub.watchlist, []int{1, 2}) {
		t.Fatalf("watchlist = %v, want partial s1 + unwatched s2", stub.watchlist)
	}
	if len(stub.watchedSeasons) != 0 {
		t.Fatalf("watched seasons = %v, want none for partial progress", stub.watchedSeasons)
	}
	if len(stub.episodeLogs) != 2 {
		t.Fatalf("episode logs = %+v, want 2 seasons", stub.episodeLogs)
	}
	if stub.episodeLogs[0].seasonID != 1 || len(stub.episodeLogs[0].numbers) != 1 || stub.episodeLogs[0].numbers[0] != 1 {
		t.Fatalf("regular season log = %+v", stub.episodeLogs[0])
	}
}

func TestImportTVTimeShow_Continuing_FullyWatchedSeason(t *testing.T) {
	stub := &importStub{show: testShow()}
	show := tvtime.ExportShow{
		Status: "continuing",
		Seasons: []tvtime.ExportSeason{
			{Number: 1, Episodes: []tvtime.ExportEpisode{
				{Number: 1, IsWatched: true},
				{Number: 2, IsWatched: true},
			}},
		},
	}

	if err := ImportTVTimeShow(stub, "token", 123, show); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(stub.watchedSeasons, []int{1}) {
		t.Fatalf("watched seasons = %v, want [1]", stub.watchedSeasons)
	}
	if !slices.Equal(stub.watchlist, []int{2}) {
		t.Fatalf("watchlist = %v, want unwatched s2 only", stub.watchlist)
	}
	if len(stub.episodeLogs) != 1 || stub.episodeLogs[0].seasonID != 1 {
		t.Fatalf("episode logs = %+v, want s1 episodes logged", stub.episodeLogs)
	}
	if !slices.Equal(stub.episodeLogs[0].numbers, []int{1, 2}) {
		t.Fatalf("episode log numbers = %v, want [1 2]", stub.episodeLogs[0].numbers)
	}
}

func TestImportTVTimeShow_Stopped(t *testing.T) {
	stub := &importStub{show: testShow()}
	show := tvtime.ExportShow{
		Status: "stopped",
		Seasons: []tvtime.ExportSeason{
			{Number: 1, Episodes: []tvtime.ExportEpisode{
				{Number: 1, IsWatched: true},
				{Number: 2, IsWatched: true},
			}},
		},
	}

	if err := ImportTVTimeShow(stub, "token", 123, show); err != nil {
		t.Fatal(err)
	}
	if !stub.dropped {
		t.Fatal("expected dropped")
	}
	if len(stub.watchlist) != 0 {
		t.Fatalf("watchlist = %v, want none", stub.watchlist)
	}
	if !slices.Equal(stub.watchedSeasons, []int{1}) {
		t.Fatalf("watched seasons = %v, want [1]", stub.watchedSeasons)
	}
	if len(stub.episodeLogs) != 1 || !slices.Equal(stub.episodeLogs[0].numbers, []int{1, 2}) {
		t.Fatalf("episode logs = %+v, want s1 [1 2]", stub.episodeLogs)
	}
}

func TestImportTVTimeShow_Stopped_PartialSeason(t *testing.T) {
	stub := &importStub{show: testShow()}
	show := tvtime.ExportShow{
		Status: "stopped",
		Seasons: []tvtime.ExportSeason{
			{Number: 1, Episodes: []tvtime.ExportEpisode{
				{Number: 1, IsWatched: true},
				{Number: 2, IsWatched: false},
			}},
		},
	}

	if err := ImportTVTimeShow(stub, "token", 123, show); err != nil {
		t.Fatal(err)
	}
	if len(stub.watchlist) != 0 {
		t.Fatalf("watchlist = %v, want none", stub.watchlist)
	}
	if len(stub.watchedSeasons) != 0 {
		t.Fatalf("watched seasons = %v, want none for partial", stub.watchedSeasons)
	}
	if len(stub.episodeLogs) != 1 || stub.episodeLogs[0].seasonID != 1 {
		t.Fatalf("episode logs = %+v", stub.episodeLogs)
	}
}
