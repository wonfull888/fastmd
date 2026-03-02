function showToast(message) {
  const toast = document.getElementById("toast");
  if (!toast) return;
  toast.textContent = message;
  toast.classList.add("show");
  clearTimeout(showToast._timer);
  showToast._timer = setTimeout(function () {
    toast.classList.remove("show");
  }, 1800);
}

async function copyText(text, successMessage) {
  if (!text) return;
  try {
    await navigator.clipboard.writeText(text);
    showToast(successMessage || "Copied");
  } catch (_err) {
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
  const copyTrigger = event.target.closest("[data-copy]");
  if (!copyTrigger) return;
  copyText(copyTrigger.getAttribute("data-copy"), "Command copied");
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
  const isOpen = item.classList.contains("open");
  document.querySelectorAll(".faq-item.open").forEach(function (openItem) {
    openItem.classList.remove("open");
  });
  if (!isOpen) {
    item.classList.add("open");
  }
}

(function runHeroTerminal() {
  const terminal = document.getElementById("hero-terminal");
  if (!terminal) return;

  const command = "cat report.md | fastmd";
  const output = "🚀 https://fastmd.dev/x7y2";

  function createLine(className) {
    const line = document.createElement("div");
    line.className = className;
    return line;
  }

  function startCycle() {
    terminal.innerHTML = "";

    const commandLine = createLine("term-line");
    const prompt = document.createElement("span");
    prompt.className = "term-prompt";
    prompt.textContent = "$";

    const commandText = document.createElement("span");
    commandText.className = "term-command";

    commandLine.appendChild(prompt);
    commandLine.appendChild(commandText);
    terminal.appendChild(commandLine);

    let index = 0;

    function typeCommand() {
      if (index < command.length) {
        commandText.textContent = command.slice(0, index + 1);
        index += 1;
        setTimeout(typeCommand, 40);
        return;
      }

      setTimeout(function () {
        const outputLine = createLine("term-output");
        outputLine.textContent = output;
        terminal.appendChild(outputLine);

        const waitingLine = createLine("term-line");
        const waitingPrompt = document.createElement("span");
        waitingPrompt.className = "term-prompt";
        waitingPrompt.textContent = "$";

        const cursor = document.createElement("span");
        cursor.className = "term-cursor";

        waitingLine.appendChild(waitingPrompt);
        waitingLine.appendChild(cursor);
        terminal.appendChild(waitingLine);

        setTimeout(startCycle, 2600);
      }, 1000);
    }

    typeCommand();
  }

  setTimeout(startCycle, 260);
})();
