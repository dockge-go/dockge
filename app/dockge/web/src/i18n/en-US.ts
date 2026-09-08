export default {
  app: { title: 'Dockge — Container Management', lang: 'en-US' },
  nav: { overview: 'Overview', resources: 'Resources', system: 'System' },
  pages: {
    dashboard: 'Dashboard', containers: 'Containers', stacks: 'Stacks', images: 'Images',
    volumes: 'Volumes', networks: 'Networks',
    sysinfo: 'System Info', sysdf: 'Disk Usage', settings: 'Settings',
  },
  actions: {
    login: 'Login', logout: 'Logout', loading: 'Loading…',
    newContainer: 'New Container', newStack: 'New Stack', refresh: 'Refresh',
    start: 'Start', stop: 'Stop', restart: 'Restart', remove: 'Remove',
    clearStopped: 'Clear Stopped', clearUnusedImages: 'Clear Unused Images',
    clearUnusedVolumes: 'Clear Unused Volumes', clearUnusedNetworks: 'Clear Unused Networks',
    pruneAll: 'Prune All', scan: 'Scan',
  },
  status: {
    running: 'Running', exited: 'Exited', paused: 'Paused', dead: 'Dead',
    unknown: 'Unknown', deployed: 'Deployed', created: 'Created',
  },
  forms: {
    username: 'Username', password: 'Password', confirmPassword: 'Confirm Password',
    label: 'Label', placeholder: { search: 'Search…' },
  },
  feedback: {
    success: 'Success', error: 'Error', info: 'Info',
    dataRefreshed: 'Data refreshed', containerRemoved: 'Container removed',
    stackRemoved: 'Stack removed', imageRemoved: 'Image removed',
    volumeRemoved: 'Volume removed', networkRemoved: 'Network removed',
    loginFailed: 'Login failed, please check username and password',
    setupSuccess: 'Admin account created',
  },
};
