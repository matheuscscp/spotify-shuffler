package main

import "github.com/zmb3/spotify/v2"

type trackSet struct {
	IDs map[spotify.ID]struct{} `json:"ids"`
}

func (t *trackSet) getSlice() []spotify.ID {
	ids := make([]spotify.ID, 0, len(t.IDs))
	for id := range t.IDs {
		ids = append(ids, id)
	}
	return ids
}

func (t *trackSet) getMap() map[spotify.ID]struct{} {
	if t.IDs == nil {
		return make(map[spotify.ID]struct{})
	}
	return t.IDs
}
