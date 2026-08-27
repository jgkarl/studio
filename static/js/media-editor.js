// Media edit page (/media/edit/{id}, internal/media/views.templ's mediaEditBody) - the
// pattern-layer annotation editor for one annotated version. Used to live in a lightbox modal
// opened from the view page; now its own plain page/URL, so the browser's own back button/history
// work like any other page and there's a real "Save" action instead of an implicit
// bake-on-modal-close.
//
// OpenSeadragon (vendored, static/openseadragon/) tiles the true original's IIIF endpoint with
// quality=gray forced (matching what BakeAnnotatedVersion actually composites - grayscale, see
// internal/media/rasterize.go) - fetched and injected onto the parsed info.json object rather than
// passed as a bare URL string, since OpenSeadragon's IIIFTileSource only honors a tileQuality
// override already present on the object, not derivable from a URL. Its own toolbar/navigator are
// off - a clean canvas with just the image and the pattern-layer SVG overlay on top.
//
// The stored annotation regions (internal/media/annotations.go) are server-rendered into one
// 0-100 viewBox <svg id="pattern-layer-svg"> sibling of the OSD mount div, handed to OpenSeadragon
// as one of its own overlays (viewer.addOverlay) so it tracks pan/zoom automatically instead of
// needing hand-rolled viewport-transform math. New regions are drawn directly on this surface:
// every coordinate a drag produces is converted from on-screen pixels to image-percentage via
// viewport.viewerElementToImageCoordinates before it's POSTed, so a region is stored (and
// replayed) in the same percent-of-image space regardless of what zoom/pan it was drawn at.
// Drawing is armed by default (the "+ Add region" checkbox starts checked) - uncheck it to pan/
// zoom instead.
//
// Regions persist immediately as they're drawn/deleted (unchanged from before). The "Save" button
// is what saves the whole-image note (POST /media/{id}/description) and re-bakes this version's
// own file (POST /media/{id}/bake) in one explicit action, then returns to the view page -
// replacing the old implicit bake-on-modal-close plus a separate tiny note-save button with one
// clear action a conservator can point at.
document.addEventListener("DOMContentLoaded", () => {
  const wrap = document.getElementById("pattern-layer-wrap");
  if (!wrap) return;

  let viewer = null;
  let imageReady = false;

  const addToggle = document.getElementById("pattern-layer-add-toggle");

  function ensureViewer() {
    if (viewer) return;
    viewer = "loading";

    fetch(wrap.dataset.infoUrl)
      .then((res) => res.json())
      .then((info) => {
        info.tileQuality = "gray";
        viewer = OpenSeadragon({
          id: "osd-viewer",
          prefixUrl: "/static/openseadragon/images/",
          tileSources: info,
          showNavigator: false,
          showNavigationControl: false,
          gestureSettingsMouse: { clickToZoom: false },
        });

        viewer.addHandler("open", () => {
          imageReady = true;
          const svg = document.getElementById("pattern-layer-svg");
          const tiledImage = viewer.world.getItemAt(0);
          if (svg) {
            viewer.addOverlay({ element: svg, location: tiledImage.getBounds() });
          }
          viewer.setMouseNavEnabled(addToggle ? !addToggle.checked : true);
        });
      });
  }
  ensureViewer();

  // --- Save: note + bake, one explicit action -------------------------------------------------

  const saveBtn = document.getElementById("media-edit-save");
  const saveStatus = document.getElementById("media-edit-save-status");
  const noteInput = document.getElementById("media-edit-note");

  if (saveBtn) {
    saveBtn.addEventListener("click", () => {
      const mediaId = wrap.dataset.mediaId;
      saveBtn.disabled = true;
      if (saveStatus) saveStatus.textContent = "Saving…";

      fetch(`/media/${mediaId}/description`, {
        method: "POST",
        headers: {
          "Content-Type": "application/x-www-form-urlencoded",
          "X-Requested-With": "fetch",
        },
        body: `description=${encodeURIComponent(noteInput ? noteInput.value : "")}`,
      })
        .then(() => fetch(`/media/${mediaId}/bake`, { method: "POST", headers: { "X-Requested-With": "fetch" } }))
        .then((res) => {
          if (!res.ok) throw new Error("bake failed");
          window.location.href = `/media/view/${mediaId}`;
        })
        .catch(() => {
          saveBtn.disabled = false;
          if (saveStatus) saveStatus.textContent = "Couldn't save — try again.";
        });
    });
  }

  // --- Pattern-layer draw tool ------------------------------------------------------------------
  // The "+ Add region" checkbox (armed by default) freezes OpenSeadragon's own pan/zoom while
  // checked, so a draw-drag doesn't also move the viewport - uncheck it to pan/zoom instead. Two
  // icon-toggle buttons pick rectangle vs. freehand brush; a Type <select> picks the annotation
  // type ("the reason" for the marked area) — see internal/media/annotations.go's
  // CreateRegion/CreateFreehandRegion.

  const toolButtons = document.querySelectorAll(".pattern-layer-tool-btn");
  const typeSelect = document.getElementById("pattern-layer-type");
  if (!addToggle || !typeSelect) return;

  let currentTool = "rect";
  toolButtons.forEach((btn) => {
    btn.addEventListener("click", () => {
      currentTool = btn.dataset.tool;
      toolButtons.forEach((b) => b.classList.toggle("is-active", b === btn));
    });
  });

  wrap.classList.toggle("is-add-mode", addToggle.checked);
  addToggle.addEventListener("change", () => {
    wrap.classList.toggle("is-add-mode", addToggle.checked);
    if (viewer && viewer !== "loading") viewer.setMouseNavEnabled(!addToggle.checked);
  });

  function reloadOverlay() {
    // Re-fetch this same edit page and swap in its freshly-rendered SVG overlay + legend, rather
    // than a full page reload — that would also tear down the live OpenSeadragon viewer the user
    // is actively drawing in. OpenSeadragon already has a live reference to the old SVG element as
    // its overlay; replacing that element in the DOM doesn't move the overlay, so it's re-added
    // the same way the "open" handler did the first time.
    fetch(window.location.pathname, { headers: { "X-Requested-With": "fetch" } })
      .then((res) => res.text())
      .then((html) => {
        const doc = new DOMParser().parseFromString(html, "text/html");
        const freshSvg = doc.getElementById("pattern-layer-svg");
        const freshLegend = doc.querySelector(".pattern-layer-legend");
        const svg = document.getElementById("pattern-layer-svg");
        const legend = document.querySelector(".pattern-layer-legend");
        if (freshSvg && svg) {
          svg.replaceWith(freshSvg);
          if (viewer && viewer !== "loading" && imageReady) {
            viewer.removeOverlay(svg);
            viewer.addOverlay({ element: freshSvg, location: viewer.world.getItemAt(0).getBounds() });
          }
        }
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

  // A drag's on-screen pixel position, resolved to a percentage of the full (untiled) image —
  // stable across whatever zoom/pan the viewport happens to be at.
  function pixelToImagePct(clientX, clientY) {
    const rect = wrap.getBoundingClientRect();
    const point = new OpenSeadragon.Point(clientX - rect.left, clientY - rect.top);
    const imagePoint = viewer.viewport.viewerElementToImageCoordinates(point);
    const size = viewer.world.getItemAt(0).source.dimensions;
    return {
      x: Math.min(Math.max((imagePoint.x / size.x) * 100, 0), 100),
      y: Math.min(Math.max((imagePoint.y / size.y) * 100, 0), 100),
    };
  }

  // --- Rectangle tool ---------------------------------------------------------------------
  // Live drag feedback is a plain on-screen box in pixel space (cheap, and the drag hasn't
  // produced a real region yet) - only the final corners get converted to image-percent, once,
  // when the drag ends.

  let dragBox = null;
  let dragStartClient = null;

  function drawBox(x1, y1, x2, y2) {
    dragBox.style.left = `${Math.min(x1, x2)}px`;
    dragBox.style.top = `${Math.min(y1, y2)}px`;
    dragBox.style.width = `${Math.abs(x2 - x1)}px`;
    dragBox.style.height = `${Math.abs(y2 - y1)}px`;
  }

  function startRectDrag(e) {
    dragStartClient = { x: e.clientX, y: e.clientY };
    const rect = wrap.getBoundingClientRect();
    dragBox = document.createElement("div");
    dragBox.className = "pattern-layer-drag-box";
    wrap.appendChild(dragBox);
    drawBox(e.clientX - rect.left, e.clientY - rect.top, e.clientX - rect.left, e.clientY - rect.top);
  }

  function moveRectDrag(e) {
    const rect = wrap.getBoundingClientRect();
    drawBox(dragStartClient.x - rect.left, dragStartClient.y - rect.top, e.clientX - rect.left, e.clientY - rect.top);
  }

  function endRectDrag(e) {
    dragBox.remove();
    dragBox = null;

    const p1 = pixelToImagePct(dragStartClient.x, dragStartClient.y);
    const p2 = pixelToImagePct(e.clientX, e.clientY);
    dragStartClient = null;
    const xPct = Math.min(p1.x, p2.x);
    const yPct = Math.min(p1.y, p2.y);
    const widthPct = Math.abs(p2.x - p1.x);
    const heightPct = Math.abs(p2.y - p1.y);
    if (widthPct < 0.5 || heightPct < 0.5) return;

    postRegion(
      `shape=rect&annotationTypeId=${encodeURIComponent(typeSelect.value)}&xPct=${xPct}&yPct=${yPct}&widthPct=${widthPct}&heightPct=${heightPct}`
    );
  }

  // --- Freehand brush tool ------------------------------------------------------------------
  // Every point is converted to image-percent as the drag happens (not just the endpoints) -
  // the live preview polyline lives in the same percent-space SVG the persisted regions do, so
  // it must already be in that space to render in the right place at the current zoom/pan.

  const SVG_NS = "http://www.w3.org/2000/svg";
  let freehandPoints = null;
  let previewPolyline = null;

  function startFreehandDrag(e) {
    freehandPoints = [pixelToImagePct(e.clientX, e.clientY)];
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
    const next = pixelToImagePct(e.clientX, e.clientY);
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
    if (!addToggle.checked || !typeSelect.value || !imageReady) return;
    e.preventDefault();
    if (currentTool === "freehand") {
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
