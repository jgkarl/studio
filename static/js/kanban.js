// Orders kanban — native HTML5 Drag and Drop API, no dependency. dnd-kit (the original app's
// drag library) is React-only with no framework-agnostic build, so this is a small vanilla
// island instead rather than vendoring React+ReactDOM just for one board.
document.addEventListener("DOMContentLoaded", () => {
  const board = document.getElementById("kanban-board");
  if (!board) return;

  let draggingCard = null;

  board.querySelectorAll(".kanban-card").forEach((card) => {
    card.addEventListener("dragstart", (e) => {
      draggingCard = card;
      e.dataTransfer.effectAllowed = "move";
      e.dataTransfer.setData("text/plain", card.dataset.orderId);
      card.classList.add("is-dragging");
    });
    card.addEventListener("dragend", () => {
      card.classList.remove("is-dragging");
      draggingCard = null;
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
      const orderId = draggingCard.dataset.orderId;
      const oldColumn = draggingCard.parentElement;
      if (oldColumn === column) return;

      column.appendChild(draggingCard);
      updateCount(oldColumn);
      updateCount(column);

      fetch(`/clients/orders/${orderId}/status`, {
        method: "POST",
        headers: {
          "Content-Type": "application/x-www-form-urlencoded",
          "X-Requested-With": "fetch",
        },
        body: `status=${encodeURIComponent(newStatus)}`,
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
});
