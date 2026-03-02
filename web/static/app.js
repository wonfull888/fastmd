// Copy install command
function copyInstall() {
    const cmd = document.getElementById('install-cmd');
    if (cmd) {
        navigator.clipboard.writeText(cmd.textContent).then(() => {
            const btn = document.querySelector('.copy-btn');
            const orig = btn.innerHTML;
            btn.innerHTML = '✓';
            btn.style.color = '#3fb950';
            setTimeout(() => { btn.innerHTML = orig; btn.style.color = ''; }, 2000);
        });
    }
}

// Copy current page link
function copyLink() {
    navigator.clipboard.writeText(window.location.href).then(() => {
        const btn = document.querySelector('.btn-copy-link');
        const orig = btn.textContent;
        btn.textContent = 'Copied!';
        btn.style.color = '#3fb950';
        setTimeout(() => { btn.textContent = orig; btn.style.color = ''; }, 2000);
    });
}
