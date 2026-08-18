// Generic list search + filter + group-by island — one script shared by every list page (Assets/
// Clients/Treatments/Reports/Media). Client-side only, same "table's small enough that a round
// trip per keystroke isn't worth it" reasoning static/js/assets-search.js's own comment already
// used — this just generalizes that one page's script into a reusable one.
//
// Markup contract:
//   <div data-el-root>
//     <input data-el-search placeholder="…">                          (optional)
//     <select data-el-filter="assetType">…</select>                   (optional, any number)
//     <select data-el-groupby><option value="">No grouping</option>…</select>  (optional)
//     <ul data-el-list>
//       <li data-el-row
//           data-el-search="haystack text"
//           data-el-asset-type="painting"
//           data-el-group-client="Jane Doe">
//         …
//       </li>
//     </ul>
//   </div>
//
// A filter <select data-el-filter="assetType"> matches rows by their data-el-asset-type
// attribute (dataset camelCases "assetType" -> looks up dataset.elAssetType). Same convention
// for data-el-groupby against data-el-group-<value>.
document.addEventListener("DOMContentLoaded", () => {
  document.querySelectorAll("[data-el-root]").forEach(initEntityList);
});

function elCapitalize(s) {
  return s.charAt(0).toUpperCase() + s.slice(1);
}

function initEntityList(root) {
  const list = root.querySelector("[data-el-list]");
  if (!list) return;
  const rows = Array.from(list.querySelectorAll(":scope > [data-el-row]"));
  const search = root.querySelector("[data-el-search]");
  const filters = Array.from(root.querySelectorAll("[data-el-filter]"));
  const groupBy = root.querySelector("[data-el-groupby]");

  function matches(row) {
    if (search && search.value.trim()) {
      const q = search.value.trim().toLowerCase();
      const hay = (row.dataset.elSearch || "").toLowerCase();
      if (!hay.includes(q)) return false;
    }
    for (const sel of filters) {
      if (!sel.value) continue;
      const datasetKey = "el" + elCapitalize(sel.dataset.elFilter);
      if (row.dataset[datasetKey] !== sel.value) return false;
    }
    return true;
  }

  function render() {
    list.querySelectorAll("[data-el-group-header]").forEach((h) => h.remove());
    const visible = rows.filter(matches);
    const groupKey = groupBy && groupBy.value ? groupBy.value : null;

    rows.forEach((row) => {
      row.hidden = !visible.includes(row);
    });

    if (!groupKey) {
      rows.forEach((row) => list.appendChild(row));
      return;
    }

    const groupDatasetKey = "elGroup" + elCapitalize(groupKey);
    const groups = new Map();
    visible.forEach((row) => {
      const label = row.dataset[groupDatasetKey] || "—";
      if (!groups.has(label)) groups.set(label, []);
      groups.get(label).push(row);
    });
    Array.from(groups.keys())
      .sort()
      .forEach((label) => {
        const header = document.createElement("li");
        header.setAttribute("data-el-group-header", "");
        header.className = "entity-list-group-header";
        header.textContent = label;
        list.appendChild(header);
        groups.get(label).forEach((row) => list.appendChild(row));
      });
  }

  if (search) search.addEventListener("input", render);
  filters.forEach((sel) => sel.addEventListener("change", render));
  if (groupBy) groupBy.addEventListener("change", render);
  render();
}
