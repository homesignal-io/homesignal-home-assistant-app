const actionLabels = {
  view_device: "View device",
  view_backups: "Review backups",
  view_updates: "Review updates",
};

const deviceDetailActions = new Set(["view_device", "view_backups", "view_updates"]);

export function primaryActionLabel(action) {
  return actionLabels[action] || "Review";
}

export function routeForPrimaryAction(action, row) {
  if (deviceDetailActions.has(action)) {
    return {
      page: "Device Detail",
      deviceId: row?.device?.device_id || "",
      focus: action,
    };
  }

  return {
    page: "Dashboard",
    deviceId: row?.device?.device_id || "",
    focus: action || "",
  };
}

export function isKnownPrimaryAction(action) {
  return Object.prototype.hasOwnProperty.call(actionLabels, action);
}
