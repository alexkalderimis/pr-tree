package github

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/alexkalderimis/pr-tree/internal/config"
	"github.com/alexkalderimis/pr-tree/internal/tree"
)

const defaultEndpoint = "https://api.github.com/graphql"

// Client queries the GitHub GraphQL API.
type Client struct {
	token      string
	endpoint   string
	httpClient *http.Client
}

// New returns a Client authenticated with the given token.
func New(token string) *Client {
	return &Client{token: token, endpoint: defaultEndpoint, httpClient: http.DefaultClient}
}

// do executes a GraphQL query and decodes the `data` field into out.
func (c *Client) do(ctx context.Context, query string, vars map[string]any, out any) error {
	payload, err := json.Marshal(map[string]any{"query": query, "variables": vars})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("github API returned HTTP %d", resp.StatusCode)
	}

	var envelope struct {
		Data   json.RawMessage `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&envelope); err != nil {
		return err
	}
	if len(envelope.Errors) > 0 {
		return fmt.Errorf("github GraphQL error: %s", envelope.Errors[0].Message)
	}
	return json.Unmarshal(envelope.Data, out)
}

// prNode mirrors the GraphQL pull request shape.
type prNode struct {
	Number         int    `json:"number"`
	Title          string `json:"title"`
	Body           string `json:"body"`
	IsDraft        bool   `json:"isDraft"`
	State          string `json:"state"`
	ReviewDecision string `json:"reviewDecision"`
	Author         struct {
		Login string `json:"login"`
	} `json:"author"`
	BaseRefName   string `json:"baseRefName"`
	HeadRefName   string `json:"headRefName"`
	HeadRefOid    string `json:"headRefOid"`
	ID            string `json:"id"`
	LatestReviews struct {
		Nodes []struct {
			State  string `json:"state"`
			Author struct {
				Login string `json:"login"`
				ID    string `json:"id"`
			} `json:"author"`
		} `json:"nodes"`
	} `json:"latestReviews"`
	ReviewRequests struct {
		Nodes []struct {
			RequestedReviewer struct {
				Login string `json:"login"`
			} `json:"requestedReviewer"`
		} `json:"nodes"`
	} `json:"reviewRequests"`
}

// toPR converts a GraphQL node into the domain model, deriving DRAFT state.
func (n prNode) toPR() tree.PullRequest {
	state := tree.State(n.State)
	if n.State == "OPEN" && n.IsDraft {
		state = tree.StateDraft
	}
	// Dedupe reviewer logins: GitHub can return the same reviewer more than
	// once (e.g. a re-requested review), and non-User reviewers (teams) yield
	// an empty login via the `... on User` fragment.
	var reviewers []string
	seen := make(map[string]bool)
	for _, rr := range n.ReviewRequests.Nodes {
		login := rr.RequestedReviewer.Login
		if login != "" && !seen[login] {
			seen[login] = true
			reviewers = append(reviewers, login)
		}
	}
	var approvers []tree.Approver
	for _, rv := range n.LatestReviews.Nodes {
		if rv.State == "APPROVED" && rv.Author.ID != "" {
			approvers = append(approvers, tree.Approver{Login: rv.Author.Login, ID: rv.Author.ID})
		}
	}
	return tree.PullRequest{
		Number:         n.Number,
		Title:          n.Title,
		State:          state,
		Author:         n.Author.Login,
		Reviewers:      reviewers,
		BaseRef:        n.BaseRefName,
		HeadRef:        n.HeadRefName,
		HeadOID:        n.HeadRefOid,
		NodeID:         n.ID,
		Body:           n.Body,
		ReviewDecision: tree.ReviewDecision(n.ReviewDecision),
		Approvers:      approvers,
	}
}

const openPRsQuery = `query($owner:String!,$name:String!,$cursor:String){
  repository(owner:$owner,name:$name){
    defaultBranchRef{name}
    pullRequests(states:[OPEN],first:100,after:$cursor){
      pageInfo{hasNextPage endCursor}
      nodes{number id title body isDraft state reviewDecision
        author{login} baseRefName headRefName headRefOid
        latestReviews(first:50){nodes{state author{login ... on User{id}}}}
        reviewRequests(first:20){nodes{requestedReviewer{... on User{login}}}}}
    }
  }
}`

// FetchOpenPRs returns all open PRs (paginated) and the repo's default branch.
func (c *Client) FetchOpenPRs(ctx context.Context, repo config.Repo) ([]tree.PullRequest, string, error) {
	var prs []tree.PullRequest
	defaultBranch := ""
	var cursor *string
	for {
		var data struct {
			Repository struct {
				DefaultBranchRef struct {
					Name string `json:"name"`
				} `json:"defaultBranchRef"`
				PullRequests struct {
					PageInfo struct {
						HasNextPage bool   `json:"hasNextPage"`
						EndCursor   string `json:"endCursor"`
					} `json:"pageInfo"`
					Nodes []prNode `json:"nodes"`
				} `json:"pullRequests"`
			} `json:"repository"`
		}
		vars := map[string]any{"owner": repo.Owner, "name": repo.Name, "cursor": cursor}
		if err := c.do(ctx, openPRsQuery, vars, &data); err != nil {
			return nil, "", err
		}
		defaultBranch = data.Repository.DefaultBranchRef.Name
		for _, n := range data.Repository.PullRequests.Nodes {
			prs = append(prs, n.toPR())
		}
		if !data.Repository.PullRequests.PageInfo.HasNextPage {
			break
		}
		end := data.Repository.PullRequests.PageInfo.EndCursor
		cursor = &end
	}
	return prs, defaultBranch, nil
}

const prByNumberQuery = `query($owner:String!,$name:String!,$number:Int!){
  repository(owner:$owner,name:$name){
    pullRequest(number:$number){number id title body isDraft state reviewDecision
      author{login} baseRefName headRefName headRefOid
      latestReviews(first:50){nodes{state author{login ... on User{id}}}}
      reviewRequests(first:20){nodes{requestedReviewer{... on User{login}}}}}
  }
}`

// FetchPRsByNumber fetches specific PRs (e.g. merged parents referenced by
// links). Missing or inaccessible PRs are skipped.
func (c *Client) FetchPRsByNumber(ctx context.Context, repo config.Repo, numbers []int) ([]tree.PullRequest, error) {
	var prs []tree.PullRequest
	for _, num := range numbers {
		var data struct {
			Repository struct {
				PullRequest *prNode `json:"pullRequest"`
			} `json:"repository"`
		}
		vars := map[string]any{"owner": repo.Owner, "name": repo.Name, "number": num}
		if err := c.do(ctx, prByNumberQuery, vars, &data); err != nil {
			return nil, err
		}
		if data.Repository.PullRequest != nil {
			prs = append(prs, data.Repository.PullRequest.toPR())
		}
	}
	return prs, nil
}

const prCommitsQuery = `query($owner:String!,$name:String!,$number:Int!){
  repository(owner:$owner,name:$name){
    pullRequest(number:$number){
      commits(first:250){nodes{commit{messageHeadline}}}
    }
  }
}`

// FetchPRCommitSubjects returns the commit subjects (message headlines) of a
// PR's commits. It is used to recognise which commits on a descendant branch
// came from an already-merged parent whose recorded head no longer shares SHAs
// with that branch (squash-merge + restack). Returns nil if the PR is missing.
// Capped at the first 250 commits, which covers any realistic stacked PR.
func (c *Client) FetchPRCommitSubjects(ctx context.Context, repo config.Repo, number int) ([]string, error) {
	var data struct {
		Repository struct {
			PullRequest *struct {
				Commits struct {
					Nodes []struct {
						Commit struct {
							MessageHeadline string `json:"messageHeadline"`
						} `json:"commit"`
					} `json:"nodes"`
				} `json:"commits"`
			} `json:"pullRequest"`
		} `json:"repository"`
	}
	vars := map[string]any{"owner": repo.Owner, "name": repo.Name, "number": number}
	if err := c.do(ctx, prCommitsQuery, vars, &data); err != nil {
		return nil, err
	}
	if data.Repository.PullRequest == nil {
		return nil, nil
	}
	subjects := make([]string, 0, len(data.Repository.PullRequest.Commits.Nodes))
	for _, n := range data.Repository.PullRequest.Commits.Nodes {
		subjects = append(subjects, n.Commit.MessageHeadline)
	}
	return subjects, nil
}

const requestReviewsMutation = `mutation($pr:ID!,$users:[ID!]!){
  requestReviews(input:{pullRequestId:$pr,userIds:$users,union:true}){clientMutationId}
}`

// RequestReviews re-requests review from the given user node ids on a PR. With
// union:true it adds to, rather than replaces, the existing requested set.
func (c *Client) RequestReviews(ctx context.Context, prNodeID string, userIDs []string) error {
	var out struct {
		RequestReviews struct {
			ClientMutationID *string `json:"clientMutationId"`
		} `json:"requestReviews"`
	}
	vars := map[string]any{"pr": prNodeID, "users": userIDs}
	return c.do(ctx, requestReviewsMutation, vars, &out)
}

const viewerQuery = `query{viewer{login}}`

// Viewer returns the authenticated user's login.
func (c *Client) Viewer(ctx context.Context) (string, error) {
	var data struct {
		Viewer struct {
			Login string `json:"login"`
		} `json:"viewer"`
	}
	if err := c.do(ctx, viewerQuery, nil, &data); err != nil {
		return "", err
	}
	return data.Viewer.Login, nil
}
