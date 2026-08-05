!define APP_NAME "Vocab"
!define APP_VERSION "{{.Version}}"
!define BINARY "vocab.exe"

OutFile "{{.Name}}.exe"
InstallDir "$PROGRAMFILES64\${APP_NAME}"
RequestExecutionLevel admin

Section "Install"
  ; Ask an in-place installation to stop through its local command mailbox.
  ; This avoids terminating an unrelated executable with the same filename.
  IfFileExists "$INSTDIR\${BINARY}" 0 +3
    ExecWait '"$INSTDIR\${BINARY}" -quit'
    Sleep 5000

  SetOutPath "$INSTDIR"
  File "${BINARY}"
  File "icon.ico"
  File "README.md"
  File "LICENSE"

  WriteUninstaller "$INSTDIR\uninstall.exe"

  WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Run" \
    "VocabDaemon" '"$INSTDIR\${BINARY}" -daemon'

  ; Start Menu entries so Vocab can be relaunched after Quit from the tray.
  CreateDirectory "$SMPROGRAMS\${APP_NAME}"
  CreateShortCut "$SMPROGRAMS\${APP_NAME}\${APP_NAME}.lnk" "$INSTDIR\${BINARY}"
  CreateShortCut "$SMPROGRAMS\${APP_NAME}\Uninstall ${APP_NAME}.lnk" "$INSTDIR\uninstall.exe"

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
  ExecWait '"$INSTDIR\${BINARY}" -quit'
  Sleep 5000
  DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "VocabDaemon"

  Delete "$INSTDIR\${BINARY}"
  Delete "$INSTDIR\icon.ico"
  Delete "$INSTDIR\README.md"
  Delete "$INSTDIR\LICENSE"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"

  Delete "$SMPROGRAMS\${APP_NAME}\${APP_NAME}.lnk"
  Delete "$SMPROGRAMS\${APP_NAME}\Uninstall ${APP_NAME}.lnk"
  RMDir "$SMPROGRAMS\${APP_NAME}"

  DeleteRegKey HKLM "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_NAME}"

  MessageBox MB_YESNO "Remove Vocab learner data and logs?" IDYES removeData IDNO keepData
removeData:
  RMDir /r "$APPDATA\vocab"
keepData:
SectionEnd
