// Generic kanban board — native HTML5 Drag and Drop API, no dependency. Common drag-and-drop
// libraries (e.g. dnd-kit) are framework-bound with no vanilla build, so this is a small vanilla
// island instead of vendoring a framework just for one board. Reused by every kanban
// in the app (currently just Projects); the board element carries the per-board config via data
// attributes so this script itself never hardcodes an entity or route:
//   data-url-template="/projects/{id}/stage"  — {id} is replaced with the dragged card's item id
//   data-status-field="stage"                 — the form field name the new column code posts as
// Each card carries data-item-id; each column (header + body) carries data-status.
document.addEventListener("DOMContentLoaded", () => {
  const board = document.getElementById("kanban-board");
  if (!board) return;

  const urlTemplate = board.dataset.urlTemplate;
  const statusField = board.dataset.statusField;

  let draggingCard = null;

  board.querySelectorAll(".kanban-card").forEach((card) => {
    card.addEventListener("dragstart", (e) => {
      draggingCard = card;
      e.dataTransfer.effectAllowed = "move";
      e.dataTransfer.setData("text/plain", card.dataset.itemId);
      card.classList.add("is-dragging");
    });
    card.addEventListener("dragend", () => {
      card.classList.remove("is-dragging");
      draggingCard = null;
    });
    // Whole-card click navigates to the project — skipped when the click landed on the title
    // link itself (that already navigates via its own href) so we don't double-navigate.
    card.addEventListener("click", (e) => {
      if (!card.dataset.href || e.target.closest("a") || e.target.closest("[data-el-tag-filter]")) return;
      window.location.href = card.dataset.href;
    });
  });

  board.querySelectorAll(".kanban-column-body").forEach((column) => {
    column.addEventListener("dragover", (e) => {
      e.preventDefault();
      e.dataTransfer.dropEffect = "move";
      column.classList.add("is-drop-target");
    });
    column.addEventListener("dragleave", () => {
      column.classList.remove("is-drop-target");
    });
    column.addEventListener("drop", (e) => {
      e.preventDefault();
      column.classList.remove("is-drop-target");
      if (!draggingCard) return;

      const newStatus = column.dataset.status;
      const itemId = draggingCard.dataset.itemId;
      const oldColumn = draggingCard.parentElement;
      if (oldColumn === column) return;

      column.appendChild(draggingCard);
      updateCount(oldColumn);
      updateCount(column);

      fetch(urlTemplate.replace("{id}", itemId), {
        method: "POST",
        headers: {
          "Content-Type": "application/x-www-form-urlencoded",
          "X-Requested-With": "fetch",
        },
        body: `${statusField}=${encodeURIComponent(newStatus)}`,
      }).then((res) => {
        if (!res.ok) {
          window.location.reload();
        }
      });
    });
  });

  function updateCount(columnBody) {
    const header = columnBody.parentElement.querySelector(".kanban-column-header .page-subtitle");
    if (header) header.textContent = columnBody.querySelectorAll(".kanban-card").length;
  }

  // Search + filter: cards live one per column (not one flat list), so this stays board-specific
  // rather than reusing static/js/entity-list-filter.js, which assumes a single flat <ul>/<li>
  // list - but follows the same data-el-filter/data-el-<key> and data-el-tag-filter/
  // data-el-tag-value conventions that script uses, for consistency across the app's list pages.
  const search = document.getElementById("kanban-search");
  const filters = Array.from(document.querySelectorAll("[data-el-filter]"));

  function matches(card) {
    if (search && search.value.trim()) {
      const q = search.value.trim().toLowerCase();
      if (!(card.dataset.search || "").toLowerCase().includes(q)) return false;
    }
    for (const sel of filters) {
      if (!sel.value) continue;
      const key = "el" + sel.dataset.elFilter.charAt(0).toUpperCase() + sel.dataset.elFilter.slice(1);
      if (card.dataset[key] !== sel.value) return false;
    }
    return true;
  }

  function render() {
    board.querySelectorAll(".kanban-card").forEach((card) => {
      card.hidden = !matches(card);
    });
  }

  if (search) search.addEventListener("input", render);
  filters.forEach((sel) => sel.addEventListener("change", render));

  board.addEventListener("click", (e) => {
    const chip = e.target.closest("[data-el-tag-filter]");
    if (!chip) return;
    e.preventDefault();
    e.stopPropagation();
    const sel = filters.find((s) => s.dataset.elFilter === chip.dataset.elTagFilter);
    if (!sel) return;
    sel.value = sel.value === chip.dataset.elTagValue ? "" : chip.dataset.elTagValue;
    render();
  });
});
