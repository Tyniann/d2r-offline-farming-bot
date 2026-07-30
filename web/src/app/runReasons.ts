/** Run-Availability-Texte für Dashboard und Settings-Queue-Editor. */
export function runAvailabilityText(status: string, reasons: string[] = []) {
  if (status === "available") {
    return { title: "Bereit", detail: "Route und Konfiguration sind bereit." };
  }
  if (status === "runtime_validation_required") {
    return { title: "Bereit", detail: "Tipp für Fernkämpfer: Beende die Routenaufnahme mit etwas Abstand zum Boss – an dieser Position beginnt später der Kampf." };
  }
  if (reasons.includes("route_assignment_missing")) {
    return { title: "Noch nicht eingerichtet", detail: "Für diesen Run wurde noch keine Route eingerichtet." };
  }
  if (reasons.includes("character_profile_run_incompatible") || reasons.includes("profile_class_mismatch")) {
    return { title: "Nicht verfügbar", detail: "Das Kampfprofil dieses Charakters unterstützt diesen Run nicht." };
  }
  return { title: "Noch nicht bereit", detail: "Öffne die Routen, um die fehlende Einrichtung zu prüfen." };
}
