// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// jenkins.go will implement CIProvider against the Jenkins REST API.
// Full implementation: Day 06.
//
// Jenkins API notes (for Day 06):
//   - Folder creation: POST /createItem?name=<folder> with XML body (Folder type)
//   - Multibranch job: POST /job/<folder>/createItem with XML body (WorkflowMultiBranchProject)
//   - Branch scan:     POST /job/<folder>/job/<name>/build
//   - Auth:           HTTP Basic (JENKINS_USER + JENKINS_TOKEN)

package plugin

import (
	"context"
	"fmt"

	"github.com/ALabiyb/platform_devportal/internal/config"
)

// JenkinsAdapter implements CIProvider. Stub until Day 06.
type JenkinsAdapter struct {
	cfg *config.Config
}

// NewJenkinsAdapter constructs a JenkinsAdapter stub.
func NewJenkinsAdapter(cfg *config.Config) *JenkinsAdapter {
	return &JenkinsAdapter{cfg: cfg}
}

func (j *JenkinsAdapter) EnsureFolder(ctx context.Context, folderName string) error {
	return fmt.Errorf("JenkinsAdapter.EnsureFolder: %w (Day 06)", ErrNotImplemented)
}

func (j *JenkinsAdapter) CreateJob(ctx context.Context, input CreateJobInput) (*JobResult, error) {
	return nil, fmt.Errorf("JenkinsAdapter.CreateJob: %w (Day 06)", ErrNotImplemented)
}

func (j *JenkinsAdapter) TriggerBranchScan(ctx context.Context, jobPath string) error {
	return fmt.Errorf("JenkinsAdapter.TriggerBranchScan: %w (Day 06)", ErrNotImplemented)
}
