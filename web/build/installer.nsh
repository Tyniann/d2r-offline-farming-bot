LangString uninstallDeleteDataPrompt 1031 \
  "Lokale Bot-Daten ebenfalls entfernen?$\r$\n$\r$\nStandardmäßig bleiben Konfiguration, Routen, Historie und Diagnose unter LocalAppData erhalten."
LangString uninstallDeleteDataPrompt 1033 \
  "Remove local bot data as well?$\r$\n$\r$\nBy default, configuration, routes, history, and diagnostics remain under LocalAppData."

LangString uninstallDeleteDataConfirm 1031 \
  "Datenroot wirklich dauerhaft löschen? Diese Aktion kann nicht rückgängig gemacht werden."
LangString uninstallDeleteDataConfirm 1033 \
  "Permanently delete the data root? This action cannot be undone."

!ifndef BUILD_UNINSTALLER
Var existingInstallDirectory

!macro customInit
  StrCpy $existingInstallDirectory ""
  ${If} $hasPerUserInstallation == "1"
  ${AndIf} $perUserInstallationFolder != ""
  ${AndIf} ${FileExists} "$perUserInstallationFolder\${APP_EXECUTABLE_FILENAME}"
    StrCpy $existingInstallDirectory "$perUserInstallationFolder"
  ${EndIf}
!macroend

!macro customPageAfterChangeDir
  !define MUI_PAGE_CUSTOMFUNCTION_SHOW restoreExistingInstallDirectory
!macroend

Function restoreExistingInstallDirectory
  # electron-builder appends ${APP_FILENAME} before this page. Keep that
  # first-install safeguard, but restore a verified existing upgrade target.
  StrCmp $existingInstallDirectory "" restore_existing_install_directory_done
  StrCpy $INSTDIR "$existingInstallDirectory"
restore_existing_install_directory_done:
FunctionEnd
!endif

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
