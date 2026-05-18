import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";
import { dirname, resolve } from "node:path";
import { isKnownPrimaryAction } from "../src/actions.js";

const scriptDir = dirname(fileURLToPath(import.meta.url));
const root = resolve(scriptDir, "../..");
const contractDir = resolve(root, "testdata/contracts/api/public-v1");

function readFixture(name) {
  return JSON.parse(readFileSync(resolve(contractDir, name), "utf8"));
}

function assert(condition, message) {
  if (!condition) {
    throw new Error(message);
  }
}

const dashboard = readFixture("dashboard.json");
const devices = readFixture("devices.json");
const activity = readFixture("activity.json");

assert(dashboard.schema_version === 1, "dashboard fixture schema_version must be 1");
assert(devices.schema_version === 1, "devices fixture schema_version must be 1");
assert(activity.schema_version === 1, "activity fixture schema_version must be 1");

const dashboardDeviceIds = new Set(
  dashboard.managed_home_assistants.map((row) => row.device.device_id)
);
const devicesDeviceIds = new Set(devices.devices.map((row) => row.device.device_id));

assert(
  dashboardDeviceIds.size === devicesDeviceIds.size &&
    [...dashboardDeviceIds].every((deviceId) => devicesDeviceIds.has(deviceId)),
  "dashboard and devices fixtures must cover the same managed device ids"
);

const issueCount = devices.devices.reduce((sum, row) => sum + row.issues.length, 0);
assert(
  dashboard.summary.open_issue_count === issueCount,
  "dashboard open_issue_count must match devices issue projection count"
);

for (const row of devices.devices) {
  assert(row.site_name, "device row must include site_name");
  assert(row.customer_display_name, "device row must include customer_display_name");
  assert(row.compact_location, "device row must include compact_location");
  assert(isKnownPrimaryAction(row.primary_action), `unknown primary action ${row.primary_action}`);

  for (const issue of row.issues) {
    assert(issue.label && issue.detail, "issue projection must carry display copy");
    assert(["critical", "warning", "info"].includes(issue.severity), "issue severity must be public");
    assert(isKnownPrimaryAction(issue.primary_action), `unknown issue primary action ${issue.primary_action}`);
  }
}

const publicCategories = new Set(["alert", "backup", "device", "update", "enrollment", "account"]);
for (const row of activity.activity) {
  assert(publicCategories.has(row.category), `activity category ${row.category} is not public`);
  assert(row.subject_label && row.detail && row.actor_label, "activity row must carry display copy");
}

console.log("Portal contract smoke passed");
