!define APP_NAME "Vocab"
!define APP_VERSION "{{.Version}}"
!define BINARY "vocab.exe"

OutFile "{{.Name}}.exe"
InstallDir "$PROGRAMFILES64\${APP_NAME}"
RequestExecutionLevel admin

Section "Install"
  SetOutPath "$INSTDIR"
  File "${BINARY}"
  File "icon.ico"
  File "README.md"
  File "LICENSE"
  File /r "lexicon"

  WriteUninstaller "$INSTDIR\uninstall.exe"

  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Run" \
    "VocabDaemon" '"$INSTDIR\${BINARY}" -daemon'

  Exec '"$INSTDIR\${BINARY}" -daemon'

  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}" \
    "DisplayName" "${APP_NAME} ${APP_VERSION}"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}" \
    "UninstallString" "$INSTDIR\uninstall.exe"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}" \
    "DisplayIcon" "$INSTDIR\icon.ico"
  WriteRegStr HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}" \
    "Publisher" "msaeedsaeedi"
  WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}" \
    "NoModify" 1
  WriteRegDWORD HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}" \
    "NoRepair" 1
SectionEnd

Section "Uninstall"
  nsExec::Exec 'taskkill /f /im ${BINARY}'
  DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "VocabDaemon"

  Delete "$INSTDIR\${BINARY}"
  Delete "$INSTDIR\icon.ico"
  Delete "$INSTDIR\README.md"
  Delete "$INSTDIR\LICENSE"
  Delete "$INSTDIR\uninstall.exe"
  RMDir /r "$INSTDIR\lexicon"
  RMDir "$INSTDIR"

  DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}"
SectionEnd