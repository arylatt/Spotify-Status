package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
	"github.com/gorilla/sessions"
	"github.com/patrickmn/go-cache"
	"github.com/spf13/viper"
)

const (
	// ProjectURL is the link to the GitHub repo
	ProjectURL = "https://github.com/arylatt/Spotify-Status"
)

func init() {
	viper.SetEnvPrefix("spotifystatus")
	viper.AutomaticEnv()

	viper.SetDefault("cache_expiry", "30m")
	viper.SetDefault("cache_purge", "60m")
	viper.SetDefault("cache_persist", "60s")
	viper.SetDefault("redirect_uri", "http://127.0.0.1:3000")
	viper.SetDefault("listen_addr", "0.0.0.0:3000")
}

type SpotifyStatus struct {
	ctx          context.Context
	cache        *cache.Cache
	cacheFile    string
	httpServer   *http.Server
	redirectURI  string
	router       *mux.Router
	sessionStore *sessions.CookieStore
}

func main() {
	router := mux.NewRouter()

	cacheItems := map[string]cache.Item{}
	if cacheFile := viper.GetString("cache_file"); cacheFile != "" {
		// TODO: logging, warn.
		cacheItems, _ = LoadCache(cacheFile)
	}

	_spotifyStatus := &SpotifyStatus{
		ctx:       context.Background(),
		cache:     cache.NewFrom(viper.GetDuration("cache_expiry"), viper.GetDuration("cache_purge"), cacheItems),
		cacheFile: viper.GetString("cache_file"),
		httpServer: &http.Server{
			Addr:    viper.GetString("listen_addr"),
			Handler: router,
		},
		redirectURI:  viper.GetString("redirect_uri"),
		router:       router,
		sessionStore: sessions.NewCookieStore(securecookie.GenerateRandomKey(32), securecookie.GenerateRandomKey(32)),
	}

	router.Path("/").HandlerFunc(rootHandler)
	router.Path("/callback").Name("auth_callback").HandlerFunc(_spotifyStatus.spotifyAuthCallbackHandler)
	router.Path("/login").HandlerFunc(_spotifyStatus.spotifyAuthLoginHandler)
	router.Path("/{id}").HandlerFunc(_spotifyStatus.spotifyBadgeHandler)
	router.Path("/{id}/link").HandlerFunc(_spotifyStatus.spotifyLinkHandler)

	go _spotifyStatus.cacheSaver()

	log.Fatal(_spotifyStatus.httpServer.ListenAndServe())
}

func (ss *SpotifyStatus) SaveCache(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}

	defer file.Close()

	return json.NewEncoder(file).Encode(ss.cache.Items())
}

func LoadCache(path string) (map[string]cache.Item, error) {
	items := map[string]cache.Item{}
	file, err := os.Open(path)
	if err != nil {
		return items, err
	}

	defer file.Close()

	err = json.NewDecoder(file).Decode(&items)

	return items, err
}

func (ss *SpotifyStatus) cacheSaver() {
	cachePersist := viper.GetDuration("cache_persist")
	if cachePersist == 0 || ss.cacheFile == "" {
		return
	}

	for {
		select {
		case <-ss.ctx.Done():
			ss.SaveCache(ss.cacheFile)
			return
		case <-time.After(cachePersist):
			ss.SaveCache(ss.cacheFile)
		}
	}
}

func rootHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, ProjectURL, http.StatusPermanentRedirect)
}
