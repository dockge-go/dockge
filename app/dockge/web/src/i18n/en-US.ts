// en-US 词典：键集与 zh-CN 严格对齐（类型由 i18n/index.ts 强制）。
const dict = {
  "app.title": "Dockge",

  // ---- common ----
  "common.ok": "OK",
  "common.cancel": "Cancel",
  "common.save": "Save",
  "common.close": "Close",
  "common.loading": "Loading…",
  "common.error": "Something went wrong",
  "common.language": "Language",
  "theme.light": "Light",
  "theme.dark": "Dark",
  "theme.auto": "Auto",

  // ---- topbar ----
  "nav.home": "Home",
  "nav.settings": "Settings",
  "nav.logout": "Logout",
  "nav.scanStacks": "Scan Stacks Folder",

  // ---- login / setup ----
  "login.title": "Sign in to Dockge",
  "login.username": "Username",
  "login.password": "Password",
  "login.remember": "Remember me",
  "login.submit": "Sign in",
  "setup.title": "Create your admin account",
  "setup.username": "Username",
  "setup.password": "Password",
  "setup.repeat": "Repeat Password",
  "setup.submit": "Create",
  "setup.welcome": "Dockge",

  // ---- home ----
  "home.active": "Active",
  "home.exited": "Exited",
  "home.inactive": "Inactive",
  "home.dockerRunTitle": "Docker Run",
  "home.dockerRunDesc": "Convert a docker run command to compose.yaml",
  "home.dockerRunPlaceholder": "docker run -d --name nginx -p 8080:80 nginx",
  "home.convert": "Convert to Compose",
  "home.convertSuccess": "Converted, please review in the editor",

  // ---- stack list ----
  "stacklist.searchPlaceholder": "Search…",
  "stacklist.empty": "No stacks yet",

  // ---- compose ----
  "compose.newTitle": "New Stack",
  "compose.name": "Stack Name",
  "compose.namePlaceholder": "my-app",
  "compose.nameHelp": "Lowercase letters, digits, underscore and hyphen only",
  "compose.deploy": "🚀 Deploy",
  "compose.saveDraft": "💾 Save Draft",
  "compose.discard": "Discard",
  "compose.edit": "Edit",
  "compose.start": "▶ Start",
  "compose.restart": "↻ Restart",
  "compose.update": "⬇ Update",
  "compose.stop": "⏹ Stop",
  "compose.down": "Down (stop and remove)",
  "compose.delete": "Delete",
  "compose.confirmDelete": "Confirm deletion",
  "compose.confirmDeleteDesc": "This will run compose down and delete all files of {name}. This cannot be undone.",
  "compose.containers": "Containers",
  "compose.terminal": "Terminal (combined logs)",
  "compose.progress": "Output",
  "compose.leaveConfirm": "You have unsaved changes. Leave anyway?",
  "compose.discardConfirm": "Discard all unsaved changes?",
  "compose.noContainers": "No containers deployed",
  "compose.terminalTitle": "Terminal - {service} ({stack})",
  "compose.switchShell": "Switch to {shell}",
  "compose.addContainerName": "Container name",
  "compose.addContainer": "Add Container",
  "compose.containerExists": "Container name already exists",
  "compose.deleteContainer": "Delete Container",
  "compose.editConfig": "Edit",
  "compose.networksHint": "Reference these networks in service environment/networks: {list}",

  // ---- container card ----
  "container.terminal": "Terminal",

  // ---- service config form (edit mode) ----
  "form.image": "Image",
  "form.ports": "Ports (HOST:CONTAINER per line)",
  "form.volumes": "Volumes (HOST:CONTAINER per line)",
  "form.restartPolicy": "Restart Policy",
  "form.env": "Environment Variables (KEY=VALUE per line)",
  "form.dependsOn": "Depends On (one service per line)",
  "form.networks": "Networks",

  "policy.always": "Always",
  "policy.unlessStopped": "Unless Stopped",
  "policy.onFailure": "On Failure",
  "policy.no": "Never",

  // ---- terminal ----
  "term.sessionEnded": "Session ended",

  // ---- editor ----
  "editor.checking": "Checking…",
  "editor.validCompose": "Compose valid",
  "editor.invalidCompose": "{n} issues",
  "editor.exitFullscreen": "Exit fullscreen",
  "editor.fullscreen": "Fullscreen",
  "editor.lines": "{n} lines",
  "editor.readonly": "Read-only",

  // ---- settings ----
  "settings.general": "General",
  "settings.appearance": "Appearance",
  "settings.security": "Security",
  "settings.globalEnv": "Global Env",
  "settings.about": "About",
  "settings.primaryHostname": "Primary Hostname",
  "settings.primaryHostnameHelp": "Hostname used by port links; falls back to the current page host when empty",
  "settings.autoGet": "Auto Get",
  "settings.checkUpdate": "Check Update On GitHub",
  "settings.showUpdateIfAvailable": "Show update if available",
  "settings.checkBeta": "Also check beta release",
  "settings.theme": "Theme",
  "settings.language": "Language",
  "settings.saved": "Saved",
  "settings.changePassword": "Change Password",
  "settings.currentPassword": "Current Password",
  "settings.newPassword": "New Password",
  "settings.repeatPassword": "Repeat New Password",
  "settings.disableAuth": "Disable Auth",
  "settings.enableAuth": "Enable Auth",
  "settings.disableAuthDesc": "Anyone will be able to access this panel. Confirm with your current password.",
  "settings.globalEnvDesc": "Global variables written to .env, applied to variable interpolation of all stacks.",
  "settings.version": "Version",
  "settings.newUpdate": "New Update",

  // ---- toast ----
  "toast.refreshed": "Refreshed",
  "toast.saved": "Saved",
  "toast.draftSaved": "Draft saved",
  "toast.deleted": "Deleted {name}",
  "toast.scanDone": "Scan completed",
};

export default dict;
