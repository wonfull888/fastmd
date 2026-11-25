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

function copyFullText() {
  var el = document.querySelector(".markdown-body");
  if (el) {
    copyText(el.innerText || el.textContent, "Full text copied");
  }
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
  const root = document.getElementById("account-dashboard");
  if (!root) return;
  let csrf = "";
  let cursor = "";
  let pending = false;
  const status = document.getElementById("account-status");
  const documents = document.getElementById("account-documents");
  const tokenText = document.getElementById("account-token");
  let visibleToken = "";
  const tokenButton = document.getElementById("toggle-token");
  const mask = "fmd_live_••••••••••••••••••••••••";
  document.getElementById("mcp-endpoint").textContent = window.location.origin + "/mcp";
  async function api(path, method = "GET", body) {
    const headers = {};
    if (method !== "GET") headers["X-CSRF-Token"] = csrf;
    if (body !== undefined) headers["Content-Type"] = "application/json";
    const response = await fetch(path, {method, headers, body: body === undefined ? undefined : JSON.stringify(body)});
    const data = await response.json();
    if (!response.ok) {
      if (response.status === 401) window.location.assign("/login?next=/dashboard");
      throw new Error(data.error || "Request failed");
    }
    return data;
  }
  function button(label, action) {
    const element = document.createElement("button");
    element.className = "btn-action";
    element.type = "button";
    element.textContent = label;
    element.addEventListener("click", async function () {
      element.disabled = true;
      try { await action(); } catch (error) { status.textContent = error.message; }
      finally { element.disabled = false; }
    });
    return element;
  }
  async function loadDocuments(reset) {
    if (reset) { cursor = ""; documents.replaceChildren(); }
    const data = await api("/v1/docs?limit=20&cursor=" + encodeURIComponent(cursor));
    data.documents.forEach(function (doc) {
      const article = document.createElement("article");
      article.className = "dashboard-doc";
      const link = document.createElement("a");
      link.href = doc.url; link.textContent = doc.title || doc.id;
      link.target = "_blank"; link.rel = "noreferrer";
      article.append(link, button("Copy URL", function () { return copyText(doc.url); }), button("Delete", async function () {
        if (!window.confirm("Delete this document?")) return;
        await api("/v1/" + encodeURIComponent(doc.id), "DELETE");
        await refresh();
      }));
      documents.appendChild(article);
    });
    cursor = data.next_cursor || "";
    document.getElementById("account-more").hidden = !cursor;
    if (!documents.childElementCount) documents.textContent = "No documents yet.";
  }
  async function loadCredentials() {
    const connections = await api("/v1/connections");
    const target = document.getElementById("account-connections"); target.replaceChildren();
    connections.connections.forEach(function (connection) {
      const row = document.createElement("div"); row.className = "dashboard-doc";
      const label = document.createElement("span"); label.textContent = connection.name + " · " + connection.scopes + (connection.revoked_at ? " · Revoked" : ""); row.appendChild(label);
      if (!connection.revoked_at) row.appendChild(button("Disconnect", async function () {
        if (!window.confirm("Disconnect this MCP client?")) return;
        await api("/v1/connections/" + encodeURIComponent(connection.id), "DELETE"); await loadCredentials();
      }));
      target.appendChild(row);
    });
    if (!connections.connections.length) target.textContent = "No connected MCP clients.";
  }
  async function refresh() {
    if (pending) return;
    pending = true; status.textContent = "Loading…";
    try {
      const me = await api("/v1/auth/me"); csrf = me.csrf_token;
      document.getElementById("account-email").textContent = me.account.github_login;
      const usage = await api("/v1/usage");
      document.getElementById("account-usage").textContent = usage.document_count + " documents";
      await loadDocuments(true); await loadCredentials(); status.textContent = "";
    } catch (error) { status.textContent = error.message; }
    finally { pending = false; }
  }
  document.getElementById("account-refresh").addEventListener("click", refresh);
  document.getElementById("account-more").addEventListener("click", function () { loadDocuments(false).catch(function(error) { status.textContent = error.message; }); });
  document.getElementById("account-logout").addEventListener("click", async function () {
    try { await api("/v1/auth/logout", "POST", {}); window.location.assign("/login"); }
    catch (error) { status.textContent = error.message; }
  });
  tokenButton.addEventListener("click", async function () {
    tokenButton.disabled = true;
    try {
      if (visibleToken) { visibleToken = ""; tokenText.textContent = mask; tokenButton.textContent = "Show token"; tokenButton.setAttribute("aria-expanded", "false"); }
      else { const result = await api("/v1/account/token/reveal", "POST", {}); visibleToken = result.token; tokenText.textContent = visibleToken; tokenButton.textContent = "Hide token"; tokenButton.setAttribute("aria-expanded", "true"); }
    } catch (error) { status.textContent = error.message; }
    finally { tokenButton.disabled = false; }
  });
  document.getElementById("copy-token").addEventListener("click", async function () {
    try { const result = await api("/v1/account/token/reveal", "POST", {}); await copyText(result.token); status.textContent = "Token copied."; }
    catch (error) { status.textContent = error.message; }
  });
  window.addEventListener("pagehide", function () { visibleToken = ""; tokenText.textContent = mask; });
  refresh();
})();

// ── escapeHTML helper ──────────────────────────────────────────────
function escapeHTML(str) {
  var div = document.createElement("div");
  div.appendChild(document.createTextNode(str));
  return div.innerHTML;
}

// ── Read Aloud (Web Speech API) ─────────────────────────────────────
(function setupReadAloud() {
  var btn = document.getElementById("btn-read-aloud");
  if (!btn || !window.speechSynthesis) return; // silent hide (M-9)

  btn.style.display = "";

  var synth = window.speechSynthesis;
  var utterance = null;
  var state = "idle"; // idle | playing | paused

  function getTextContent() {
    var content = document.querySelector(".doc-content");
    if (!content) return "";

    // Walk DOM, skip code blocks and pre elements
    var parts = [];
    function walk(node) {
      if (node.nodeType === 3) { // text node
        var t = node.textContent.trim();
        if (t) parts.push(t);
        return;
      }
      if (node.nodeType !== 1) return;
      var tag = node.tagName ? node.tagName.toLowerCase() : "";
      if (tag === "pre" || tag === "code") return;
      for (var i = 0; i < node.childNodes.length; i++) {
        walk(node.childNodes[i]);
      }
    }
    walk(content);
    return parts.join(". ") + ".";
  }

  function stop() {
    if (utterance) {
      utterance.onend = null;
      utterance.onerror = null;
    }
    synth.cancel();
    utterance = null;
    state = "idle";
    btn.textContent = "Read aloud";
  }

  function speak(text) {
    stop();
    utterance = new SpeechSynthesisUtterance(text);
    utterance.rate = 0.95;
    utterance.onend = function () {
      state = "idle";
      btn.textContent = "Read aloud";
    };
    utterance.onerror = function () {
      state = "idle";
      btn.textContent = "Read aloud";
    };
    synth.speak(utterance);
    state = "playing";
    btn.textContent = "Pause";
  }

  window.toggleReadAloud = function () {
    if (state === "idle") {
      speak(getTextContent());
    } else if (state === "playing") {
      synth.pause();
      state = "paused";
      btn.textContent = "Continue";
    } else if (state === "paused") {
      synth.resume();
      state = "playing";
      btn.textContent = "Pause";
    }
  };

  // Stop on page unload
  window.addEventListener("beforeunload", stop);
})();
