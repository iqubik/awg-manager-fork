package updater

import (
	"errors"
	"time"
)

// UpdateInfo holds the result of an update check.
type UpdateInfo struct {
	Available      bool      `json:"available"`
	CurrentVersion string    `json:"currentVersion"`
	LatestVersion  string    `json:"latestVersion,omitempty"`
	DownloadURL    string    `json:"downloadUrl,omitempty"`
	CheckedAt      time.Time `json:"checkedAt"`
	Checking       bool      `json:"checking"`
	Error          string    `json:"error,omitempty"`
	Warning        string    `json:"warning,omitempty"`
	Channel        string    `json:"channel,omitempty"`
	Source         string    `json:"source,omitempty"`    // "release" | "entware"
	SourceURL      string    `json:"sourceUrl,omitempty"` // VERSION or Packages.gz URL
}

var ErrUpgradeInProgress = errors.New("upgrade already in progress")
