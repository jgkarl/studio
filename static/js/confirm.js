// Tiny vanilla-JS island, no dependency: any <form data-confirm="message"> asks before
// submitting — used by delete buttons app-wide instead of a bespoke confirm dialog per page.
document.addEventListener("submit", (e) => {
  const form = e.target;
  if (form instanceof HTMLFormElement && form.dataset.confirm && !window.confirm(form.dataset.confirm)) {
    e.preventDefault();
  }
});
