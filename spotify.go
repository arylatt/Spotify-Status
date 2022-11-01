package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/patrickmn/go-cache"
	"github.com/zmb3/spotify/v2"
	spotifyauth "github.com/zmb3/spotify/v2/auth"
	"golang.org/x/oauth2"
)

type FullTrack struct {
	*spotify.FullTrack

	spotifyStatus *SpotifyStatus
}

const (
	// SessionKey is the key used for retrieving session state
	SessionKey = "spotify-state"
)

func (ss *SpotifyStatus) authenticator() *spotifyauth.Authenticator {
	return spotifyauth.New(spotifyauth.WithRedirectURL(fmt.Sprintf("%s/callback", ss.redirectURI)), spotifyauth.WithScopes(spotifyauth.ScopeUserReadCurrentlyPlaying))
}

func (ss *SpotifyStatus) spotifyAuthLoginHandler(w http.ResponseWriter, r *http.Request) {
	state := uuid.NewString()
	auth := ss.authenticator()

	redirect := auth.AuthURL(state)

	session, _ := ss.sessionStore.Get(r, SessionKey)
	session.AddFlash(state)
	session.Save(r, w)

	http.Redirect(w, r, redirect, http.StatusTemporaryRedirect)
}

func (ss *SpotifyStatus) spotifyAuthCallbackHandler(w http.ResponseWriter, r *http.Request) {
	session, _ := ss.sessionStore.Get(r, SessionKey)
	if session.IsNew {
		http.Redirect(w, r, fmt.Sprintf("%s/login", ss.redirectURI), http.StatusTemporaryRedirect)
		return
	}

	flashes := session.Flashes()
	session.Save(r, w)

	if len(flashes) != 1 {
		http.Redirect(w, r, fmt.Sprintf("%s/login", ss.redirectURI), http.StatusTemporaryRedirect)
		return
	}

	state := flashes[0].(string)
	auth := ss.authenticator()
	token, err := auth.Token(r.Context(), state, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	client := spotify.New(auth.Client(r.Context(), token))
	user, err := client.CurrentUser(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tokenBytes, _ := json.Marshal(&token)
	tokenStr := base64.StdEncoding.EncodeToString(tokenBytes)

	ss.cache.Set(user.ID, tokenStr, cache.NoExpiration)

	http.Redirect(w, r, fmt.Sprintf("%s/%s", ss.redirectURI, user.ID), http.StatusTemporaryRedirect)
}

func (ss *SpotifyStatus) getTrack(w http.ResponseWriter, r *http.Request) (*FullTrack, bool) {
	tokenEntry, exists := ss.cache.Get(mux.Vars(r)["id"])
	if !exists {
		http.Error(w, "not found...", http.StatusNotFound)
		return nil, false
	}

	auth := ss.authenticator()
	token := &oauth2.Token{}
	tokenBytes, _ := base64.StdEncoding.DecodeString(tokenEntry.(string))

	json.Unmarshal(tokenBytes, &token)

	client := spotify.New(auth.Client(r.Context(), token))

	playing, err := client.PlayerCurrentlyPlaying(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return nil, false
	}

	track := &FullTrack{FullTrack: playing.Item, spotifyStatus: ss}
	if !playing.Playing {
		track.FullTrack = nil
	}

	w.Header().Add("cache-control", "private, no-store")

	trackID := "ruh roh, no track"
	if track.FullTrack != nil {
		trackID = track.ID.String()
	}

	w.Header().Set("etag", trackID)

	return track, true
}

func (ss *SpotifyStatus) spotifyBadgeHandler(w http.ResponseWriter, r *http.Request) {
	track, ok := ss.getTrack(w, r)
	if !ok {
		return
	}

	svg, err := track.Badge()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if track.FullTrack != nil {
		w.Header().Set("spotify-uri", track.ExternalURLs["spotify"])
	}

	w.Header().Set("content-type", "image/svg+xml")
	w.WriteHeader(http.StatusOK)
	w.Write(svg)
}

func (ss *SpotifyStatus) spotifyLinkHandler(w http.ResponseWriter, r *http.Request) {
	track, ok := ss.getTrack(w, r)
	if !ok {
		return
	}

	if track.FullTrack != nil {
		http.Redirect(w, r, track.ExternalURLs["spotify"], http.StatusTemporaryRedirect)
		return
	}

	http.Error(w, "nothing playing...", http.StatusTeapot)
}
