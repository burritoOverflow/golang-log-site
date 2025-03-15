const logContainer = document.getElementById("log-container");

function colorizeLog(line) {
  if (
    line.includes("ERROR") ||
    line.includes("CRITICAL") ||
    line.includes("FATAL")
  ) {
    return '<div class="error">' + line + "</div>";
  } else if (line.includes("WARN") || line.includes("WARNING")) {
    return '<div class="warning">' + line + "</div>";
  } else if (line.includes("INFO")) {
    return '<div class="info">' + line + "</div>";
  } else if (line.includes("DEBUG")) {
    return '<div class="debug">' + line + "</div>";
  }
  return "<div>" + line + "</div>";
}

// Load initial content
fetch("/content")
  .then((response) => response.text())
  .then((data) => {
    const lines = data.split("\n");
    let html = "";
    for (const line of lines) {
      if (line.trim()) {
        html += colorizeLog(line);
      }
    }
    logContainer.innerHTML = html;
    logContainer.scrollTop = logContainer.scrollHeight;
  });

// Connect to SSE endpoint
const evtSource = new EventSource("/logs");
evtSource.onmessage = function (event) {
  const newLine = event.data;
  const newElement = document.createElement("div");
  newElement.innerHTML = colorizeLog(newLine);
  logContainer.appendChild(newElement);

  // Auto-scroll to bottom if already at bottom
  const isScrolledToBottom =
    logContainer.scrollHeight - logContainer.clientHeight <=
    logContainer.scrollTop + 100;
  if (isScrolledToBottom) {
    logContainer.scrollTop = logContainer.scrollHeight;
  }
};

evtSource.onerror = function () {
  console.log("SSE connection error. Reconnecting...");
};
