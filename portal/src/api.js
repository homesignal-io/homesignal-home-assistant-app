import dashboardFixture from "../../testdata/contracts/api/public-v1/dashboard.json";
import devicesFixture from "../../testdata/contracts/api/public-v1/devices.json";
import activityFixture from "../../testdata/contracts/api/public-v1/activity.json";
import { getAccessToken, isAuthConfigured } from "./auth.js";

const apiBaseUrl = (import.meta.env.VITE_HOMESIGNAL_API_BASE_URL || "").replace(/\/$/, "");

async function readModel(path, fallback) {
  if (!apiBaseUrl) return fallback;

  try {
    const headers = {
      Accept: "application/json",
    };
    const apiToken = getAccessToken();
    if (apiToken) {
      headers.Authorization = `Bearer ${apiToken}`;
    }

    const response = await fetch(`${apiBaseUrl}${path}`, {
      headers,
    });

    if (response.status === 501) return fallback;
    if (response.status === 401 && isAuthConfigured()) {
      throw new Error(apiToken ? "Session expired or unauthorized." : "Sign in required.");
    }
    if (!response.ok) throw new Error(`HTTP ${response.status}`);

    return await response.json();
  } catch (error) {
    if (isAuthConfigured()) {
      throw error;
    }
    console.warn(`Falling back to contract fixture for ${path}:`, error);
    return fallback;
  }
}

export function getDashboard() {
  return readModel("/dashboard", dashboardFixture);
}

export function getDevices() {
  return readModel("/devices", devicesFixture);
}

export function getActivity() {
  return readModel("/activity", activityFixture);
}
