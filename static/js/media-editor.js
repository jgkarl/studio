// Media edit page (/media/edit/{id}, internal/media/views.templ's mediaEditBody) - the
// pattern-layer annotation editor for one annotated version.
//
// OpenSeadragon (vendored, static/openseadragon/) tiles the true original's IIIF endpoint with
// quality=gray forced (matching what BakeAnnotatedVersion composites - grayscale, see
// internal/media/rasterize.go) - fetched and injected onto the parsed info.json object rather than
// passed as a bare URL string, since OpenSeadragon's IIIFTileSource only honors a tileQuality
// override already present on the object. Its own toolbar/navigator are off - a clean canvas with
// just the image and the pattern-layer SVG overlay on top.
//
// Interaction model:
//   - Default: the stage just pans/zooms (OpenSeadragon's own mouse/touch nav). No draw-mode
//     toggle - the old "+ Add region" checkbox is gone.
//   - "+ New annotation" opens the #annotation-draft panel and arms drawing: a drag marks one
//     region (rectangle or freehand brush), shown as a dashed preview on the overlay but NOT yet
//     persisted. Pick a type, optionally write a note, then "Save annotation" POSTs it.
//   - Existing annotations are listed in #annotations-table (one <tr> each). "Edit" flips a row to
//     an inline type <select> + note <input> (the drawn shape is fixed); its save/cancel icons
//     persist or discard. "Delete" removes the region.
//   - The bottom "Save & finish" button saves the whole-image note (POST /media/{id}/description)
//     and re-bakes this version's file (POST /media/{id}/bake), then returns to the view page.
//
// After any create/edit/delete the editor re-fetches its own page and swaps in the fresh
// #pattern-layer-svg (re-added as the OSD overlay) and #annotations-table - never a full reload,
// which would tear down the live viewer.
document.addEventListener("DOMContentLoaded", () => {
  const wrap = document.getElementById("pattern-layer-wrap");
  if (!wrap) return;
  const mediaId = wrap.dataset.mediaId;
  const SVG_NS = "http://www.w3.org/2000/svg";

  let viewer = null;
  let imageReady = false;
  // Whether the "+ New annotation" draft panel is open and drawing is armed. Declared up here
  // (not with the rest of the draft state below) because ensureViewer's async "open" handler
  // reads it, and the draft wiring further down early-returns when no annotation types exist.
  let drafting = false;

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
          if (svg) viewer.addOverlay({ element: svg, location: tiledImage.getBounds() });
          // Drawing is only armed while the draft panel is open; the default is plain pan/zoom.
          viewer.setMouseNavEnabled(!drafting);
        });
      });
  }
  ensureViewer();

  function svgEl() {
    return document.getElementById("pattern-layer-svg");
  }

  // Re-fetch this edit page and swap in its freshly-rendered overlay + annotations table, rather
  // than a full page reload (which would tear down the live OpenSeadragon viewer). OSD holds a
  // live reference to the old SVG element as its overlay, so it's removed and the fresh one
  // re-added the same way the "open" handler did the first time.
  function reloadEditor() {
    return fetch(window.location.pathname, { headers: { "X-Requested-With": "fetch" } })
      .then((res) => res.text())
      .then((html) => {
        const doc = new DOMParser().parseFromString(html, "text/html");

        const freshSvg = doc.getElementById("pattern-layer-svg");
        const svg = svgEl();
        if (freshSvg && svg) {
          svg.replaceWith(freshSvg);
          if (viewer && viewer !== "loading" && imageReady) {
            viewer.removeOverlay(svg);
            viewer.addOverlay({ element: freshSvg, location: viewer.world.getItemAt(0).getBounds() });
          }
        }

        const freshTable = doc.getElementById("annotations-table");
        const table = document.getElementById("annotations-table");
        if (freshTable && table) table.replaceWith(freshTable);
      });
  }

  // --- Bottom "Save & finish": whole-image note + bake, one explicit action -------------------

  const saveBtn = document.getElementById("media-edit-save");
  const saveStatus = document.getElementById("media-edit-save-status");
  const noteInput = document.getElementById("media-edit-note");

  if (saveBtn) {
    saveBtn.addEventListener("click", async () => {
      saveBtn.disabled = true;
      if (saveStatus) saveStatus.textContent = "Saving…";
      try {
        const descRes = await fetch(`/media/${mediaId}/description`, {
          method: "POST",
          headers: { "Content-Type": "application/x-www-form-urlencoded", "X-Requested-With": "fetch" },
          body: `description=${encodeURIComponent(noteInput ? noteInput.value : "")}`,
        });
        if (!descRes.ok) {
          throw new Error(`saving the note failed (HTTP ${descRes.status}): ${(await descRes.text()).trim()}`);
        }
        const bakeRes = await fetch(`/media/${mediaId}/bake`, {
          method: "POST",
          headers: { "X-Requested-With": "fetch" },
        });
        if (!bakeRes.ok) {
          throw new Error(`baking the image failed (HTTP ${bakeRes.status}): ${(await bakeRes.text()).trim()}`);
        }
        window.location.href = `/media/view/${mediaId}`;
      } catch (err) {
        console.error("media edit save failed:", err);
        saveBtn.disabled = false;
        if (saveStatus) saveStatus.textContent = `Couldn't save — ${err.message}`;
      }
    });
  }

  // --- Annotations table: inline edit / delete (event-delegated, survives table swaps) --------

  document.addEventListener("click", (e) => {
    const btn = e.target.closest(".annotation-act");
    if (!btn) return;
    const row = btn.closest(".annotation-row");
    if (!row) return;
    const regionId = row.dataset.regionId;

    if (btn.classList.contains("annotation-act-edit")) {
      row.classList.add("is-editing");
      return;
    }
    if (btn.classList.contains("annotation-act-cancel")) {
      // Reload to restore the row's stored values verbatim (the user may have typed into the
      // inline fields before cancelling).
      reloadEditor();
      return;
    }
    if (btn.classList.contains("annotation-act-delete")) {
      if (!window.confirm("Delete this annotation?")) return;
      setRowBusy(row, true);
      fetch(`/media/${mediaId}/annotations/${regionId}/delete`, {
        method: "POST",
        headers: { "X-Requested-With": "fetch" },
      })
        .then((res) => {
          if (!res.ok) throw new Error(`HTTP ${res.status}`);
          return reloadEditor();
        })
        .catch((err) => {
          setRowBusy(row, false);
          window.alert(`Couldn't delete the annotation — ${err.message}`);
        });
      return;
    }
    if (btn.classList.contains("annotation-act-save")) {
      const typeId = row.querySelector(".annotation-edit-type").value;
      const note = row.querySelector(".annotation-edit-note").value;
      setRowBusy(row, true);
      fetch(`/media/${mediaId}/annotations/${regionId}`, {
        method: "POST",
        headers: { "Content-Type": "application/x-www-form-urlencoded", "X-Requested-With": "fetch" },
        body: `annotationTypeId=${encodeURIComponent(typeId)}&note=${encodeURIComponent(note)}`,
      })
        .then((res) => {
          if (!res.ok) throw new Error(`HTTP ${res.status}`);
          return reloadEditor();
        })
        .catch((err) => {
          setRowBusy(row, false);
          window.alert(`Couldn't save the annotation — ${err.message}`);
        });
    }
  });

  function setRowBusy(row, busy) {
    row.classList.toggle("is-busy", busy);
    row.querySelectorAll(".annotation-act").forEach((b) => (b.disabled = busy));
  }

  // --- "+ New annotation" draft: draw a region, pick a type + note, then save ------------------

  const newBtn = document.getElementById("annotation-new");
  const draft = document.getElementById("annotation-draft");
  if (!newBtn || !draft) return;

  const draftType = document.getElementById("annotation-draft-type");
  const draftNote = document.getElementById("annotation-draft-note");
  const draftSave = document.getElementById("annotation-draft-save");
  const draftCancel = document.getElementById("annotation-draft-cancel");
  const draftStatus = document.getElementById("annotation-draft-status");
  const toolButtons = draft.querySelectorAll(".pattern-layer-tool-btn");

  let currentTool = "rect";
  let pending = null; // {shape:"rect", xPct,yPct,widthPct,heightPct} | {shape:"freehand", points:[{x,y}]}
  let previewNode = null;

  function clearPreview() {
    if (previewNode) previewNode.remove();
    previewNode = null;
  }

  function setDrafting(on) {
    drafting = on;
    draft.hidden = !on;
    newBtn.hidden = on;
    wrap.classList.toggle("is-add-mode", on);
    if (viewer && viewer !== "loading") viewer.setMouseNavEnabled(!on);
    if (!on) {
      clearPreview();
      pending = null;
      if (draftNote) draftNote.value = "";
      if (draftStatus) draftStatus.textContent = "";
      setToolActive("rect");
    }
  }

  function setToolActive(tool) {
    currentTool = tool;
    toolButtons.forEach((b) => b.classList.toggle("is-active", b.dataset.tool === tool));
  }

  newBtn.addEventListener("click", () => setDrafting(true));
  draftCancel.addEventListener("click", () => setDrafting(false));
  toolButtons.forEach((b) => {
    b.addEventListener("click", () => {
      setToolActive(b.dataset.tool);
      clearPreview();
      pending = null;
      if (draftStatus) draftStatus.textContent = "";
    });
  });

  draftSave.addEventListener("click", () => {
    if (!pending) {
      draftStatus.textContent = "Draw a region on the image first.";
      return;
    }
    const typeId = draftType.value;
    if (!typeId) {
      draftStatus.textContent = "Pick an annotation type.";
      return;
    }
    let body;
    if (pending.shape === "freehand") {
      body =
        `shape=freehand&annotationTypeId=${encodeURIComponent(typeId)}` +
        `&points=${encodeURIComponent(JSON.stringify(pending.points))}` +
        `&note=${encodeURIComponent(draftNote.value)}`;
    } else {
      body =
        `shape=rect&annotationTypeId=${encodeURIComponent(typeId)}` +
        `&xPct=${pending.xPct}&yPct=${pending.yPct}&widthPct=${pending.widthPct}&heightPct=${pending.heightPct}` +
        `&note=${encodeURIComponent(draftNote.value)}`;
    }
    draftSave.disabled = true;
    draftStatus.textContent = "Saving…";
    fetch(`/media/${mediaId}/annotations`, {
      method: "POST",
      headers: { "Content-Type": "application/x-www-form-urlencoded", "X-Requested-With": "fetch" },
      body,
    })
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return reloadEditor();
      })
      .then(() => {
        draftSave.disabled = false;
        setDrafting(false);
      })
      .catch((err) => {
        draftSave.disabled = false;
        draftStatus.textContent = `Couldn't save — ${err.message}`;
      });
  });

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

  // --- Rectangle tool -------------------------------------------------------------------------
  // Live drag feedback is a plain on-screen box in pixel space; only the final corners get
  // converted to image-percent, once, when the drag ends.

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

    pending = { shape: "rect", xPct, yPct, widthPct, heightPct };
    clearPreview();
    const el = document.createElementNS(SVG_NS, "rect");
    el.setAttribute("x", xPct);
    el.setAttribute("y", yPct);
    el.setAttribute("width", widthPct);
    el.setAttribute("height", heightPct);
    el.setAttribute("class", "annotation-preview");
    svgEl().appendChild(el);
    previewNode = el;
    draftStatus.textContent = "Region marked — pick a type and save.";
  }

  // --- Freehand brush tool ------------------------------------------------------------------
  // Every point is converted to image-percent as the drag happens - the live preview polyline
  // lives in the same percent-space SVG the persisted regions do.

  let freehandPoints = null;
  let previewPolyline = null;

  function startFreehandDrag(e) {
    freehandPoints = [pixelToImagePct(e.clientX, e.clientY)];
    previewPolyline = document.createElementNS(SVG_NS, "polyline");
    previewPolyline.setAttribute("class", "annotation-preview annotation-preview-line");
    svgEl().appendChild(previewPolyline);
    updateFreehandPreview();
  }

  function updateFreehandPreview() {
    previewPolyline.setAttribute("points", freehandPoints.map((p) => `${p.x},${p.y}`).join(" "));
  }

  function moveFreehandDrag(e) {
    const next = pixelToImagePct(e.clientX, e.clientY);
    const last = freehandPoints[freehandPoints.length - 1];
    if (Math.hypot(next.x - last.x, next.y - last.y) < 0.5) return;
    freehandPoints.push(next);
    updateFreehandPreview();
  }

  function endFreehandDrag() {
    previewPolyline.remove();
    previewPolyline = null;
    const points = freehandPoints;
    freehandPoints = null;
    if (points.length < 3) return;

    pending = { shape: "freehand", points };
    clearPreview();
    const el = document.createElementNS(SVG_NS, "polygon");
    el.setAttribute("points", points.map((p) => `${p.x},${p.y}`).join(" "));
    el.setAttribute("class", "annotation-preview");
    svgEl().appendChild(el);
    previewNode = el;
    draftStatus.textContent = "Region marked — pick a type and save.";
  }

  // --- Shared drag wiring (mouse) -----------------------------------------------------------

  wrap.addEventListener("mousedown", (e) => {
    if (!drafting || !imageReady) return;
    e.preventDefault();
    if (currentTool === "freehand") startFreehandDrag(e);
    else startRectDrag(e);
  });

  window.addEventListener("mousemove", (e) => {
    if (dragBox) moveRectDrag(e);
    else if (freehandPoints) moveFreehandDrag(e);
  });

  window.addEventListener("mouseup", (e) => {
    if (dragBox) endRectDrag(e);
    else if (freehandPoints) endFreehandDrag();
  });

  // --- Touch wiring (phone/tablet) --------------------------------------------------------------
  // Mirrors the mouse wiring - every start/move/end helper only reads clientX/clientY off whatever
  // event it's given, so a touch's coordinates thread straight through via this tiny adapter.

  function touchPoint(touchEvent, list) {
    const t = touchEvent[list][0];
    return { clientX: t.clientX, clientY: t.clientY };
  }

  function abandonTouchDraw() {
    if (dragBox) {
      dragBox.remove();
      dragBox = null;
      dragStartClient = null;
    }
    if (previewPolyline) {
      previewPolyline.remove();
      previewPolyline = null;
      freehandPoints = null;
    }
  }

  wrap.addEventListener(
    "touchstart",
    (e) => {
      if (!drafting || !imageReady || e.touches.length !== 1) return;
      e.preventDefault();
      const point = touchPoint(e, "touches");
      if (currentTool === "freehand") startFreehandDrag(point);
      else startRectDrag(point);
    },
    { passive: false }
  );

  window.addEventListener(
    "touchmove",
    (e) => {
      if (!dragBox && !freehandPoints) return;
      if (e.touches.length > 1) {
        abandonTouchDraw();
        return;
      }
      e.preventDefault();
      const point = touchPoint(e, "touches");
      if (dragBox) moveRectDrag(point);
      else if (freehandPoints) moveFreehandDrag(point);
    },
    { passive: false }
  );

  window.addEventListener("touchend", (e) => {
    if (!dragBox && !freehandPoints) return;
    if (dragBox) endRectDrag(touchPoint(e, "changedTouches"));
    else if (freehandPoints) endFreehandDrag();
  });

  window.addEventListener("touchcancel", abandonTouchDraw);
});
