// Pattern-layer overlay on the media view page: a "Show pattern layer" checkbox toggles the SVG
// overlay's visibility, and an "+ Add region" checkbox arms click-drag rectangle drawing against
// the grayscale IIIF image. Regions are percentage coordinates of the image's own rendered box, so
// no resize/zoom bookkeeping is needed — see internal/media/annotations.go's CreateRegion.
document.addEventListener("DOMContentLoaded", () => {
  const wrap = document.getElementById("pattern-layer-wrap");
  if (!wrap) return;

  const showToggle = document.getElementById("pattern-layer-show");
  const svg = document.getElementById("pattern-layer-svg");
  if (showToggle && svg) {
    showToggle.addEventListener("change", () => {
      svg.style.display = showToggle.checked ? "" : "none";
    });
  }

  const addToggle = document.getElementById("pattern-layer-add-toggle");
  const palette = document.getElementById("pattern-layer-palette");
  if (!addToggle || !palette) return;

  addToggle.addEventListener("change", () => {
    palette.hidden = !addToggle.checked;
    wrap.classList.toggle("is-add-mode", addToggle.checked);
  });

  function selectedTypeId() {
    const checked = palette.querySelector('input[name="pattern-layer-type"]:checked');
    return checked ? checked.value : null;
  }

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

  wrap.addEventListener("mousedown", (e) => {
    if (!addToggle.checked || !selectedTypeId()) return;
    wrapRect = wrap.getBoundingClientRect();
    startX = e.clientX - wrapRect.left;
    startY = e.clientY - wrapRect.top;
    dragBox = document.createElement("div");
    dragBox.className = "pattern-layer-drag-box";
    wrap.appendChild(dragBox);
    drawBox(startX, startY, startX, startY);
    e.preventDefault();
  });

  window.addEventListener("mousemove", (e) => {
    if (!dragBox) return;
    drawBox(startX, startY, e.clientX - wrapRect.left, e.clientY - wrapRect.top);
  });

  window.addEventListener("mouseup", (e) => {
    if (!dragBox) return;
    const endX = e.clientX - wrapRect.left;
    const endY = e.clientY - wrapRect.top;
    dragBox.remove();
    dragBox = null;

    const xPct = (Math.min(startX, endX) / wrapRect.width) * 100;
    const yPct = (Math.min(startY, endY) / wrapRect.height) * 100;
    const widthPct = (Math.abs(endX - startX) / wrapRect.width) * 100;
    const heightPct = (Math.abs(endY - startY) / wrapRect.height) * 100;
    if (widthPct < 1 || heightPct < 1) return;

    const typeId = selectedTypeId();
    const mediaId = wrap.dataset.mediaId;
    fetch(`/media/${mediaId}/annotations`, {
      method: "POST",
      headers: {
        "Content-Type": "application/x-www-form-urlencoded",
        "X-Requested-With": "fetch",
      },
      body: `annotationTypeId=${encodeURIComponent(typeId)}&xPct=${xPct}&yPct=${yPct}&widthPct=${widthPct}&heightPct=${heightPct}`,
    }).then((res) => {
      if (res.ok) window.location.reload();
    });
  });
});
