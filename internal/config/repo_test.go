package config

import "testing"

func TestParseRemoteURL(t *testing.T) {
	cases := []struct {
		url, owner, name string
		wantErr          bool
	}{
		{"git@github.com:octo/hello.git", "octo", "hello", false},
		{"https://github.com/octo/hello.git", "octo", "hello", false},
		{"https://github.com/octo/hello", "octo", "hello", false},
		{"ssh://git@github.com/octo/hello.git", "octo", "hello", false},
		{"not a url", "", "", true},
	}
	for _, c := range cases {
		repo, err := ParseRemoteURL(c.url)
		if c.wantErr {
			if err == nil {
				t.Fatalf("ParseRemoteURL(%q): expected error", c.url)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ParseRemoteURL(%q): %v", c.url, err)
		}
		if repo.Owner != c.owner || repo.Name != c.name {
			t.Fatalf("ParseRemoteURL(%q) = %s/%s, want %s/%s", c.url, repo.Owner, repo.Name, c.owner, c.name)
		}
	}
}

func TestParseOwnerName(t *testing.T) {
	repo, err := ParseOwnerName("octo/hello")
	if err != nil || repo.Owner != "octo" || repo.Name != "hello" {
		t.Fatalf("ParseOwnerName: got %+v err %v", repo, err)
	}
	if _, err := ParseOwnerName("bogus"); err == nil {
		t.Fatal("ParseOwnerName(bogus): expected error")
	}
}
