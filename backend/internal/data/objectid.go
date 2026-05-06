package data

import (
	"net/url"
	"regexp"
	"strings"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

var objectIDPattern = regexp.MustCompile(`[a-fA-F0-9]{24}`)

func normalizeObjectIDHex(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return ""
	}

	if _, err := primitive.ObjectIDFromHex(value); err == nil {
		return value
	}

	if decoded, err := url.QueryUnescape(value); err == nil && strings.TrimSpace(decoded) != "" {
		value = strings.TrimSpace(decoded)
		if _, err := primitive.ObjectIDFromHex(value); err == nil {
			return value
		}
	}

	match := objectIDPattern.FindString(value)
	if match != "" {
		return match
	}

	return strings.TrimSpace(raw)
}

func objectIDFromAny(raw string) (primitive.ObjectID, error) {
	return primitive.ObjectIDFromHex(normalizeObjectIDHex(raw))
}
