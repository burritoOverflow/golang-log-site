const logContainer = document.getElementById("log-container");

function colorizeLog(line) {
  const div = document.createElement("div");
  div.textContent = line;
  div.classList.add("log-line");

  if (
    line.includes("ERROR") ||
    line.includes("CRITICAL") ||
    line.includes("FATAL")
  ) {
    div.classList.add("error");
  } else if (line.includes("WARN") || line.includes("WARNING")) {
    div.classList.add("warning");
  } else if (line.includes("INFO")) {
    div.classList.add("info");
  } else if (line.includes("DEBUG")) {
    div.classList.add("debug");
  }

  return div;
}

fetch("/content")
  .then((response) => response.text())
  .then((data) => {
    const lines = data.split("\n");
    logContainer.innerHTML = "";

    for (const line of lines) {
      if (line.trim()) {
        logContainer.appendChild(colorizeLog(line));
      }
    }
    logContainer.scrollTop = logContainer.scrollHeight;
  });

let eventSource = null;
let reconnectTimeout = 1000; // Start with 1 second; adjust via exponential backoff
const maxReconnectTimeout = 30000; // Maximum 30 seconds
let reconnectAttempts = 0;
const maxReconnectAttempts = 10;

function connectEventSource() {
  if (eventSource) {
    eventSource.close();
  }

  eventSource = new EventSource("/logs");

  eventSource.onopen = function () {
    console.info("SSE connection established");
    // Reset reconnection timeout on successful connection
    reconnectTimeout = 1000;
    reconnectAttempts = 0;
  };

  eventSource.onmessage = function (event) {
    const newLine = event.data;
    logContainer.appendChild(colorizeLog(newLine));

    // Auto-scroll to bottom if already at bottom
    const isScrolledToBottom =
      logContainer.scrollHeight - logContainer.clientHeight <=
      logContainer.scrollTop + 100;

    if (isScrolledToBottom) {
      logContainer.scrollTop = logContainer.scrollHeight;
    }
  };

  eventSource.onerror = function () {
    console.error("SSE connection error. Reconnecting...");
    eventSource.close();

    reconnectAttempts++;
    if (reconnectAttempts <= maxReconnectAttempts) {
      console.info(
        `Reconnection attempt ${reconnectAttempts} in ${
          reconnectTimeout / 1000
        } seconds...`
      );

      setTimeout(() => {
        connectEventSource();
        // Implement exponential backoff for reconnection
        reconnectTimeout = Math.min(reconnectTimeout * 2, maxReconnectTimeout);
      }, reconnectTimeout);
    } else {
      console.error(
        `Failed to reconnect after ${maxReconnectAttempts} attempts. Please refresh the page.`
      );

      const errorDiv = document.createElement("div");
      errorDiv.classList.add("connection-error");
      errorDiv.textContent = "Connection lost. Please refresh the page.";
      logContainer.prepend(errorDiv);
    }
  };
}

// init
connectEventSource();

document.addEventListener("visibilitychange", () => {
  if (
    document.visibilityState === "visible" &&
    (!eventSource || eventSource.readyState === EventSource.CLOSED)
  ) {
    console.info("Page became visible again, reconnecting SSE...");
    reconnectAttempts = 0; // Reset reconnect attempts
    reconnectTimeout = 1000; // Reset timeout
    connectEventSource();
  }
});
