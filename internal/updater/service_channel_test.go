package updater

import "testing"

func TestChangelogSourcesForChannel_DefaultsToUpstream(t *testing.T) {
	oldEntwareRepoURL := entwareRepoURL
	oldReleaseBaseURL := releaseBaseURL
	entwareRepoURL = "http://example.test"
	releaseBaseURL = ""
	t.Cleanup(func() {
		entwareRepoURL = oldEntwareRepoURL
		releaseBaseURL = oldReleaseBaseURL
	})

	cases := []struct {
		channel          string
		wantPrimaryURL   string
		wantSecondaryURL string
	}{
		{channel: "stable", wantPrimaryURL: "http://example.test/CHANGELOG.md"},
		{channel: "develop", wantPrimaryURL: "http://example.test/develop/CHANGELOG.md"},
		{channel: "", wantPrimaryURL: "http://example.test/CHANGELOG.md"},
	}

	for _, tc := range cases {
		primary, secondary := changelogSourcesForChannel(tc.channel)
		if primary != tc.wantPrimaryURL || secondary != tc.wantSecondaryURL {
			t.Fatalf("changelogSourcesForChannel(%q) = (%q, %q), want (%q, %q)", tc.channel, primary, secondary, tc.wantPrimaryURL, tc.wantSecondaryURL)
		}
	}
}

func TestChangelogSourcesForChannel_PrefersForkReleaseBase(t *testing.T) {
	oldEntwareRepoURL := entwareRepoURL
	oldReleaseBaseURL := releaseBaseURL
	entwareRepoURL = "http://example.test"
	releaseBaseURL = "https://github.example/releases/download/iq-latest"
	t.Cleanup(func() {
		entwareRepoURL = oldEntwareRepoURL
		releaseBaseURL = oldReleaseBaseURL
	})

	primary, secondary := changelogSourcesForChannel("stable")
	if primary != "https://github.example/releases/download/iq-latest/CHANGELOG.md" {
		t.Fatalf("stable primary = %q", primary)
	}
	if secondary != "http://example.test/CHANGELOG.md" {
		t.Fatalf("stable secondary = %q", secondary)
	}

	primary, secondary = changelogSourcesForChannel("develop")
	if primary != "https://github.example/releases/download/iq-latest/CHANGELOG.md" {
		t.Fatalf("develop primary = %q", primary)
	}
	if secondary != "http://example.test/develop/CHANGELOG.md" {
		t.Fatalf("develop secondary = %q", secondary)
	}
}
