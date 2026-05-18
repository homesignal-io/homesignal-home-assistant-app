const envToken = import.meta.env.VITE_HOMESIGNAL_API_TOKEN || "";
const cognitoDomain = (import.meta.env.VITE_HOMESIGNAL_COGNITO_DOMAIN || "").replace(/\/$/, "");
const cognitoClientId = import.meta.env.VITE_HOMESIGNAL_COGNITO_CLIENT_ID || "";
const configuredRedirectUri = import.meta.env.VITE_HOMESIGNAL_COGNITO_REDIRECT_URI || "";
const configuredLogoutUri = import.meta.env.VITE_HOMESIGNAL_COGNITO_LOGOUT_URI || "";

const tokenStorageKey = "homesignal.portal.auth.tokens";
const pkceStorageKey = "homesignal.portal.auth.pkce";

export function isAuthConfigured() {
  return Boolean(cognitoDomain && cognitoClientId);
}

export function getAuthState() {
  const token = getAccessToken();
  return {
    configured: isAuthConfigured(),
    signedIn: Boolean(token),
    accessToken: token,
  };
}

export function getAccessToken() {
  if (envToken) return envToken;

  const tokens = readTokens();
  if (!tokens?.access_token) return "";
  if (tokens.expires_at && Date.now() > tokens.expires_at - 30_000) {
    clearTokens();
    return "";
  }
  return tokens.access_token;
}

export async function completeSignInFromLocation() {
  if (!isAuthConfigured() || typeof window === "undefined") return getAuthState();

  const params = new URLSearchParams(window.location.search);
  const code = params.get("code");
  const state = params.get("state");
  if (!code) return getAuthState();

  const pkce = readJSON(sessionStorage.getItem(pkceStorageKey));
  sessionStorage.removeItem(pkceStorageKey);
  if (!pkce?.state || !pkce?.verifier || pkce.state !== state) {
    throw new Error("Sign-in state could not be verified.");
  }

  const body = new URLSearchParams({
    grant_type: "authorization_code",
    client_id: cognitoClientId,
    code,
    redirect_uri: redirectUri(),
    code_verifier: pkce.verifier,
  });

  const response = await fetch(`${cognitoDomain}/oauth2/token`, {
    method: "POST",
    headers: {
      "Content-Type": "application/x-www-form-urlencoded",
    },
    body,
  });
  if (!response.ok) {
    throw new Error(`Sign-in failed with HTTP ${response.status}.`);
  }

  const tokens = await response.json();
  const expiresIn = Number(tokens.expires_in || 3600);
  sessionStorage.setItem(
    tokenStorageKey,
    JSON.stringify({
      ...tokens,
      expires_at: Date.now() + expiresIn * 1000,
    })
  );
  window.history.replaceState(null, "", window.location.pathname + (window.location.hash || "#page=Dashboard"));
  return getAuthState();
}

export async function beginSignIn() {
  if (!isAuthConfigured()) return;

  const verifier = randomString(64);
  const challenge = await pkceChallenge(verifier);
  const state = randomString(32);
  sessionStorage.setItem(pkceStorageKey, JSON.stringify({ verifier, state }));

  const params = new URLSearchParams({
    client_id: cognitoClientId,
    response_type: "code",
    scope: "openid email profile",
    redirect_uri: redirectUri(),
    code_challenge_method: "S256",
    code_challenge: challenge,
    state,
  });
  window.location.assign(`${cognitoDomain}/oauth2/authorize?${params.toString()}`);
}

export function signOut() {
  clearTokens();
  if (!isAuthConfigured()) return;

  const params = new URLSearchParams({
    client_id: cognitoClientId,
    logout_uri: logoutUri(),
  });
  window.location.assign(`${cognitoDomain}/logout?${params.toString()}`);
}

function redirectUri() {
  if (configuredRedirectUri) return configuredRedirectUri;
  return window.location.origin + window.location.pathname;
}

function logoutUri() {
  if (configuredLogoutUri) return configuredLogoutUri;
  return window.location.origin + window.location.pathname;
}

function readTokens() {
  if (typeof sessionStorage === "undefined") return null;
  return readJSON(sessionStorage.getItem(tokenStorageKey));
}

function clearTokens() {
  if (typeof sessionStorage === "undefined") return;
  sessionStorage.removeItem(tokenStorageKey);
}

function readJSON(value) {
  if (!value) return null;
  try {
    return JSON.parse(value);
  } catch {
    return null;
  }
}

function randomString(length) {
  const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-._~";
  const bytes = new Uint8Array(length);
  crypto.getRandomValues(bytes);
  return Array.from(bytes, (byte) => alphabet[byte % alphabet.length]).join("");
}

async function pkceChallenge(verifier) {
  const encoded = new TextEncoder().encode(verifier);
  const digest = await crypto.subtle.digest("SHA-256", encoded);
  return base64Url(digest);
}

function base64Url(buffer) {
  const bytes = new Uint8Array(buffer);
  let binary = "";
  for (const byte of bytes) {
    binary += String.fromCharCode(byte);
  }
  return btoa(binary).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/, "");
}
