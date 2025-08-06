package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"cloud.google.com/go/storage"
	"github.com/pkg/browser"
	"github.com/spf13/pflag"
	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"go.yaml.in/yaml/v2"

	"github.com/matheuscscp/serve-for-me/serveforme"
)

const (
	serverURL   = "https://serve-for-me-r2enu3lpxa-nw.a.run.app"
	serverPath  = "/spotify-shuffler"
	redirectURI = serverURL + serverPath

	envVarClientID     = "SPOTIFY_ID"
	envVarClientSecret = "SPOTIFY_SECRET"

	storageBucketName = "matheuscscp-spotify-shuffler"
)

var requiredScopes = []string{
	spotifyauth.ScopePlaylistReadPrivate,
	spotifyauth.ScopePlaylistReadCollaborative,
	spotifyauth.ScopeUserLibraryRead,
	spotifyauth.ScopeUserReadPrivate,
	spotifyauth.ScopeUserReadCurrentlyPlaying,
	spotifyauth.ScopeUserReadPlaybackState,
	spotifyauth.ScopeUserModifyPlaybackState,
	spotifyauth.ScopeUserReadRecentlyPlayed,
	spotifyauth.ScopeUserTopRead,
	spotifyauth.ScopeStreaming,
}

func main() {
	ctx := setupSignalHandler()

	var (
		credsPath     string
		forceRefresh  bool
		purgeEnqueued bool
		toEnqueue     int
	)

	flags := pflag.NewFlagSet(os.Args[0], pflag.ContinueOnError)

	flags.StringVar(&credsPath, "creds-path", "./creds.yaml", "Path to the credentials YAML file")
	flags.BoolVarP(&forceRefresh, "force-refresh", "f", false, "Force refresh of playable tracks from Spotify")
	flags.BoolVarP(&purgeEnqueued, "purge-enqueued", "p", false, "Purge all enqueued tracks before starting")
	flags.IntVarP(&toEnqueue, "to-enqueue", "n", 10, "Number of tracks to enqueue")

	if err := flags.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, pflag.ErrHelp) {
			return
		}
		fmt.Fprintf(os.Stderr, "error parsing flags: %v\n", err)
		os.Exit(1)
	}

	if _, ok := os.LookupEnv(envVarClientID); !ok {
		b, err := os.ReadFile(credsPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading credentials file: %v\n", err)
			os.Exit(1)
		}
		var creds struct {
			ClientID     string `yaml:"clientID"`
			ClientSecret string `yaml:"clientSecret"`
		}
		if err := yaml.Unmarshal(b, &creds); err != nil {
			fmt.Fprintf(os.Stderr, "error unmarshalling credentials: %v\n", err)
			os.Exit(1)
		}
		os.Setenv(envVarClientID, creds.ClientID)
		os.Setenv(envVarClientSecret, creds.ClientSecret)
	}

	state := generateState()

	auth := spotifyauth.New(
		spotifyauth.WithRedirectURL(redirectURI),
		spotifyauth.WithScopes(requiredScopes...))

	var spotifyClient *spotify.Client
	clientCreated := make(chan struct{})

	authCtx, cancelAuthCtx := context.WithCancel(ctx)
	defer cancelAuthCtx()
	authServerStarted := make(chan struct{})

	authHandler := map[string]http.Handler{
		serverPath: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if spotifyClient != nil {
				return
			}
			token, err := auth.Token(r.Context(), state, r)
			if err != nil {
				http.Error(w, "Couldn't get token", http.StatusNotFound)
				return
			}
			httpClient := auth.Client(r.Context(), token)
			spotifyClient = spotify.New(httpClient, spotify.WithRetry(true))
			fmt.Fprint(w, authSuccessPage)
			close(clientCreated)
		}),
	}

	var opts []serveforme.ClientOption
	if os.Getenv("GITHUB_ACTIONS") == "true" {
		opts = append(opts, serveforme.WithGitHubActions())
	} else {
		opts = append(opts, serveforme.WithGoogleIDToken())
	}

	authServerClosed := make(chan struct{})
	go func() {
		defer close(authServerClosed)
		if err := serveforme.ServeForMe(authCtx, serverURL, authServerStarted, authHandler, opts...); err != nil {
			fmt.Fprintf(os.Stderr, "error starting auth server: %v\n", err)
			os.Exit(1)
		}
	}()

	select {
	case <-authServerStarted:
	case <-ctx.Done():
		fmt.Fprintf(os.Stderr, "authentication server did not start: %v\n", ctx.Err())
		os.Exit(1)
	}

	url := auth.AuthURL(state)
	if err := browser.OpenURL(url); err != nil {
		fmt.Printf("Please open the following URL in your browser to authenticate:\n\n%s\n", url)
	}

	<-clientCreated
	fmt.Print("\nAuthentication successful!\n\n")
	cancelAuthCtx()

	storageClient, err := storage.NewClient(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating storage client: %v\n", err)
		os.Exit(1)
	}
	defer storageClient.Close()

	ctrl := &controller{
		spotify:       spotifyClient,
		storage:       storageClient.Bucket(storageBucketName),
		forceRefresh:  forceRefresh,
		purgeEnqueued: purgeEnqueued,
		toEnqueue:     toEnqueue,
	}

	if err := ctrl.reconcile(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

func generateState() string {
	t1 := generateRandomTripleDigits()
	t2 := generateRandomTripleDigits()
	state := fmt.Sprintf("%s-%s", t1, t2)
	return state
}

func generateRandomTripleDigits() string {
	var digits strings.Builder
	for range 3 {
		n, err := rand.Int(rand.Reader, big.NewInt(10))
		if err != nil {
			fmt.Fprintf(os.Stderr, "error generating random number: %v\n", err)
			os.Exit(1)
		}
		digits.WriteString(n.String())
	}
	return digits.String()
}

func setupSignalHandler() context.Context {
	ctx, cancel := context.WithCancel(context.Background())

	c := make(chan os.Signal, 2)
	signal.Notify(c, syscall.SIGTERM, os.Interrupt)
	go func() {
		<-c
		cancel()
		<-c
		os.Exit(1) // second signal. Exit directly.
	}()

	return ctx
}
