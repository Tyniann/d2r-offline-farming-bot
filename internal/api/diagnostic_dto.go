package api

// DiagnosticBundleRequest aktiviert ausschließlich ausdrücklich bestätigte sensitive Beigaben.
type DiagnosticBundleRequest struct {
	IncludeTelemetry bool `json:"include_telemetry"`
	IncludeRoutes    bool `json:"include_routes"`
}

// DiagnosticBundleDTO beschreibt das lokal gespeicherte Paket ohne absoluten Pfad.
type DiagnosticBundleDTO struct {
	Filename          string `json:"filename"`
	Bytes             int64  `json:"bytes"`
	IncludedTelemetry bool   `json:"included_telemetry"`
	IncludedRoutes    bool   `json:"included_routes"`
}
