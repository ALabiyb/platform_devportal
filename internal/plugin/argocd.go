// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// argocd.go will implement GitOpsProvider against the ArgoCD REST API v1.
// Full implementation: Day 08.
//
// ArgoCD API notes (for Day 08):
//   - Create Application: POST /api/v1/applications
//   - Sync Application:   POST /api/v1/applications/<name>/sync
//   - Get status:         GET  /api/v1/applications/<name>
//   - Auth:              Authorization: Bearer <ARGOCD_TOKEN>
//   - Token generation:  argocd account generate-token

package plugin

import (
	"context"
	"fmt"

	"github.com/ALabiyb/platform_devportal/internal/config"
)

// ArgoCDAdapter implements GitOpsProvider. Stub until Day 08.
type ArgoCDAdapter struct {
	cfg *config.Config
}

// NewArgoCDAdapter constructs an ArgoCDAdapter stub.
func NewArgoCDAdapter(cfg *config.Config) *ArgoCDAdapter {
	return &ArgoCDAdapter{cfg: cfg}
}

func (a *ArgoCDAdapter) CreateApplication(ctx context.Context, input CreateAppInput) error {
	return fmt.Errorf("ArgoCDAdapter.CreateApplication: %w (Day 08)", ErrNotImplemented)
}

func (a *ArgoCDAdapter) SyncApplication(ctx context.Context, appName string) error {
	return fmt.Errorf("ArgoCDAdapter.SyncApplication: %w (Day 08)", ErrNotImplemented)
}

func (a *ArgoCDAdapter) GetApplicationStatus(ctx context.Context, appName string) (*AppStatus, error) {
	return nil, fmt.Errorf("ArgoCDAdapter.GetApplicationStatus: %w (Day 08)", ErrNotImplemented)
}
