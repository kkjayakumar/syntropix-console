package githubclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	patToken string
	appAuth  *AppAuth
	HTTP     *http.Client
}

func NewClient(patToken string, appAuth *AppAuth) *Client {
	return &Client{
		patToken: patToken,
		appAuth:  appAuth,
		HTTP:     &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) Enabled() bool {
	return c != nil && (c.patToken != "" || c.appAuth != nil)
}

func (c *Client) authToken(ctx context.Context) (string, error) {
	if c == nil {
		return "", fmt.Errorf("github client not configured")
	}
	if c.patToken != "" {
		return c.patToken, nil
	}
	if c.appAuth != nil {
		return c.appAuth.InstallationToken(ctx)
	}
	return "", fmt.Errorf("github token not configured")
}

type CreateIssueInput struct {
	Owner  string
	Repo   string
	Title  string
	Body   string
	Labels []string
}

type CreateIssueResult struct {
	Number int    `json:"number"`
	URL    string `json:"html_url"`
}

type CreatePRInput struct {
	Owner string
	Repo  string
	Title string
	Head  string
	Base  string
	Body  string
}

type PullRequestResult struct {
	Number int    `json:"number"`
	URL    string `json:"html_url"`
}

func (c *Client) CreateIssue(ctx context.Context, in CreateIssueInput) (*CreateIssueResult, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("github token not configured")
	}
	token, err := c.authToken(ctx)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"title":  in.Title,
		"body":   in.Body,
		"labels": in.Labels,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues", in.Owner, in.Repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")

	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	respBody, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("github create issue %d: %s", res.StatusCode, string(respBody))
	}
	var out CreateIssueResult
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

type UpdateIssueInput struct {
	Owner  string
	Repo   string
	Number int
	Title  string
	Body   string
}

func (c *Client) UpdateIssue(ctx context.Context, in UpdateIssueInput) error {
	if !c.Enabled() {
		return fmt.Errorf("github not configured")
	}
	token, err := c.authToken(ctx)
	if err != nil {
		return err
	}
	payload := map[string]any{"title": in.Title, "body": in.Body}
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d", in.Owner, in.Repo, in.Number)
	req, err := http.NewRequestWithContext(ctx, http.MethodPatch, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("github update issue %d: %s", res.StatusCode, string(b))
	}
	return nil
}

func (c *Client) TriggerWorkflowDispatch(ctx context.Context, owner, repo, workflowFile, ref string, inputs map[string]string) error {
	if !c.Enabled() {
		return fmt.Errorf("github not configured")
	}
	token, err := c.authToken(ctx)
	if err != nil {
		return err
	}
	payload := map[string]any{"ref": ref, "inputs": inputs}
	body, _ := json.Marshal(payload)
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/actions/workflows/%s/dispatches", owner, repo, workflowFile)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		b, _ := io.ReadAll(res.Body)
		return fmt.Errorf("workflow dispatch %d: %s", res.StatusCode, string(b))
	}
	return nil
}

func (c *Client) FindOpenPullRequest(ctx context.Context, owner, repo, headBranch string) (*PullRequestResult, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("github not configured")
	}
	token, err := c.authToken(ctx)
	if err != nil {
		return nil, err
	}
	head := fmt.Sprintf("%s:%s", owner, headBranch)
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls?state=open&head=%s&per_page=1", owner, repo, head)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	respBody, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("github list pulls %d: %s", res.StatusCode, string(respBody))
	}
	var pulls []PullRequestResult
	if err := json.Unmarshal(respBody, &pulls); err != nil {
		return nil, err
	}
	if len(pulls) == 0 {
		return nil, nil
	}
	return &pulls[0], nil
}

func (c *Client) CreatePullRequest(ctx context.Context, in CreatePRInput) (*PullRequestResult, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("github not configured")
	}
	token, err := c.authToken(ctx)
	if err != nil {
		return nil, err
	}
	payload := map[string]any{
		"title": in.Title,
		"head":  in.Head,
		"base":  in.Base,
		"body":  in.Body,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/pulls", in.Owner, in.Repo)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	respBody, _ := io.ReadAll(res.Body)
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("github create pull %d: %s", res.StatusCode, string(respBody))
	}
	var out PullRequestResult
	if err := json.Unmarshal(respBody, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func ParseRepo(repo string) (owner, name string, err error) {
	parts := strings.Split(repo, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid repo %q, expected owner/name", repo)
	}
	return parts[0], parts[1], nil
}

type BlameResult struct {
	AuthorName   string
	AuthorEmail  string
	CommitSHA    string
	CommitMsg    string
	AuthoredDate time.Time
}

func (c *Client) GetLineBlame(ctx context.Context, owner, repo, ref, path string, lineNum int) (*BlameResult, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("github not configured")
	}
	token, err := c.authToken(ctx)
	if err != nil {
		return nil, err
	}

	query := `query($owner: String!, $repo: String!, $ref: String!, $path: String!) {
		repository(owner: $owner, name: $repo) {
			object(expression: $ref) {
				... on Commit {
					blame(path: $path) {
						ranges {
							startingLine
							endingLine
							commit {
								oid
								message
								authoredDate
								author {
									name
									email
								}
							}
						}
					}
				}
			}
		}
	}`

	variables := map[string]string{
		"owner": owner,
		"repo":  repo,
		"ref":   ref,
		"path":  path,
	}

	payload := map[string]any{
		"query":     query,
		"variables": variables,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.github.com/graphql", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	respBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("graphql error %d: %s", res.StatusCode, string(respBody))
	}

	var graphQLRes struct {
		Data struct {
			Repository struct {
				Object struct {
					Blame struct {
						Ranges []struct {
							StartingLine int `json:"startingLine"`
							EndingLine   int `json:"endingLine"`
							Commit       struct {
								Oid          string    `json:"oid"`
								Message      string    `json:"message"`
								AuthoredDate time.Time `json:"authoredDate"`
								Author       struct {
									Name  string `json:"name"`
									Email string `json:"email"`
								} `json:"author"`
							} `json:"commit"`
						} `json:"ranges"`
					} `json:"blame"`
				} `json:"object"`
			} `json:"repository"`
		} `json:"data"`
		Errors []struct {
			Message string `json:"message"`
		} `json:"errors"`
	}

	if err := json.Unmarshal(respBody, &graphQLRes); err != nil {
		return nil, err
	}

	if len(graphQLRes.Errors) > 0 {
		return nil, fmt.Errorf("graphql error: %s", graphQLRes.Errors[0].Message)
	}

	ranges := graphQLRes.Data.Repository.Object.Blame.Ranges
	for _, r := range ranges {
		if lineNum >= r.StartingLine && lineNum <= r.EndingLine {
			return &BlameResult{
				AuthorName:   r.Commit.Author.Name,
				AuthorEmail:  r.Commit.Author.Email,
				CommitSHA:    r.Commit.Oid,
				CommitMsg:    r.Commit.Message,
				AuthoredDate: r.Commit.AuthoredDate,
			}, nil
		}
	}

	return nil, fmt.Errorf("line %d not found in blame ranges for path %s", lineNum, path)
}
