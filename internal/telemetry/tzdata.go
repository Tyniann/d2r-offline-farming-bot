package telemetry

// Die installierte Windows-App darf nicht vom Zeitzonendatenbestand des Hosts
// abhängen, weil lokale Tagesgrenzen einschließlich DST Core-Autorität sind.
import _ "time/tzdata"
