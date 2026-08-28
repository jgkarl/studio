// Desktop account-menu dropdown — a single click-to-open panel (just sign out; locale/theme/
// Settings are their own icon buttons in .navbar-controls) behind the navbar's user chip. Same
// hidden-panel/backdrop-click/Escape pattern as
// static/js/modal.js/lightbox.js, sized for exactly one panel. The mobile drawer already shows
// these controls inline and doesn't use this — only relevant at the >=1024px breakpoint where the
// account-menu-trigger button is visible at all (see .navbar-controls in app.css).
document.addEventListener("DOMContentLoaded", () => {
  const trigger = document.querySelector("[data-account-menu-open]");
  const panel = document.getElementById("account-menu");
  if (!trigger || !panel) return;

  function close() {
    panel.hidden = true;
  }

  trigger.addEventListener("click", (e) => {
    e.stopPropagation();
    panel.hidden = !panel.hidden;
  });

  document.addEventListener("click", (e) => {
    if (!panel.hidden && !panel.contains(e.target) && e.target !== trigger) close();
  });

  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape" && !panel.hidden) close();
  });
});
