// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// defectdojo.go will implement SecurityProvider against the DefectDojo REST API v2.
// Full implementation: Day 07.
//
// DefectDojo API notes (for Day 07):
//   - List/create products:    GET|POST /api/v2/products/
//   - List/create engagements: GET|POST /api/v2/engagements/
//   - Auth:                   Authorization: Token <DEFECTDOJO_TOKEN>
//   - Idempotency:            Search by name before creating to avoid duplicates

package plugin

import (
	"context"
	"fmt"

	"github.com/ALabiyb/platform_devportal/internal/config"
)

// DefectDojoAdapter implements SecurityProvider. Stub until Day 07.
type DefectDojoAdapter struct {
	cfg *config.Config
}

// NewDefectDojoAdapter constructs a DefectDojoAdapter stub.
func NewDefectDojoAdapter(cfg *config.Config) *DefectDojoAdapter {
	return &DefectDojoAdapter{cfg: cfg}
}

func (d *DefectDojoAdapter) EnsureProduct(ctx context.Context, name, description string) (int, error) {
	return 0, fmt.Errorf("DefectDojoAdapter.EnsureProduct: %w (Day 07)", ErrNotImplemented)
}

func (d *DefectDojoAdapter) CreateEngagement(ctx context.Context, input CreateEngagementInput) (int, error) {
	return 0, fmt.Errorf("DefectDojoAdapter.CreateEngagement: %w (Day 07)", ErrNotImplemented)
}
