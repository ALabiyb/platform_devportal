// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// harbor.go will implement RegistryProvider against the Harbor v2 REST API.
// Full implementation: Day 07.
//
// Harbor API notes (for Day 07):
//   - Create project:      POST /api/v2.0/projects
//   - Create robot:        POST /api/v2.0/projects/<name>/robots
//   - Auth:               HTTP Basic (HARBOR_USER + HARBOR_TOKEN)
//   - Robot name prefix:  "robot$<project>+<name>"

package plugin

import (
	"context"
	"fmt"

	"github.com/ALabiyb/platform_devportal/internal/config"
)

// HarborAdapter implements RegistryProvider. Stub until Day 07.
type HarborAdapter struct {
	cfg *config.Config
}

// NewHarborAdapter constructs a HarborAdapter stub.
func NewHarborAdapter(cfg *config.Config) *HarborAdapter {
	return &HarborAdapter{cfg: cfg}
}

func (h *HarborAdapter) EnsureProject(ctx context.Context, projectName string) error {
	return fmt.Errorf("HarborAdapter.EnsureProject: %w (Day 07)", ErrNotImplemented)
}

func (h *HarborAdapter) EnsureRobotAccount(ctx context.Context, projectName, robotName string) (*RobotCredential, error) {
	return nil, fmt.Errorf("HarborAdapter.EnsureRobotAccount: %w (Day 07)", ErrNotImplemented)
}
