function showToast(message) {
  const toast = document.getElementById("toast");
  if (!toast) return;
  toast.textContent = message;
  toast.classList.add("show");
  clearTimeout(showToast.timer);
  showToast.timer = setTimeout(function () {
    toast.classList.remove("show");
  }, 1800);
}

async function copyText(text, successMessage) {
  if (!text) return;
  try {
    await navigator.clipboard.writeText(text);
    showToast(successMessage || "Copied");
  } catch (_error) {
    const input = document.createElement("textarea");
    input.value = text;
    document.body.appendChild(input);
    input.select();
    document.execCommand("copy");
    input.remove();
    showToast(successMessage || "Copied");
  }
}

document.addEventListener("click", function (event) {
  const trigger = event.target.closest("[data-copy]");
  if (!trigger) return;
  copyText(trigger.getAttribute("data-copy"), "Command copied");
});

function copyCode(button) {
  const block = button.closest(".code-block");
  if (!block) return;
  const code = block.querySelector("code");
  if (!code) return;
  copyText(code.innerText || code.textContent, "Code copied");
}

function copyLink() {
  copyText(window.location.href, "Link copied");
}

function toggleFaq(button) {
  const item = button.closest(".faq-item");
  if (!item) return;
  const alreadyOpen = item.classList.contains("open");
  document.querySelectorAll(".faq-item.open").forEach(function (node) {
    node.classList.remove("open");
  });
  if (!alreadyOpen) item.classList.add("open");
}

(function runHeroTerminal() {
  const root = document.getElementById("hero-terminal");
  if (!root) return;

  const command = "cat report.md | fastmd";
  const result = "🚀 https://fastmd.dev/x7y2";
  let cursorBlinkTimer = null;

  function create(type, text) {
    const line = document.createElement("div");
    line.className = type;
    line.textContent = text;
    return line;
  }

  function start() {
    if (cursorBlinkTimer) {
      clearInterval(cursorBlinkTimer);
      cursorBlinkTimer = null;
    }

    root.innerHTML = "";

    const commandLine = document.createElement("div");
    commandLine.className = "term-line";

    const prompt = document.createElement("span");
    prompt.className = "term-prompt";
    prompt.textContent = "$";

    const content = document.createElement("span");
    content.className = "term-command";

    commandLine.appendChild(prompt);
    commandLine.appendChild(content);
    root.appendChild(commandLine);

    let index = 0;

    function type() {
      if (index < command.length) {
        content.textContent = command.slice(0, index + 1);
        index += 1;
        setTimeout(type, 36);
        return;
      }

      setTimeout(function () {
        root.appendChild(create("term-output", result));

        const waitLine = document.createElement("div");
        waitLine.className = "term-line";

        const waitPrompt = document.createElement("span");
        waitPrompt.className = "term-prompt";
        waitPrompt.textContent = "$";

        const cursor = document.createElement("span");
        cursor.className = "term-cursor";

        waitLine.appendChild(waitPrompt);
        waitLine.appendChild(cursor);
        root.appendChild(waitLine);

        // Blink loop: visible 0.5s, hidden 0.5s.
        let cursorVisible = true;
        cursor.style.opacity = "1";
        cursorBlinkTimer = setInterval(function () {
          cursorVisible = !cursorVisible;
          cursor.style.opacity = cursorVisible ? "1" : "0";
        }, 500);

        setTimeout(start, 2400);
      }, 1000);
    }

    type();
  }

  setTimeout(start, 200);
})();

(function setupDashboard() {
  const tokenInput = document.getElementById("dashboard-token");
  if (!tokenInput) return;

  const loadButton = document.getElementById("dashboard-load");
  const saveButton = document.getElementById("dashboard-save-token");
  const clearButton = document.getElementById("dashboard-clear-token");
  const refreshButton = document.getElementById("dashboard-refresh");
  const status = document.getElementById("dashboard-status");
  const empty = document.getElementById("dashboard-empty");
  const docs = document.getElementById("dashboard-docs");
  const tokenKey = "fastmd.dashboard.token";

  function formatDate(ts) {
    const date = new Date(ts * 1000);
    if (Number.isNaN(date.getTime())) return "Unknown date";
    return date.toLocaleString();
  }

  function setStatus(message) {
    if (status) status.textContent = message;
  }

  function currentToken() {
    return (tokenInput.value || "").trim();
  }

  function renderDocs(items) {
    docs.innerHTML = "";
    if (!items.length) {
      empty.style.display = "block";
      empty.textContent = "No documents found for this token.";
      return;
    }

    empty.style.display = "none";
    items.forEach(function (doc) {
      const article = document.createElement("article");
      article.className = "dashboard-doc";

      const title = doc.title || doc.id;
      article.innerHTML =
        '<div class="dashboard-doc-head">' +
          '<div>' +
            '<h3 class="dashboard-doc-title"></h3>' +
            '<div class="dashboard-doc-meta">' +
              '<span></span>' +
              '<span></span>' +
            '</div>' +
          '</div>' +
          '<div class="dashboard-doc-actions">' +
            '<button class="btn-action" type="button">Copy URL</button>' +
            '<button class="btn-action" type="button">Delete</button>' +
          '</div>' +
        '</div>' +
        '<a class="dashboard-doc-link" target="_blank" rel="noreferrer"></a>';

      article.querySelector(".dashboard-doc-title").textContent = title;
      article.querySelector(".dashboard-doc-meta span:first-child").textContent = "ID: " + doc.id;
      article.querySelector(".dashboard-doc-meta span:last-child").textContent = "Created: " + formatDate(doc.created_at);

      const link = article.querySelector(".dashboard-doc-link");
      link.href = doc.url;
      link.textContent = doc.url;

      const copyBtn = article.querySelectorAll(".btn-action")[0];
      const deleteBtn = article.querySelectorAll(".btn-action")[1];

      copyBtn.addEventListener("click", function () {
        copyText(doc.url, "Link copied");
      });

      deleteBtn.addEventListener("click", async function () {
        if (!window.confirm("Delete this document?")) return;
        try {
          const response = await fetch("/v1/" + encodeURIComponent(doc.id), {
            method: "DELETE",
            headers: {
              Authorization: "Bearer " + currentToken()
            }
          });

          if (!response.ok) {
            const body = await response.text();
            throw new Error(body || "Delete failed");
          }

          showToast("Document deleted");
          loadDocs();
        } catch (error) {
          showToast("Delete failed");
        }
      });

      docs.appendChild(article);
    });
  }

  async function loadDocs() {
    const token = currentToken();
    if (!token) {
      setStatus("Paste a token to load documents.");
      docs.innerHTML = "";
      empty.style.display = "block";
      empty.textContent = "No documents loaded yet.";
      return;
    }

    setStatus("Loading documents...");
    try {
      const response = await fetch("/v1/docs", {
        headers: {
          Authorization: "Bearer " + token
        }
      });

      if (!response.ok) {
        const body = await response.text();
        throw new Error(body || "Request failed");
      }

      const data = await response.json();
      const items = Array.isArray(data.documents) ? data.documents : [];
      setStatus(items.length ? "Documents loaded." : "No documents found.");
      renderDocs(items);
    } catch (_error) {
      docs.innerHTML = "";
      empty.style.display = "block";
      empty.textContent = "Failed to load documents.";
      setStatus("Check the token and try again.");
      showToast("Load failed");
    }
  }

  const savedToken = window.localStorage.getItem(tokenKey);
  if (savedToken) {
    tokenInput.value = savedToken;
    loadDocs();
  }

  loadButton.addEventListener("click", loadDocs);
  refreshButton.addEventListener("click", loadDocs);
  saveButton.addEventListener("click", function () {
    const token = currentToken();
    if (!token) {
      showToast("Token is empty");
      return;
    }
    window.localStorage.setItem(tokenKey, token);
    showToast("Token saved");
  });
  clearButton.addEventListener("click", function () {
    window.localStorage.removeItem(tokenKey);
    tokenInput.value = "";
    docs.innerHTML = "";
    empty.style.display = "block";
    empty.textContent = "No documents loaded yet.";
    setStatus("Paste a token to load documents.");
    showToast("Token cleared");
  });
})();
