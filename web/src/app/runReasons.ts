/** Run-Availability-Texte für Dashboard und Settings-Queue-Editor. */

import { characterClassLabel } from "./characterReasons";

const reasonCopy: Record<string, { title: string; detail: string }> = {
  profile_class_mismatch: {
    title: "Nicht verfügbar",
    detail: "Die gespeicherte Route gehört zu einer anderen Klasse oder einem anderen Charakter.",
  },
  character_profile_run_incompatible: {
    title: "Nicht verfügbar",
    detail: "Die gespeicherte Route gehört zu einer anderen Klasse oder einem anderen Charakter.",
  },
  profile_run_strategy_unavailable: {
    title: "Nicht verfügbar",
    detail: "Für dieses Kampfprofil ist dieser Run noch nicht freigegeben.",
  },
  route_assignment_missing: {
    title: "Noch nicht eingerichtet",
    detail: "Für diesen Run wurde noch keine Route eingerichtet. Entferne den Run oder nimm die Route auf.",
  },
  leg_acquisition_route_missing: {
    title: "Noch nicht eingerichtet",
    detail: "Nimm zuerst die Wirt-Route auf.",
  },
  leg_acquisition_route_stale: {
    title: "Noch nicht bereit",
    detail: "Die Wirt-Route muss neu aufgenommen werden.",
  },
  cow_sweep_route_missing: {
    title: "Noch nicht eingerichtet",
    detail: "Nimm zuerst die Cow-Route auf.",
  },
  cow_sweep_route_stale: {
    title: "Noch nicht bereit",
    detail: "Die Cow-Route muss neu aufgenommen werden.",
  },
  route_set_binding_mismatch: {
    title: "Nicht verfügbar",
    detail: "Die beiden Kuhlevel-Routen passen nicht zum selben Charakter.",
  },
  route_lifecycle_unavailable: {
    title: "Noch nicht bereit",
    detail: "Die Route ist unvollständig oder nicht mehr verwendbar.",
  },
  route_stale: {
    title: "Noch nicht bereit",
    detail: "Die Route muss neu aufgenommen werden.",
  },
};

const reasonPriority = [
  "profile_class_mismatch",
  "character_profile_run_incompatible",
  "profile_run_strategy_unavailable",
  "route_assignment_missing",
  "route_set_binding_mismatch",
  "leg_acquisition_route_missing",
  "leg_acquisition_route_stale",
  "cow_sweep_route_missing",
  "cow_sweep_route_stale",
  "route_lifecycle_unavailable",
  "route_stale",
];

function unsupportedResolutionText(message: string) {
  if (!/requires 1280x720/i.test(message)) {
    return "";
  }
  const got = message.match(/got (\d+)x(\d+)/i);
  if (got) {
    return `D2R läuft in ${got[1]} × ${got[2]}. Stelle Fenster-Modus 1280 × 720 ein. Der Bot arbeitet nur in dieser Auflösung.`;
  }
  return "D2R läuft nicht in 1280 × 720. Stelle Fenster-Modus 1280 × 720 ein. Der Bot arbeitet nur in dieser Auflösung.";
}

export function isRunStartable(status: string) {
  return status === "available" || status === "runtime_validation_required";
}

export function runAvailabilityText(status: string, reasons: string[] = [], characterClass = "") {
  if (status === "available") {
    return { title: "Bereit", detail: "Route und Konfiguration sind bereit." };
  }
  if (status === "runtime_validation_required") {
    return { title: "Bereit", detail: "Tipp für Fernkämpfer: Beende die Routenaufnahme mit etwas Abstand zum Boss – an dieser Position beginnt später der Kampf." };
  }
  const matched = reasonPriority.find((reason) => reasons.includes(reason));
  if (matched === "profile_run_strategy_unavailable") {
    const classLabel = characterClassLabel(characterClass);
    return {
      title: "Nicht verfügbar",
      detail: classLabel
        ? `Dieser Run ist für ${classLabel} noch nicht freigegeben.`
        : "Für dieses Kampfprofil ist dieser Run noch nicht freigegeben.",
    };
  }
  if (matched && reasonCopy[matched]) {
    return reasonCopy[matched];
  }
  for (const reason of reasons) {
    if (reasonCopy[reason]) return reasonCopy[reason];
  }
  return { title: "Noch nicht bereit", detail: "Öffne die Routen, um die fehlende Einrichtung zu prüfen." };
}

export function queueStartErrorText(message: string) {
  const text = message.trim();
  const resolution = unsupportedResolutionText(text);
  if (resolution) {
    return resolution;
  }
  if (text.includes("queue_entry_unavailable") || text.includes("profile_class_mismatch")) {
    return "Ein Run in der Reihenfolge ist für diesen Charakter nicht startfähig.";
  }
  if (text.includes("queue_context_mismatch")) {
    return "Die Queue gehört nicht zur bestätigten Auswahl.";
  }
  if (text.includes("state_changed")) {
    return "Der Charakterkatalog hat sich geändert. Seite aktualisieren.";
  }
  if (text.includes("game_start_failed") || text.includes("expected in_game") || text.includes("start queue game")) {
    return "Das Spiel konnte nicht sicher gestartet werden. D2R muss im Rogue Encampment stehen oder auf dem Offline-Charakterbildschirm, damit der Bot das Spiel öffnet.";
  }
  if (text.includes("no usable client area")) {
    return "Das D2R-Fenster hat keine nutzbare Fläche. Stelle Fenster-Modus 1280 × 720 ein und lass das Fenster sichtbar, nicht minimiert.";
  }
  if (text.includes("character mismatch")) {
    return "Im Spiel ist ein anderer Charakter aktiv als die bestätigte Auswahl.";
  }
  if (text.includes("start area mismatch")) {
    return "Das Spiel muss im Rogue Encampment stehen, bevor die Queue startet.";
  }
  return text || "Die Farm-Queue konnte nicht sicher geprüft werden.";
}

export function selectionErrorText(message: string) {
  const text = message.trim();
  const resolution = unsupportedResolutionText(text);
  if (resolution) {
    return resolution;
  }
  if (text.includes("no usable client area")) {
    return "Das D2R-Fenster hat keine nutzbare Fläche. Stelle Fenster-Modus 1280 × 720 ein und lass das Fenster sichtbar, nicht minimiert.";
  }
  if (text.includes("character selection timeout") || text.includes("character_selection_unconfirmed")) {
    return "Der Charakterbildschirm wurde nicht sicher erkannt. D2R muss auf dem Offline-Charakterbildschirm bei 1280 × 720 stehen, und der gewünschte Save muss sichtbar markiert sein.";
  }
  if (text.includes("target anchor not found")) {
    return "Der Charakter wurde auf dem Auswahlbildschirm nicht gefunden. Prüfe 1280 × 720 und den markierten Save.";
  }
  return text || "Die Charakterauswahl konnte nicht sicher bestätigt werden.";
}
