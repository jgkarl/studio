// Album filter/search/group — vanilla JS over cards already rendered server-side (no extra
// round-trip for what's purely a client-side view change), same behavior as the original app's
// React AlbumGallery client component.
document.addEventListener("DOMContentLoaded", () => {
  const grid = document.getElementById("album-grid");
  if (!grid) return;

  const search = document.getElementById("album-search");
  const kindBtns = Array.from(document.querySelectorAll(".album-kind-btn"));
  const groupToggle = document.getElementById("album-group-toggle");
  const countEl = document.getElementById("album-count");
  const cards = Array.from(grid.querySelectorAll(".album-card"));
  let kind = "all";

  function applyFilters() {
    const q = search.value.trim().toLowerCase();
    let visible = 0;
    cards.forEach((card) => {
      const matchesKind = kind === "all" || card.dataset.kind === kind;
      const matchesSearch = !q || card.dataset.search.toLowerCase().includes(q);
      const show = matchesKind && matchesSearch;
      card.style.display = show ? "" : "none";
      if (show) visible++;
    });
    countEl.textContent = visible + (visible === 1 ? " item" : " items");
    applyGrouping();
  }

  function applyGrouping() {
    grid.querySelectorAll(".album-group-header").forEach((h) => h.remove());
    if (!groupToggle.checked) {
      grid.classList.remove("album-grid-grouped");
      cards.forEach((c) => grid.appendChild(c));
      return;
    }
    grid.classList.add("album-grid-grouped");
    const groups = new Map();
    cards
      .filter((c) => c.style.display !== "none")
      .forEach((c) => {
        const key = c.dataset.projectId || "__none__";
        const title = c.dataset.projectTitle || "No workflow";
        if (!groups.has(key)) groups.set(key, { title, cards: [] });
        groups.get(key).cards.push(c);
      });
    groups.forEach((group) => {
      const header = document.createElement("div");
      header.className = "album-group-header";
      header.textContent = `${group.title} (${group.cards.length})`;
      grid.appendChild(header);
      group.cards.forEach((c) => grid.appendChild(c));
    });
  }

  search.addEventListener("input", applyFilters);
  groupToggle.addEventListener("change", applyFilters);
  kindBtns.forEach((btn) => {
    btn.addEventListener("click", () => {
      kind = btn.dataset.kind;
      kindBtns.forEach((b) => b.classList.toggle("is-active", b === btn));
      applyFilters();
    });
  });

  applyFilters();
});
