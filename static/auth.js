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
  return res
}

async function stopRecording() {
  const res = await authFetch("/api/stop_recording", {
    method: "POST",
  });

  if (!res.ok) {
    console.error("Failed to stop recording", res.status);
  }
  return res
}

async function getLiveVideo() {
  const refreshed = await tryRefreshTokens();
  if (!refreshed) {
    console.error("User not logged in:");
  }
  const token = getAccessToken();
  const videoElement = document.getElementById("video-feed");

  if (token) {
    videoElement.src = `/api/video_feed?access_token=${token}`;
  } else {
    console.error("User not logged in:");
    window.location.href = "/";
  }
}

async function checkLoggedIn() {
  const refreshed = await tryRefreshTokens();
  if (!refreshed) {
    console.error("User not logged in:");
  }
  const token = getAccessToken();

  if (token) {
    window.location.href = "/video_feed.html";
  }
}

async function loadVideoHistory() {
    const videoListContainer = document.getElementById('video-list');

    try {
        const response = await authFetch('/api/user/videos');
        if (!response.ok) throw new Error('Failed to fetch videos');
        
        const videos = await response.json();

        videoListContainer.innerHTML = '';

        videos.forEach(video => {
            const li = document.createElement('li');
            
            li.innerHTML = `
                <a href="/view/${video.id}" class="block px-4 py-2 rounded-lg hover:bg-white/5 transition-colors border border-transparent hover:border-white/10">
                    <div class="text-[12px] text-white font-medium">${video.date}</div>
                    <div class="text-[11px] text-gray-500 uppercase tracking-tight">${video.label}</div>
                </a>
            `;
            
            videoListContainer.appendChild(li);
        });

    } catch (error) {
        console.error("Error loading sidebar:", error);
        videoListContainer.innerHTML = '<p class="text-[10px] text-red-500">Failed to load lifts.</p>';
    }
}
