// Generic modal — generalizes lightbox.js's hidden-overlay pattern to work with multiple named
// modals on one page (an asset detail page can have up to five: record-condition, +Treatment,
// +Report, +Project, +Media). No "close after successful submit" logic is needed: every form a
// modal holds posts to a handler that redirects back on success, so a fresh page load starts
// every modal hidden again for free.
//
// Markup contract:
//   <button data-modal-open="my-modal">+ Add</button>
//   <div id="my-modal" class="modal-overlay" hidden>
//     <div class="modal-panel">
//       <button data-modal-close aria-label="Close">×</button>
//       ...
//     </div>
//   </div>
document.addEventListener("DOMContentLoaded", () => {
  function closeOverlay(overlay) {
    overlay.hidden = true;
    document.body.style.overflow = "";
  }

  document.querySelectorAll("[data-modal-open]").forEach((trigger) => {
    const overlay = document.getElementById(trigger.dataset.modalOpen);
    if (!overlay) return;
    trigger.addEventListener("click", () => {
      overlay.hidden = false;
      document.body.style.overflow = "hidden";
    });
  });

  document.querySelectorAll(".modal-overlay").forEach((overlay) => {
    overlay.querySelectorAll("[data-modal-close]").forEach((btn) => {
      btn.addEventListener("click", () => closeOverlay(overlay));
    });
    overlay.addEventListener("click", (e) => {
      if (e.target === overlay) closeOverlay(overlay);
    });
  });

  document.addEventListener("keydown", (e) => {
    if (e.key !== "Escape") return;
    document.querySelectorAll(".modal-overlay").forEach((overlay) => {
      if (!overlay.hidden) closeOverlay(overlay);
    });
  });
});
