// ---------------------------------------------------------------------------
// Author: Labiyb M. Said — DevSecOps Engineer
// Contact: saidlabiybm@gmail.com
// ---------------------------------------------------------------------------

// Package membersync propagates DevPortal application member changes to all
// connected platform tools (DefectDojo, Harbor, Dependency-Track) in a single
// best-effort pass. Every tool error is logged but never returned — the member
// change is already committed to DevPortal's DB and Gitea before sync runs.
package membersync

import (
	"context"
	"log/slog"

	"github.com/google/uuid"

	"github.com/ALabiyb/platform_devportal/internal/db"
	"github.com/ALabiyb/platform_devportal/internal/plugin"
)

// Service fans out application membership changes to all platform tools.
type Service struct {
	db       *db.DB
	security plugin.SecurityProvider
	registry plugin.RegistryProvider
	dt       plugin.DependencyTrackProvider
}

// New constructs a MemberSync Service. Nil providers are silently skipped.
func New(
	database *db.DB,
	security plugin.SecurityProvider,
	registry plugin.RegistryProvider,
	dt plugin.DependencyTrackProvider,
) *Service {
	return &Service{
		db:       database,
		security: security,
		registry: registry,
		dt:       dt,
	}
}

// SyncApplicationMembers resolves the current member email list for appID and
// propagates it to DefectDojo, Harbor, and Dependency-Track for every service
// belonging to the application. All errors are non-fatal and logged as warnings.
func (s *Service) SyncApplicationMembers(ctx context.Context, appID uuid.UUID) {
	members, err := s.db.ListApplicationMembers(ctx, appID)
	if err != nil {
		slog.Warn("membersync: list members", "app_id", appID, "err", err)
		return
	}
	emails := make([]string, 0, len(members))
	for _, m := range members {
		emails = append(emails, m.Email)
	}

	services, err := s.db.ListServicesByApplication(ctx, appID)
	if err != nil {
		slog.Warn("membersync: list services", "app_id", appID, "err", err)
		return
	}

	for _, svc := range services {
		s.syncService(ctx, svc, emails)
	}
}

func (s *Service) syncService(ctx context.Context, svc db.Project, emails []string) {
	// DefectDojo — product member list.
	if s.security != nil && svc.DefectDojoProductID != nil {
		if err := s.security.SyncProductMembers(ctx, *svc.DefectDojoProductID, emails); err != nil {
			slog.Warn("membersync: defectdojo", "service", svc.Slug, "err", err)
		}
	}

	// Harbor — project member list.
	if s.registry != nil && svc.HarborProject != "" {
		if err := s.registry.SyncProjectMembers(ctx, svc.HarborProject, emails); err != nil {
			slog.Warn("membersync: harbor", "service", svc.Slug, "err", err)
		}
	}

	// Dependency-Track — ensure project exists and update email notification rule.
	if s.dt != nil {
		projectUUID, err := s.dt.EnsureProject(ctx, svc.Slug)
		if err != nil {
			slog.Warn("membersync: dt ensure project", "service", svc.Slug, "err", err)
			return
		}
		if projectUUID == "" {
			return // DT not configured
		}
		if err := s.dt.EnsureEmailNotification(ctx, projectUUID, svc.Slug+"-vuln", emails); err != nil {
			slog.Warn("membersync: dt notification", "service", svc.Slug, "err", err)
		}
	}
}
