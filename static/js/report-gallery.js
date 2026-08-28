// Report Image gallery — native HTML5 Drag and Drop API (same approach as kanban.js), reordering
// within one grid instead of across kanban-style columns: each photo is draggable, and dropping it
// onto another photo inserts it before/after that one (whichever side of it the pointer's on),
// then the whole new order posts in one request. Also drives the gallery's own "Columns" select
// (data-autosubmit-on-change), which has no separate JS home of its own worth a whole file for.
document.addEventListener("DOMContentLoaded", () => {
  document.querySelectorAll("[data-autosubmit-on-change] select").forEach((select) => {
    select.addEventListener("change", () => select.form.requestSubmit());
  });

  const grid = document.getElementById("report-gallery-grid");
  if (!grid) return;
  const reorderUrl = grid.dataset.reorderUrl;

  function persistOrder() {
    const refIds = Array.from(grid.querySelectorAll(".gallery-item")).map((el) => el.dataset.refId);
    fetch(reorderUrl, {
      method: "POST",
      headers: {
        "Content-Type": "application/x-www-form-urlencoded",
        "X-Requested-With": "fetch",
      },
      body: refIds.map((id) => "refId=" + encodeURIComponent(id)).join("&"),
    }).then((res) => {
      if (!res.ok) window.location.reload();
    });
  }

  let draggingItem = null;

  grid.querySelectorAll(".gallery-item").forEach((item) => {
    item.setAttribute("draggable", "true");

    item.addEventListener("dragstart", (e) => {
      draggingItem = item;
      e.dataTransfer.effectAllowed = "move";
      e.dataTransfer.setData("text/plain", item.dataset.refId);
      item.classList.add("is-dragging");
    });

    item.addEventListener("dragend", () => {
      item.classList.remove("is-dragging");
      draggingItem = null;
    });

    item.addEventListener("dragover", (e) => {
      if (!draggingItem || draggingItem === item) return;
      e.preventDefault();
      e.dataTransfer.dropEffect = "move";
      const rect = item.getBoundingClientRect();
      const before = e.clientX - rect.left < rect.width / 2;
      grid.insertBefore(draggingItem, before ? item : item.nextSibling);
    });

    item.addEventListener("drop", (e) => {
      e.preventDefault();
      if (draggingItem) persistOrder();
    });
  });
});
