package gitlab

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

var httpClient = &http.Client{
	Timeout: 30 * time.Second,
}

type User struct {
	ID       int    `json:"id"`
	Username string `json:"username"`
	Name     string `json:"name"`
}

type Project struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	PathWithNamespace string `json:"path_with_namespace"`
	HTTPURLToRepo     string `json:"http_url_to_repo"`
	SSHURLToRepo      string `json:"ssh_url_to_repo"`
	WebURL            string `json:"web_url"`
	Description       string `json:"description"`
	Visibility        string `json:"visibility"`
	Archived          bool   `json:"archived"`
	ForkedFromProject *struct {
		ID int `json:"id"`
	} `json:"forked_from_project"`
	Namespace struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Path     string `json:"path"`
		Kind     string `json:"kind"`
		FullPath string `json:"full_path"`
	} `json:"namespace"`
}

type Group struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Path     string `json:"path"`
	FullPath string `json:"full_path"`
	WebURL   string `json:"web_url"`
}

func (p *Project) IsFork() bool {
	return p.ForkedFromProject != nil
}

func (p *Project) IsPrivate() bool {
	return p.Visibility == "private"
}

func (p *Project) IsPublic() bool {
	return p.Visibility == "public"
}

func GetAuthenticatedUser(token string, baseURL string) (*User, error) {
	body, err := doRequest(token, baseURL, "/api/v4/user")
	if err != nil {
		return nil, fmt.Errorf("fetching user profile: %w", err)
	}

	var user User
	if err := json.Unmarshal(body, &user); err != nil {
		return nil, fmt.Errorf("parsing user profile: %w", err)
	}

	return &user, nil
}

func GetUserProjects(token string, baseURL string) ([]Project, error) {
	var allProjects []Project
	page := 1

	for {
		path := fmt.Sprintf("/api/v4/projects?membership=true&owned=true&per_page=100&page=%d", page)
		body, err := doRequest(token, baseURL, path)
		if err != nil {
			return nil, fmt.Errorf("fetching user projects (page %d): %w", page, err)
		}

		var projects []Project
		if err := json.Unmarshal(body, &projects); err != nil {
			return nil, fmt.Errorf("parsing projects: %w", err)
		}

		if len(projects) == 0 {
			break
		}

		allProjects = append(allProjects, projects...)

		if len(projects) < 100 {
			break
		}
		page++
	}

	return allProjects, nil
}

func GetUserGroups(token string, baseURL string) ([]Group, error) {
	var allGroups []Group
	page := 1

	for {
		path := fmt.Sprintf("/api/v4/groups?per_page=100&page=%d", page)
		body, err := doRequest(token, baseURL, path)
		if err != nil {
			return nil, fmt.Errorf("fetching groups (page %d): %w", page, err)
		}

		var groups []Group
		if err := json.Unmarshal(body, &groups); err != nil {
			return nil, fmt.Errorf("parsing groups: %w", err)
		}

		if len(groups) == 0 {
			break
		}

		allGroups = append(allGroups, groups...)

		if len(groups) < 100 {
			break
		}
		page++
	}

	return allGroups, nil
}

func GetGroupProjects(token string, baseURL string, groupID int) ([]Project, error) {
	var allProjects []Project
	page := 1

	for {
		path := fmt.Sprintf("/api/v4/groups/%d/projects?per_page=100&include_subgroups=true&with_shared=false&page=%d", groupID, page)
		body, err := doRequest(token, baseURL, path)
		if err != nil {
			return nil, fmt.Errorf("fetching group projects (page %d): %w", page, err)
		}

		var projects []Project
		if err := json.Unmarshal(body, &projects); err != nil {
			return nil, fmt.Errorf("parsing group projects: %w", err)
		}

		if len(projects) == 0 {
			break
		}

		allProjects = append(allProjects, projects...)

		if len(projects) < 100 {
			break
		}
		page++
	}

	return allProjects, nil
}

func doRequest(token string, baseURL string, path string) ([]byte, error) {
	url := baseURL + path

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("PRIVATE-TOKEN", token)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("authentication failed — check your token")
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned %d: %s", resp.StatusCode, string(body))
	}

	return body, nil
}
