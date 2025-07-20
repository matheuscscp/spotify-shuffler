package main

import (
	"context"
	"fmt"
	"math/rand/v2"

	"cloud.google.com/go/storage"
	"github.com/zmb3/spotify/v2"
)

type controller struct {
	spotify       *spotify.Client
	storage       *storage.BucketHandle
	forceRefresh  bool
	purgeEnqueued bool
	toEnqueue     int
}

func (c *controller) reconcile(ctx context.Context) error {
	if c.purgeEnqueued {
		if err := c.updateQueuedTracks(ctx, make(map[spotify.ID]struct{})); err != nil {
			return fmt.Errorf("failed to purge enqueued tracks: %w", err)
		}
		fmt.Println("All enqueued tracks have been purged.")
	}

	allPlayableTracks, err := c.getAllPlayableTracks(ctx)
	if err != nil {
		return fmt.Errorf("failed to get all playable tracks: %w", err)
	}
	fmt.Printf("Found %d playable tracks in total.\n", len(allPlayableTracks))

	queuedTracks, err := c.getQueuedTracks(ctx)
	if err != nil {
		return fmt.Errorf("failed to get queued tracks: %w", err)
	}
	fmt.Printf("Found %d queued tracks.\n", len(queuedTracks))

	playableTracks := getPlayableTracks(allPlayableTracks, queuedTracks)
	if len(playableTracks) > 0 {
		fmt.Printf("Found %d playable tracks that are not queued.\n", len(playableTracks))
	} else {
		fmt.Println("All tracks were played! Queued tracks will be reset.")
		playableTracks = allPlayableTracks
		queuedTracks = make(map[spotify.ID]struct{})
	}

	var toEnqueue []spotify.ID
	for range c.toEnqueue {
		var selected spotify.ID
		selected, playableTracks = selectTrackToEnqueue(playableTracks)
		toEnqueue = append(toEnqueue, selected)
		queuedTracks[selected] = struct{}{}
		if len(playableTracks) == 0 {
			fmt.Println("All tracks were played! Resetting playable tracks.")
			playableTracks = getPlayableTracks(allPlayableTracks, queuedTracks)
		}
	}

	for i, track := range toEnqueue {
		if err := c.spotify.QueueSong(ctx, track); err != nil {
			return fmt.Errorf("failed to enqueue track %s: %w", track, err)
		}
		n := i + 1
		if n%10 == 0 {
			fmt.Printf("Enqueued %d/%d tracks...\n", n, len(toEnqueue))
		}
	}

	if err := c.updateQueuedTracks(ctx, queuedTracks); err != nil {
		return fmt.Errorf("failed to update queued tracks: %w", err)
	}

	fmt.Printf("Enqueued %d tracks successfully.\n", len(toEnqueue))

	return nil
}

func getPlayableTracks(allPlayableTracks []spotify.ID, queuedTracks map[spotify.ID]struct{}) []spotify.ID {
	playableTracks := make([]spotify.ID, 0, len(allPlayableTracks))
	for _, track := range allPlayableTracks {
		if _, ok := queuedTracks[track]; !ok {
			playableTracks = append(playableTracks, track)
		}
	}
	return playableTracks
}

func selectTrackToEnqueue(playableTracks []spotify.ID) (spotify.ID, []spotify.ID) {
	idx := rand.IntN(len(playableTracks))
	track := playableTracks[idx]
	return track, append(playableTracks[:idx], playableTracks[idx+1:]...)
}
