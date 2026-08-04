// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// jenkins.go implements CIProvider against the Jenkins REST API using HTTP Basic auth.
//
// Auth: JENKINS_TOKEN must be a Jenkins API token (not the user password).
// API tokens automatically satisfy Jenkins CSRF protection — no crumb needed.
//
// Idempotency:
//
//	EnsureFolder     — GET /job/{name}/api/json; skip POST if 200.
//	CreateJob        — GET /job/{folder}/job/{name}/api/json; skip POST if 200.
//	TriggerBranchScan — always fires; Jenkins deduplicates queued scans.

package plugin

import (
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/ALabiyb/platform_devportal/internal/config"
)

// folderXML is the Jenkins CloudBees Folder config used to create team folders.
const folderXML = `<?xml version='1.1' encoding='UTF-8'?>
<com.cloudbees.hudson.plugins.folder.Folder plugin="cloudbees-folder@6.815.v0dd5a_579d00d">
  <description></description>
</com.cloudbees.hudson.plugins.folder.Folder>`

// JenkinsAdapter implements CIProvider against the Jenkins REST API.
type JenkinsAdapter struct {
	client    *http.Client
	baseURL   string // internal Jenkins URL used for API calls, e.g. "http://jenkins:8080"
	publicURL string // public-facing URL embedded in webhook configs and developer-visible links
	auth      string // precomputed "Basic base64(user:token)" value set on every request
}

// NewJenkinsAdapter constructs a JenkinsAdapter from config.
// httpClient should be the shared TLS-configured client built in main.
func NewJenkinsAdapter(cfg *config.Config, httpClient *http.Client) *JenkinsAdapter {
	creds := base64.StdEncoding.EncodeToString([]byte(cfg.JenkinsUser + ":" + cfg.JenkinsToken))
	return &JenkinsAdapter{
		client:    httpClient,
		baseURL:   strings.TrimRight(cfg.JenkinsURL, "/"),
		publicURL: strings.TrimRight(cfg.JenkinsPublicURL, "/"),
		auth:      "Basic " + creds,
	}
}

// EnsureFolder creates a Jenkins folder for a team.
// If the folder already exists the call is a no-op (idempotent).
func (j *JenkinsAdapter) EnsureFolder(ctx context.Context, folderName string) error {
	checkURL := fmt.Sprintf("%s/job/%s/api/json", j.baseURL, url.PathEscape(folderName))
	resp, err := j.do(ctx, http.MethodGet, checkURL, "", nil)
	if err != nil {
		return fmt.Errorf("jenkins EnsureFolder check: %w", err)
	}
	drain(resp.Body)
	if resp.StatusCode == http.StatusOK {
		return nil // folder already exists
	}

	createURL := fmt.Sprintf("%s/createItem?name=%s", j.baseURL, url.QueryEscape(folderName))
	resp, err = j.do(ctx, http.MethodPost, createURL, "application/xml", strings.NewReader(folderXML))
	if err != nil {
		return fmt.Errorf("jenkins EnsureFolder create: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jenkins EnsureFolder: unexpected status %d: %s", resp.StatusCode, truncate(body, 256))
	}
	return nil
}

// CreateJob creates a multibranch pipeline job inside the team folder.
// Returns a JobResult with the job path (folder/name) and the public URL.
// Idempotent — returns the existing result if the job already exists.
func (j *JenkinsAdapter) CreateJob(ctx context.Context, input CreateJobInput) (*JobResult, error) {
	// Idempotency check
	checkURL := fmt.Sprintf("%s/job/%s/job/%s/api/json",
		j.baseURL,
		url.PathEscape(input.FolderName),
		url.PathEscape(input.JobName),
	)
	resp, err := j.do(ctx, http.MethodGet, checkURL, "", nil)
	if err != nil {
		return nil, fmt.Errorf("jenkins CreateJob check: %w", err)
	}
	drain(resp.Body)
	if resp.StatusCode == http.StatusOK {
		return j.buildJobResult(input.FolderName, input.JobName), nil
	}

	xml := renderMultibranchXML(input)
	createURL := fmt.Sprintf("%s/job/%s/createItem?name=%s",
		j.baseURL,
		url.PathEscape(input.FolderName),
		url.QueryEscape(input.JobName),
	)
	resp, err = j.do(ctx, http.MethodPost, createURL, "application/xml", strings.NewReader(xml))
	if err != nil {
		return nil, fmt.Errorf("jenkins CreateJob create: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("jenkins CreateJob: unexpected status %d: %s", resp.StatusCode, truncate(body, 256))
	}
	return j.buildJobResult(input.FolderName, input.JobName), nil
}

// TriggerBranchScan fires a branch indexing scan on a multibranch pipeline job.
// jobPath must be "folderName/jobName" as returned by CreateJob.
// Jenkins deduplicates queued scans, so repeated calls are safe.
func (j *JenkinsAdapter) TriggerBranchScan(ctx context.Context, jobPath string) error {
	parts := strings.SplitN(jobPath, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("jenkins TriggerBranchScan: invalid jobPath %q — want folder/name", jobPath)
	}

	scanURL := fmt.Sprintf("%s/job/%s/job/%s/build",
		j.baseURL,
		url.PathEscape(parts[0]),
		url.PathEscape(parts[1]),
	)
	resp, err := j.do(ctx, http.MethodPost, scanURL, "", nil)
	if err != nil {
		return fmt.Errorf("jenkins TriggerBranchScan: %w", err)
	}
	defer resp.Body.Close()

	// 201 = queued successfully; 200 = also accepted
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("jenkins TriggerBranchScan: unexpected status %d: %s", resp.StatusCode, truncate(body, 256))
	}
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

// do executes an HTTP request with Jenkins Basic auth.
// body and contentType may be nil/"" for GET requests.
func (j *JenkinsAdapter) do(ctx context.Context, method, rawURL, contentType string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, rawURL, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", j.auth)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return j.client.Do(req)
}

func (j *JenkinsAdapter) buildJobResult(folder, name string) *JobResult {
	return &JobResult{
		Path: folder + "/" + name,
		URL:  fmt.Sprintf("%s/job/%s/job/%s", j.publicURL, url.PathEscape(folder), url.PathEscape(name)),
	}
}

// renderMultibranchXML builds the Jenkins WorkflowMultiBranchProject config XML.
// Values are XML-escaped to prevent injection via project names or URLs.
func renderMultibranchXML(input CreateJobInput) string {
	return `<?xml version='1.1' encoding='UTF-8'?>
<org.jenkinsci.plugins.workflow.multibranch.WorkflowMultiBranchProject plugin="workflow-multibranch@756.v891d88f2cd46">
  <description>` + xmlEscape(input.Description) + `</description>
  <sources class="jenkins.branch.MultiBranchProject$BranchSourceList" plugin="branch-api@2.1144.v4fb_568ff4b_4a">
    <data>
      <jenkins.branch.BranchSource>
        <source class="jenkins.plugins.git.GitSCMSource" plugin="git@5.2.1">
          <id>` + xmlEscape(input.JobName) + `</id>
          <remote>` + xmlEscape(input.GitURL) + `</remote>
          <credentialsId>` + xmlEscape(input.CredentialsID) + `</credentialsId>
          <traits>
            <jenkins.plugins.git.traits.BranchDiscoveryTrait/>
          </traits>
        </source>
        <strategy class="jenkins.branch.DefaultBranchPropertyStrategy">
          <properties class="empty-list"/>
        </strategy>
      </jenkins.branch.BranchSource>
    </data>
    <owner class="org.jenkinsci.plugins.workflow.multibranch.WorkflowMultiBranchProject" reference="../.."/>
  </sources>
  <factory class="org.jenkinsci.plugins.workflow.multibranch.WorkflowBranchProjectFactory">
    <owner class="org.jenkinsci.plugins.workflow.multibranch.WorkflowMultiBranchProject" reference="../.."/>
    <scriptPath>Jenkinsfile</scriptPath>
  </factory>
</org.jenkinsci.plugins.workflow.multibranch.WorkflowMultiBranchProject>`
}

// xmlEscape replaces the five XML special characters so values are safe to
// embed in element text content and attribute values.
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}

// drain discards the response body so the underlying TCP connection can be reused.
func drain(body io.ReadCloser) {
	io.Copy(io.Discard, body) //nolint:errcheck
	body.Close()
}

// truncate caps a byte slice at n bytes for safe inclusion in error messages.
func truncate(b []byte, n int) string {
	if len(b) > n {
		return string(b[:n]) + "…"
	}
	return string(b)
}
