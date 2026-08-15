// Client-side filter for the assets list search box - the table is small enough (one studio's
// worth of objects) that a round-trip per keystroke isn't worth it. Same pattern as album.js.
document.addEventListener("DOMContentLoaded", () => {
  const search = document.getElementById("asset-search");
  const list = document.getElementById("asset-list");
  if (!search || !list) return;

  const rows = Array.from(list.querySelectorAll("[data-search]"));

  search.addEventListener("input", () => {
    const q = search.value.trim().toLowerCase();
    rows.forEach((row) => {
      const show = !q || row.dataset.search.toLowerCase().includes(q);
      row.parentElement.style.display = show ? "" : "none";
    });
  });
});
