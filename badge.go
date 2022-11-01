package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/patrickmn/go-cache"
)

const (
	// CacheKeyNoTrack is the cache key used when for the SVG of no track playing
	CacheKeyNoTrack = "NoTrack"

	// ShieldsIOURLFormat is the URL format to be interpolated for fetching the SVG
	ShieldsIOURLFormat = "https://img.shields.io/static/v1?style=flat&logo=spotify&label=Now%%20Playing&message=%s&color=brightgreen"
)

func (ft *FullTrack) Badge() (svg []byte, err error) {
	if ft.FullTrack == nil {
		return ft.spotifyStatus.NoTrackBadge()
	}

	svgStr, exists := ft.spotifyStatus.cache.Get(ft.ID.String())
	if exists {
		return base64.StdEncoding.DecodeString(svgStr.(string))
	}

	artists := []string{}
	for _, artist := range ft.Artists {
		artists = append(artists, artist.Name)
	}

	msg := url.QueryEscape(fmt.Sprintf("%s by %s; from %s", ft.Name, strings.Join(artists, ", "), ft.Album.Name))
	svg, err = ft.spotifyStatus.BadgeBytes(msg)
	if err == nil {
		ft.spotifyStatus.cache.SetDefault(ft.ID.String(), base64.StdEncoding.EncodeToString(svg))
	}

	return
}

func (ss *SpotifyStatus) NoTrackBadge() (svg []byte, err error) {
	svgStr, exists := ss.cache.Get(CacheKeyNoTrack)
	if exists {
		return base64.StdEncoding.DecodeString(svgStr.(string))
	}

	svg, err = ss.BadgeBytes("nothing...")
	if err == nil {
		ss.cache.Set(CacheKeyNoTrack, base64.StdEncoding.EncodeToString(svg), cache.NoExpiration)
	}

	return
}

func (ss *SpotifyStatus) BadgeBytes(msg string) (svg []byte, err error) {
	resp, err := http.Get(fmt.Sprintf(ShieldsIOURLFormat, msg))
	if err != nil {
		return
	}

	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}
