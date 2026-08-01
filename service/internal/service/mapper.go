package service

import (
	"mime"
	"path/filepath"
	"strings"
)

func trackNode(track map[string]any) node {
	id := entityID(track)
	album := firstObject(track, "album")
	artists := nestedList(track, "artists")
	audio := firstObject(track, "audioSpec")
	extension := stringValue(audio, "extension", "format", "container")
	if extension == "" {
		extension = strings.TrimPrefix(filepath.Ext(stringValue(audio, "path")), ".")
	}
	contentType := mime.TypeByExtension("." + extension)
	if contentType == "" {
		contentType = "audio/" + extension
	}
	durationMS := intValue(audio, "duration")
	if durationMS == 0 {
		durationMS = intValue(track, "duration")
	}
	artistID := ""
	if len(artists) > 0 {
		artistID = stringValue(object(artists[0]), "guid", "id")
	}
	coverID := extractCoverID(track)
	if coverID == "" {
		coverID = extractCoverID(album)
	}
	attr := map[string]any{
		"id":            id,
		"parent":        stringValue(album, "guid", "id"),
		"isDir":         false,
		"title":         stringValue(track, "title", "name"),
		"album":         stringValue(album, "name", "title"),
		"artist":        joinedNames(artists),
		"albumId":       stringValue(album, "guid", "id"),
		"artistId":      artistID,
		"coverArt":      coverID,
		"duration":      millisToSeconds(durationMS),
		"track":         intValue(track, "trackNo"),
		"discNumber":    intValue(track, "discNo"),
		"year":          intValue(track, "year"),
		"size":          intValue(audio, "size"),
		"bitRate":       intValue(audio, "bitrate") / 1000,
		"samplingRate":  intValue(audio, "sampleRate"),
		"bitDepth":      intValue(audio, "bitDepth"),
		"channelCount":  intValue(audio, "channel", "channels"),
		"suffix":        extension,
		"contentType":   contentType,
		"created":       unixToRFC3339(intValue(track, "createdAt")),
		"type":          "music",
		"isVideo":       false,
		"path":          stringValue(audio, "path"),
		"userRating":    0,
		"averageRating": 0,
	}
	if boolValue(track, "isFavorite") {
		attr["starred"] = unixToRFC3339(intValue(track, "updatedAt", "createdAt"))
	}
	return n("song", cleanAttr(attr))
}

func albumNode(album map[string]any) node {
	artists := nestedList(album, "artists")
	artistID := ""
	if len(artists) > 0 {
		artistID = stringValue(object(artists[0]), "guid", "id")
	}
	return n("album", cleanAttr(map[string]any{
		"id":        entityID(album),
		"name":      stringValue(album, "name", "title"),
		"artist":    joinedNames(artists),
		"artistId":  artistID,
		"coverArt":  extractCoverID(album),
		"songCount": intValue(album, "trackCount", "totalTrack"),
		"duration":  millisToSeconds(intValue(album, "duration")),
		"year":      intValue(album, "originalReleaseYear", "year"),
		"created":   unixToRFC3339(intValue(album, "createdAt")),
	}))
}

func artistNode(artist map[string]any) node {
	return n("artist", cleanAttr(map[string]any{
		"id":         entityID(artist),
		"name":       stringValue(artist, "name", "title"),
		"coverArt":   extractCoverID(artist),
		"albumCount": intValue(artist, "albumCount"),
	}))
}

func playlistNode(playlist map[string]any, username string) node {
	return n("playlist", cleanAttr(map[string]any{
		"id":        entityID(playlist),
		"name":      stringValue(playlist, "name", "title"),
		"owner":     username,
		"public":    false,
		"songCount": intValue(playlist, "trackCount", "songCount"),
		"duration":  millisToSeconds(intValue(playlist, "duration")),
		"created":   unixToRFC3339(intValue(playlist, "createdAt")),
		"changed":   unixToRFC3339(intValue(playlist, "updatedAt", "createdAt")),
		"coverArt":  extractCoverID(playlist),
	}))
}

func cleanAttr(input map[string]any) map[string]any {
	result := make(map[string]any, len(input))
	for key, value := range input {
		switch typed := value.(type) {
		case string:
			if typed != "" {
				result[key] = typed
			}
		case int64:
			if typed != 0 {
				result[key] = typed
			}
		default:
			if value != nil {
				result[key] = value
			}
		}
	}
	return result
}
