!macro customUnInstall
  IfSilent preserve_phase15_data
  MessageBox MB_YESNO|MB_DEFBUTTON2|MB_ICONQUESTION \
    "Lokale Bot-Daten ebenfalls entfernen?$\r$\n$\r$\nStandardmäßig bleiben Konfiguration, Routen, Historie und Diagnose unter LocalAppData erhalten." \
    IDNO preserve_phase15_data
  MessageBox MB_YESNO|MB_DEFBUTTON2|MB_ICONEXCLAMATION \
    "Datenroot wirklich dauerhaft löschen? Diese Aktion kann nicht rückgängig gemacht werden." \
    IDNO preserve_phase15_data
  RMDir /r "$LOCALAPPDATA\D2ROfflineFarmingBot"
preserve_phase15_data:
!macroend
