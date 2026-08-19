// Pattern-layer editor, embedded in the lightbox modal (internal/media/views.templ's
// lightboxModal): an "+ Add region" checkbox arms drawing, a Tool dropdown picks rectangle vs.
// freehand brush, and a Type dropdown picks the annotation type ("the reason" for the marked
// area) — see internal/media/annotations.go's CreateRegion/CreateFreehandRegion. Regions are
// percentage coordinates of the image's own rendered box, so no resize/zoom bookkeeping is
// needed. The SVG overlay + legend are always rendered (no show/hide toggle) whenever the
// lightbox is open; this script only handles arming/drawing/persisting new regions.
document.addEventListener("DOMContentLoaded", () => {
  const wrap = document.getElementById("pattern-layer-wrap");
  const addToggle = document.getElementById("pattern-layer-add-toggle");
  const toolSelect = document.getElementById("pattern-layer-tool");
  const typeSelect = document.getElementById("pattern-layer-type");
  if (!wrap || !addToggle || !typeSelect) return;

  addToggle.addEventListener("change", () => {
    wrap.classList.toggle("is-add-mode", addToggle.checked);
  });

  function reloadOverlay() {
    // Re-fetch the media view page and swap in its freshly-rendered SVG overlay + legend, rather
    // than a full page reload — that would also re-close the lightbox modal the user is actively
    // drawing in.
    fetch(window.location.pathname, { headers: { "X-Requested-With": "fetch" } })
      .then((res) => res.text())
      .then((html) => {
        const doc = new DOMParser().parseFromString(html, "text/html");
        const freshSvg = doc.getElementById("pattern-layer-svg");
        const freshLegend = doc.querySelector(".pattern-layer-legend");
        const svg = document.getElementById("pattern-layer-svg");
        const legend = document.querySelector(".pattern-layer-legend");
        if (freshSvg && svg) svg.replaceWith(freshSvg);
        if (freshLegend && legend) legend.replaceWith(freshLegend);
      });
  }

  function postRegion(body) {
    const mediaId = wrap.dataset.mediaId;
    return fetch(`/media/${mediaId}/annotations`, {
      method: "POST",
      headers: {
        "Content-Type": "application/x-www-form-urlencoded",
        "X-Requested-With": "fetch",
      },
      body,
    }).then((res) => {
      if (res.ok) reloadOverlay();
      return res;
    });
  }

  // --- Rectangle tool ---------------------------------------------------------------------

  let dragBox = null;
  let wrapRect = null;
  let startX = 0;
  let startY = 0;

  function drawBox(x1, y1, x2, y2) {
    dragBox.style.left = `${Math.min(x1, x2)}px`;
    dragBox.style.top = `${Math.min(y1, y2)}px`;
    dragBox.style.width = `${Math.abs(x2 - x1)}px`;
    dragBox.style.height = `${Math.abs(y2 - y1)}px`;
  }

  function startRectDrag(e) {
    wrapRect = wrap.getBoundingClientRect();
    startX = e.clientX - wrapRect.left;
    startY = e.clientY - wrapRect.top;
    dragBox = document.createElement("div");
    dragBox.className = "pattern-layer-drag-box";
    wrap.appendChild(dragBox);
    drawBox(startX, startY, startX, startY);
  }

  function moveRectDrag(e) {
    drawBox(startX, startY, e.clientX - wrapRect.left, e.clientY - wrapRect.top);
  }

  function endRectDrag(e) {
    const endX = e.clientX - wrapRect.left;
    const endY = e.clientY - wrapRect.top;
    dragBox.remove();
    dragBox = null;

    const xPct = (Math.min(startX, endX) / wrapRect.width) * 100;
    const yPct = (Math.min(startY, endY) / wrapRect.height) * 100;
    const widthPct = (Math.abs(endX - startX) / wrapRect.width) * 100;
    const heightPct = (Math.abs(endY - startY) / wrapRect.height) * 100;
    if (widthPct < 1 || heightPct < 1) return;

    postRegion(
      `shape=rect&annotationTypeId=${encodeURIComponent(typeSelect.value)}&xPct=${xPct}&yPct=${yPct}&widthPct=${widthPct}&heightPct=${heightPct}`
    );
  }

  // --- Freehand brush tool ------------------------------------------------------------------

  const SVG_NS = "http://www.w3.org/2000/svg";
  let freehandPoints = null;
  let previewPolyline = null;

  function pointToPct(e) {
    const x = ((e.clientX - wrapRect.left) / wrapRect.width) * 100;
    const y = ((e.clientY - wrapRect.top) / wrapRect.height) * 100;
    return { x: Math.min(Math.max(x, 0), 100), y: Math.min(Math.max(y, 0), 100) };
  }

  function startFreehandDrag(e) {
    wrapRect = wrap.getBoundingClientRect();
    freehandPoints = [pointToPct(e)];
    const svg = document.getElementById("pattern-layer-svg");
    previewPolyline = document.createElementNS(SVG_NS, "polyline");
    previewPolyline.setAttribute("fill", "none");
    previewPolyline.setAttribute("stroke", "var(--accent, #3FA864)");
    previewPolyline.setAttribute("stroke-width", "0.4");
    previewPolyline.setAttribute("vector-effect", "non-scaling-stroke");
    svg.appendChild(previewPolyline);
    updatePreview();
  }

  function updatePreview() {
    previewPolyline.setAttribute("points", freehandPoints.map((p) => `${p.x},${p.y}`).join(" "));
  }

  function moveFreehandDrag(e) {
    const next = pointToPct(e);
    const last = freehandPoints[freehandPoints.length - 1];
    // Only keep a new point once the pointer has moved a visible amount — keeps the point count
    // (and the JSON payload) reasonable for a long brush stroke.
    if (Math.hypot(next.x - last.x, next.y - last.y) < 0.5) return;
    freehandPoints.push(next);
    updatePreview();
  }

  function endFreehandDrag() {
    previewPolyline.remove();
    previewPolyline = null;
    const points = freehandPoints;
    freehandPoints = null;
    if (points.length < 3) return;

    postRegion(
      `shape=freehand&annotationTypeId=${encodeURIComponent(typeSelect.value)}&points=${encodeURIComponent(JSON.stringify(points))}`
    );
  }

  // --- Shared drag wiring --------------------------------------------------------------------

  wrap.addEventListener("mousedown", (e) => {
    if (!addToggle.checked || !typeSelect.value) return;
    e.preventDefault();
    if (toolSelect && toolSelect.value === "freehand") {
      startFreehandDrag(e);
    } else {
      startRectDrag(e);
    }
  });

  window.addEventListener("mousemove", (e) => {
    if (dragBox) moveRectDrag(e);
    else if (freehandPoints) moveFreehandDrag(e);
  });

  window.addEventListener("mouseup", (e) => {
    if (dragBox) endRectDrag(e);
    else if (freehandPoints) endFreehandDrag();
  });
});
