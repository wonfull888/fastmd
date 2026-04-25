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
  const authSection = document.getElementById("dashboard-auth");
  if (!authSection) return;

  const listSection  = document.getElementById("dashboard-list");
  const tokenInput   = document.getElementById("dashboard-token");
  const loadButton   = document.getElementById("dashboard-load");
  const refreshButton = document.getElementById("dashboard-refresh");
  const switchButton = document.getElementById("dashboard-switch");
  const statusEl     = document.getElementById("dashboard-status");
  const emptyEl      = document.getElementById("dashboard-empty");
  const docsEl       = document.getElementById("dashboard-docs");
  const cookieName   = "fastmd_token";
  const cookieMaxAge = 60 * 60 * 24 * 365;

  // ── cookie helpers ──────────────────────────────────────────────
  function getCookie() {
    const match = document.cookie.split(";").map(s => s.trim())
      .find(s => s.startsWith(cookieName + "="));
    return match ? decodeURIComponent(match.split("=")[1]) : "";
  }

  function setCookie(token) {
    document.cookie = cookieName + "=" + encodeURIComponent(token) +
      ";path=/;max-age=" + cookieMaxAge + ";SameSite=Lax";
  }

  function clearCookie() {
    document.cookie = cookieName + "=;path=/;max-age=0";
  }

  // ── view helpers ─────────────────────────────────────────────────
  function showAuth() {
    authSection.style.display = "";
    listSection.style.display = "none";
  }

  function showList() {
    authSection.style.display = "none";
    listSection.style.display = "";
  }

  function setStatus(msg) {
    if (statusEl) statusEl.textContent = msg;
  }

  // ── active token (from cookie only after first load) ─────────────
  var activeToken = "";

  // ── render ───────────────────────────────────────────────────────
  function formatDate(ts) {
    const d = new Date(ts * 1000);
    return Number.isNaN(d.getTime()) ? "Unknown date" : d.toLocaleString();
  }

  function renderDocs(items) {
    docsEl.innerHTML = "";
    if (!items.length) {
      emptyEl.style.display = "block";
      return;
    }
    emptyEl.style.display = "none";
    items.forEach(function (doc) {
      const article = document.createElement("article");
      article.className = "dashboard-doc";
      const title = doc.title || doc.id;
      article.innerHTML =
        '<div class="dashboard-doc-head">' +
          '<div>' +
            '<h3 class="dashboard-doc-title"></h3>' +
            '<div class="dashboard-doc-meta">' +
              '<span></span><span></span>' +
            '</div>' +
          '</div>' +
          '<div class="dashboard-doc-actions">' +
            '<button class="btn-action" type="button">Copy URL</button>' +
            '<button class="btn-action btn-danger" type="button">Delete</button>' +
          '</div>' +
        '</div>' +
        '<a class="dashboard-doc-link" target="_blank" rel="noreferrer"></a>';

      article.querySelector(".dashboard-doc-title").textContent = title;
      article.querySelector(".dashboard-doc-meta span:first-child").textContent = "ID: " + doc.id;
      article.querySelector(".dashboard-doc-meta span:last-child").textContent = "Created: " + formatDate(doc.created_at);
      const link = article.querySelector(".dashboard-doc-link");
      link.href = doc.url;
      link.textContent = doc.url;

      article.querySelectorAll(".btn-action")[0].addEventListener("click", function () {
        copyText(doc.url, "Link copied");
      });
      article.querySelectorAll(".btn-action")[1].addEventListener("click", async function () {
        if (!window.confirm("Delete this document?")) return;
        try {
          const res = await fetch("/v1/" + encodeURIComponent(doc.id), {
            method: "DELETE",
            headers: { Authorization: "Bearer " + activeToken }
          });
          if (!res.ok) throw new Error();
          showToast("Document deleted");
          loadDocs();
        } catch (_) {
          showToast("Delete failed");
        }
      });
      docsEl.appendChild(article);
    });
  }

  // ── load docs ────────────────────────────────────────────────────
  async function loadDocs() {
    setStatus("Loading...");
    showList();
    try {
      const res = await fetch("/v1/docs", {
        headers: { Authorization: "Bearer " + activeToken }
      });
      if (!res.ok) throw new Error();
      const data = await res.json();
      const items = Array.isArray(data.documents) ? data.documents : [];
      setStatus(items.length + " document" + (items.length === 1 ? "" : "s"));
      renderDocs(items);
    } catch (_) {
      showToast("Load failed — check your token");
      showAuth();
    }
  }

  // ── enter dashboard with token ───────────────────────────────────
  function enterWithToken(token) {
    activeToken = token;
    setCookie(token);
    loadDocs();
  }

  // ── bootstrap: check URL param → cookie → show auth ─────────────
  const params = new URLSearchParams(window.location.search);
  const urlToken = params.get("token");

  if (urlToken) {
    // Clean token from URL bar so it's not visible or shareable
    params.delete("token");
    const newUrl = window.location.pathname + (params.toString() ? "?" + params.toString() : "");
    history.replaceState(null, "", newUrl);
    enterWithToken(urlToken);
  } else {
    const cookieToken = getCookie();
    if (cookieToken) {
      activeToken = cookieToken;
      loadDocs();
    } else {
      showAuth();
    }
  }

  // ── events ───────────────────────────────────────────────────────
  loadButton.addEventListener("click", function () {
    const t = (tokenInput.value || "").trim();
    if (!t) { showToast("Token is empty"); return; }
    enterWithToken(t);
  });

  tokenInput.addEventListener("keydown", function (e) {
    if (e.key === "Enter") loadButton.click();
  });

  refreshButton.addEventListener("click", function () {
    if (activeToken) loadDocs();
  });

  switchButton.addEventListener("click", function () {
    clearCookie();
    activeToken = "";
    tokenInput.value = "";
    docsEl.innerHTML = "";
    emptyEl.style.display = "none";
    showAuth();
  });
})();
