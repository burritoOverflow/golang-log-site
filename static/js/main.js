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

const evtSource = new EventSource("/logs");

evtSource.onopen = function () {
  console.info("SSE connection established");
};

evtSource.onmessage = function (event) {
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

evtSource.onerror = function () {
  console.error("SSE connection error. Reconnecting...");
};
