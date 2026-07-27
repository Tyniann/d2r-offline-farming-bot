package api

import (
	"fmt"

	"github.com/Tyniann/d2r-offline-farming-bot/internal/app"
)

// CreateDiagnosticBundle lässt ausschließlich den Core den lokalen Diagnoseinhalt auswählen und redigieren.
func (b *LiveBackend) CreateDiagnosticBundle(request DiagnosticBundleRequest) (DiagnosticBundleDTO, error) {
	if b == nil || b.diagnostics == nil {
		return DiagnosticBundleDTO{}, fmt.Errorf("%s: diagnostic bundles require an installed data root", app.Phase15ReasonDiagnosticBundleFailed)
	}
	result, err := b.diagnostics.Create(app.DiagnosticBundleOptions{
		IncludeTelemetry: request.IncludeTelemetry,
		IncludeRoutes:    request.IncludeRoutes,
	})
	if err != nil {
		return DiagnosticBundleDTO{}, err
	}
	return DiagnosticBundleDTO{
		Filename: result.Filename, Bytes: result.Bytes,
		IncludedTelemetry: result.IncludedTelemetry, IncludedRoutes: result.IncludedRoutes,
	}, nil
}
