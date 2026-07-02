package github

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/alexkalderimis/pr-tree/internal/config"
)

func TestFetchOpenPRs(t *testing.T) {
	const resp = `{"data":{"repository":{"defaultBranchRef":{"name":"main"},
	  "pullRequests":{"pageInfo":{"hasNextPage":false,"endCursor":null},"nodes":[
	    {"number":1,"id":"PR_kwROOT","title":"ROOT","body":"","isDraft":false,"state":"OPEN","reviewDecision":"APPROVED",
	     "author":{"login":"alice"},"baseRefName":"main","headRefName":"a","headRefOid":"deadbeef",
	     "latestReviews":{"nodes":[{"state":"APPROVED","author":{"login":"bob","id":"U_bob"}},{"state":"COMMENTED","author":{"login":"carol","id":"U_carol"}}]},
	     "reviewRequests":{"nodes":[{"requestedReviewer":{"login":"bob"}},{"requestedReviewer":{"login":"bob"}},{"requestedReviewer":{}}]}},
	    {"number":2,"title":"STEM","body":"upstream: #1","isDraft":true,"state":"OPEN",
	     "author":{"login":"bob"},"baseRefName":"a","headRefName":"b","headRefOid":"cafef00d",
	     "reviewRequests":{"nodes":[]}}
	  ]}}}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if !strings.Contains(string(body), "pullRequests") {
			t.Errorf("query missing pullRequests: %s", body)
		}
		if !strings.Contains(string(body), "headRefOid") {
			t.Errorf("query missing headRefOid: %s", body)
		}
		if !strings.Contains(string(body), "latestReviews") {
			t.Errorf("query missing latestReviews: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, resp)
	}))
	defer srv.Close()

	c := &Client{token: "t", endpoint: srv.URL, httpClient: srv.Client()}
	prs, defaultBranch, err := c.FetchOpenPRs(context.Background(), config.Repo{Owner: "o", Name: "n"})
	if err != nil {
		t.Fatalf("FetchOpenPRs: %v", err)
	}
	if defaultBranch != "main" {
		t.Fatalf("defaultBranch = %q, want main", defaultBranch)
	}
	if len(prs) != 2 {
		t.Fatalf("got %d PRs, want 2", len(prs))
	}
	if prs[0].State != "OPEN" || prs[0].Author != "alice" || prs[0].Reviewers[0] != "bob" {
		t.Fatalf("PR0 decoded wrong: %+v", prs[0])
	}
	if len(prs[0].Reviewers) != 1 { // duplicate "bob" and empty-login reviewer dropped
		t.Fatalf("PR0 reviewers not deduped: %+v", prs[0].Reviewers)
	}
	if prs[0].ReviewDecision != "APPROVED" {
		t.Fatalf("PR0 reviewDecision = %q, want APPROVED", prs[0].ReviewDecision)
	}
	if prs[0].HeadOID != "deadbeef" {
		t.Fatalf("PR0 HeadOID = %q, want deadbeef", prs[0].HeadOID)
	}
	if prs[1].State != "DRAFT" { // isDraft true -> DRAFT
		t.Fatalf("PR1 state = %q, want DRAFT", prs[1].State)
	}
	if prs[0].NodeID != "PR_kwROOT" {
		t.Fatalf("PR0 NodeID = %q, want PR_kwROOT", prs[0].NodeID)
	}
	if len(prs[0].Approvers) != 1 || prs[0].Approvers[0].Login != "bob" || prs[0].Approvers[0].ID != "U_bob" {
		t.Fatalf("PR0 approvers wrong: %+v", prs[0].Approvers) // only APPROVED, not COMMENTED
	}
}

func TestViewer(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"data":{"viewer":{"login":"alice"}}}`)
	}))
	defer srv.Close()

	c := &Client{token: "t", endpoint: srv.URL, httpClient: srv.Client()}
	login, err := c.Viewer(context.Background())
	if err != nil || login != "alice" {
		t.Fatalf("Viewer = %q, err %v", login, err)
	}
}

func TestDo_GraphQLError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `{"errors":[{"message":"bad query"}]}`)
	}))
	defer srv.Close()

	c := &Client{token: "t", endpoint: srv.URL, httpClient: srv.Client()}
	var out json.RawMessage
	err := c.do(context.Background(), "query{}", nil, &out)
	if err == nil || !strings.Contains(err.Error(), "bad query") {
		t.Fatalf("expected GraphQL error, got %v", err)
	}
}

func TestRequestReviews(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		io.WriteString(w, `{"data":{"requestReviews":{"clientMutationId":null}}}`)
	}))
	defer srv.Close()

	c := &Client{token: "t", endpoint: srv.URL, httpClient: srv.Client()}
	err := c.RequestReviews(context.Background(), "PR_node", []string{"U_bob", "U_foo"})
	if err != nil {
		t.Fatalf("RequestReviews: %v", err)
	}
	if !strings.Contains(gotBody, "requestReviews") {
		t.Errorf("body missing requestReviews mutation: %s", gotBody)
	}
	if !strings.Contains(gotBody, "PR_node") || !strings.Contains(gotBody, "U_bob") {
		t.Errorf("body missing variables: %s", gotBody)
	}
}

func TestFetchPRsByNumber(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if strings.Contains(string(body), `"number":1`) {
			io.WriteString(w, `{"data":{"repository":{"pullRequest":{
				"number":1,"title":"MERGED ROOT","body":"","isDraft":false,"state":"MERGED",
				"author":{"login":"alice"},"baseRefName":"main","headRefName":"a",
				"reviewRequests":{"nodes":[]}}}}}`)
			return
		}
		// number 2: not found / inaccessible -> null pullRequest, must be skipped
		io.WriteString(w, `{"data":{"repository":{"pullRequest":null}}}`)
	}))
	defer srv.Close()

	c := &Client{token: "t", endpoint: srv.URL, httpClient: srv.Client()}
	prs, err := c.FetchPRsByNumber(context.Background(), config.Repo{Owner: "o", Name: "n"}, []int{1, 2})
	if err != nil {
		t.Fatalf("FetchPRsByNumber: %v", err)
	}
	if len(prs) != 1 {
		t.Fatalf("got %d PRs, want 1 (the nil one skipped)", len(prs))
	}
	if prs[0].Number != 1 || prs[0].State != "MERGED" {
		t.Fatalf("PR decoded wrong: %+v", prs[0])
	}
}

func TestUpdatePullRequestBody(t *testing.T) {
	var gotBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		io.WriteString(w, `{"data":{"updatePullRequest":{"pullRequest":{"number":7}}}}`)
	}))
	defer srv.Close()

	c := &Client{token: "t", endpoint: srv.URL, httpClient: srv.Client()}
	err := c.UpdatePullRequestBody(context.Background(), "PR_node", "new body")
	if err != nil {
		t.Fatalf("UpdatePullRequestBody: %v", err)
	}
	if !strings.Contains(gotBody, "updatePullRequest") {
		t.Errorf("body missing updatePullRequest mutation: %s", gotBody)
	}
	if !strings.Contains(gotBody, "PR_node") || !strings.Contains(gotBody, "new body") {
		t.Errorf("body missing variables: %s", gotBody)
	}
}
