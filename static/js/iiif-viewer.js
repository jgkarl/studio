// Deep-zoom viewer backed by the real IIIF Image API (internal/iiif) — hand-rolled rather than
// vendoring OpenSeadragon (studio-go's README states "no vendored libraries" as an explicit
// value). Wheel zooms, drag pans; both recompute a pct: region centered on the current viewport
// and swap the <img> src, debounced so a zoom/pan gesture doesn't fire a request per event tick.
// Read-only — no drawing here, that's static/js/pattern-layer.js's job.
document.addEventListener("DOMContentLoaded", () => {
  const trigger = document.getElementById("iiif-viewer-trigger");
  const overlay = document.getElementById("iiif-viewer-overlay");
  if (!trigger || !overlay) return;

  const stage = document.getElementById("iiif-viewer-stage");
  const image = document.getElementById("iiif-viewer-image");
  const closeBtn = document.getElementById("iiif-viewer-close");
  const zoomInBtn = document.getElementById("iiif-viewer-zoom-in");
  const zoomOutBtn = document.getElementById("iiif-viewer-zoom-out");
  const resetBtn = document.getElementById("iiif-viewer-reset");
  const mediaId = stage.dataset.mediaId;

  const MIN_ZOOM = 1;
  const MAX_ZOOM = 8;
  let zoom = MIN_ZOOM;
  let centerX = 50;
  let centerY = 50;
  let pending = null;

  function currentRegion() {
    const w = 100 / zoom;
    const h = 100 / zoom;
    const x = Math.min(Math.max(centerX - w / 2, 0), 100 - w);
    const y = Math.min(Math.max(centerY - h / 2, 0), 100 - h);
    return { x, y, w, h };
  }

  function render() {
    clearTimeout(pending);
    pending = setTimeout(() => {
      if (zoom <= MIN_ZOOM) {
        image.src = `/api/iiif/${mediaId}/full/max/0/default.jpg`;
        return;
      }
      const { x, y, w, h } = currentRegion();
      image.src = `/api/iiif/${mediaId}/pct:${x.toFixed(2)},${y.toFixed(2)},${w.toFixed(2)},${h.toFixed(2)}/max/0/default.jpg`;
    }, 150);
  }

  function setZoom(next) {
    next = Math.min(Math.max(next, MIN_ZOOM), MAX_ZOOM);
    if (next === zoom) return;
    zoom = next;
    stage.classList.toggle("is-zoomed", zoom > MIN_ZOOM);
    render();
  }

  function resetView() {
    zoom = MIN_ZOOM;
    centerX = 50;
    centerY = 50;
    stage.classList.remove("is-zoomed");
  }

  function open() {
    overlay.hidden = false;
    document.body.style.overflow = "hidden";
  }

  function close() {
    overlay.hidden = true;
    document.body.style.overflow = "";
    resetView();
    image.src = `/api/iiif/${mediaId}/full/max/0/default.jpg`;
  }

  trigger.addEventListener("click", open);
  closeBtn.addEventListener("click", close);
  overlay.addEventListener("click", (e) => {
    if (e.target === overlay) close();
  });
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape" && !overlay.hidden) close();
  });

  zoomInBtn.addEventListener("click", () => setZoom(zoom * 1.6));
  zoomOutBtn.addEventListener("click", () => setZoom(zoom / 1.6));
  resetBtn.addEventListener("click", () => {
    resetView();
    render();
  });

  stage.addEventListener(
    "wheel",
    (e) => {
      e.preventDefault();
      setZoom(zoom * (e.deltaY < 0 ? 1.2 : 1 / 1.2));
    },
    { passive: false }
  );

  let dragging = false;
  let lastX = 0;
  let lastY = 0;

  stage.addEventListener("mousedown", (e) => {
    if (zoom <= MIN_ZOOM) return;
    dragging = true;
    lastX = e.clientX;
    lastY = e.clientY;
  });

  window.addEventListener("mousemove", (e) => {
    if (!dragging) return;
    const rect = stage.getBoundingClientRect();
    const { w, h } = currentRegion();
    const dxPct = ((e.clientX - lastX) / rect.width) * w;
    const dyPct = ((e.clientY - lastY) / rect.height) * h;
    centerX = Math.min(Math.max(centerX - dxPct, 0), 100);
    centerY = Math.min(Math.max(centerY - dyPct, 0), 100);
    lastX = e.clientX;
    lastY = e.clientY;
    render();
  });

  window.addEventListener("mouseup", () => {
    dragging = false;
  });
});
