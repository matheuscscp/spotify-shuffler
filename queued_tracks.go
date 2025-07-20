package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"cloud.google.com/go/storage"
	"github.com/zmb3/spotify/v2"
)

const (
	storageKeyQueuedTracks = "queuedTracks"
)

func (c *controller) getQueuedTracks(ctx context.Context) (map[spotify.ID]struct{}, error) {
	queuedTracksHandle := c.storage.Object(storageKeyQueuedTracks)

	reader, err := queuedTracksHandle.NewReader(ctx)
	if err != nil {
		if errors.Is(err, storage.ErrObjectNotExist) {
			var emptySet trackSet
			return emptySet.getMap(), nil
		}
		return nil, err
	}
	defer reader.Close()

	var queuedTracks trackSet
	if err := json.NewDecoder(reader).Decode(&queuedTracks); err != nil {
		return nil, fmt.Errorf("failed to decode queued tracks: %w", err)
	}

	return queuedTracks.getMap(), nil
}

func (c *controller) updateQueuedTracks(ctx context.Context, queuedTracks map[spotify.ID]struct{}) error {
	queuedTracksHandle := c.storage.Object(storageKeyQueuedTracks)

	writer := queuedTracksHandle.NewWriter(ctx)
	defer writer.Close()

	if err := json.NewEncoder(writer).Encode(trackSet{queuedTracks}); err != nil {
		return fmt.Errorf("failed to encode queued tracks: %w", err)
	}

	return nil
}
