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
