package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"cloud.google.com/go/storage"
	"github.com/zmb3/spotify/v2"
)

const (
	storageKeyPlayableTracks = "playableTracks"
)

func (c *controller) getAllPlayableTracks(ctx context.Context) ([]spotify.ID, error) {
	playableTracksHandle := c.storage.Object(storageKeyPlayableTracks)

	attrs, err := playableTracksHandle.Attrs(ctx)
	if err != nil && !errors.Is(err, storage.ErrObjectNotExist) {
		return nil, fmt.Errorf("failed to get attributes of playable tracks object: %w", err)
	}
	if err == nil && time.Since(attrs.Updated) < 24*time.Hour && !c.forceRefresh {
		reader, err := playableTracksHandle.NewReader(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to create reader for playable tracks object: %w", err)
		}
		defer reader.Close()

		var playableTracks trackSet
		if err := json.NewDecoder(reader).Decode(&playableTracks); err != nil {
			return nil, fmt.Errorf("failed to receive playable tracks: %w", err)
		}
		return playableTracks.getSlice(), nil
	}

	fmt.Println("Refreshing playable tracks from Spotify...")
	playableTracks, err := c.listAllPlayableTracks(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed refreshing all playable tracks: %w", err)
	}

	writer := playableTracksHandle.NewWriter(ctx)
	defer writer.Close()
	if err := json.NewEncoder(writer).Encode(playableTracks); err != nil {
		return nil, fmt.Errorf("failed to upload playable tracks: %w", err)
	}

	return playableTracks.getSlice(), nil
}

func (c *controller) listAllPlayableTracks(ctx context.Context) (*trackSet, error) {
	ids := make(map[spotify.ID]struct{})

	const step = 50
	for offset := 0; ; offset += step {
		oneBasedFrom := offset + 1
		oneBasedTo := offset + step
		fmt.Printf("Fetching tracks from %d to %d...\n", oneBasedFrom, oneBasedTo)
		page, err := c.spotify.CurrentUsersTracks(ctx, spotify.Limit(step), spotify.Offset(offset))
		if err != nil {
			return nil, fmt.Errorf("failed to get current user's tracks, offset %d: %w", offset, err)
		}
		for _, track := range page.Tracks {
			if track.IsPlayable != nil && *track.IsPlayable {
				ids[track.ID] = struct{}{}
			}
		}
		if len(page.Tracks) < step {
			break
		}
	}

	return &trackSet{ids}, nil
}
