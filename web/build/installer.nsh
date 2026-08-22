LangString uninstallDeleteDataPrompt 1031 \
  "Lokale Bot-Daten ebenfalls entfernen?$\r$\n$\r$\nStandardmäßig bleiben Konfiguration, Routen, Historie und Diagnose unter LocalAppData erhalten."
LangString uninstallDeleteDataPrompt 1033 \
  "Remove local bot data as well?$\r$\n$\r$\nBy default, configuration, routes, history, and diagnostics remain under LocalAppData."

LangString uninstallDeleteDataConfirm 1031 \
  "Datenroot wirklich dauerhaft löschen? Diese Aktion kann nicht rückgängig gemacht werden."
LangString uninstallDeleteDataConfirm 1033 \
  "Permanently delete the data root? This action cannot be undone."

!macro customUnInstall
  IfSilent preserve_phase15_data
  MessageBox MB_YESNO|MB_DEFBUTTON2|MB_ICONQUESTION \
    "$(uninstallDeleteDataPrompt)" \
    IDNO preserve_phase15_data
  MessageBox MB_YESNO|MB_DEFBUTTON2|MB_ICONEXCLAMATION \
    "$(uninstallDeleteDataConfirm)" \
    IDNO preserve_phase15_data
  RMDir /r "$LOCALAPPDATA\D2ROfflineFarmingBot"
preserve_phase15_data:
!macroend
