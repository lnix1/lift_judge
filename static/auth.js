let accessToken = null;

function getAccessToken() {
  return accessToken;
}

function setAccessToken(token) {
  accessToken = token;
}

function clearTokens() {
  accessToken = null;
}

async function authFetch(input, init = {}) {
  let token = getAccessToken();

  const firstResponse = await fetch(input, withAuth(init, token));

  if (firstResponse.status !== 401) {
    return firstResponse;
  }

  // If 401, try to refresh using the HTTP-only refresh cookie
  const refreshed = await tryRefreshTokens();
  if (!refreshed) {
    handleLogout();
    throw new Error("Session expired. Please log in again.");
  }

  // Retry with new access token
  token = getAccessToken();
  return fetch(input, withAuth(init, token));
}

function withAuth(init, token) {
  const headers = new Headers(init.headers || {});
  if (token) {
    headers.set("Authorization", `Bearer ${token}`);
  }
  return Object.assign({}, init, { headers });
}

async function tryRefreshTokens() {
  const res = await fetch("/api/refresh", {
    method: "POST",
  });

  if (!res.ok) {
    return false;
  }

  const data = await res.json();
  if (!data.token) {
    return false;
  }

  setAccessToken(data.token);
  return true;
}

function handleLogout() {
  clearTokens();
  // Optional: have a logout endpoint that clears the refresh cookie server-side
  // await fetch("/api/logout", { method: "POST", credentials: "include" });

  window.location.href = "/";
}

async function startRecording() {
  const res = await authFetch("/api/start_recording", {
    method: "POST",
  });

  if (!res.ok) {
    console.error("Failed to start recording", res.status);
  }
}

async function stopRecording() {
  const res = await authFetch("/api/stop_recording", {
    method: "POST",
  });

  if (!res.ok) {
    console.error("Failed to stop recording", res.status);
  }
}

async function getLiveVideo() {
  const refreshed = await tryRefreshTokens();
  const token = getAccessToken();
  const videoElement = document.getElementById("video-feed");

  if (token) {
    videoElement.src = `/api/video_feed?access_token=${token}`;
  } else {
    console.error("User not logged in:");
    window.location.href = "/";
  }
}
