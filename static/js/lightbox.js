// Media view lightbox — a viewer, not an editor. Rotate + brightness/contrast are plain CSS
// (transform/filter) applied to the already-rendered <img>, nothing round-trips to the server.
// The description textarea (see static/js/pattern-layer.js for its save handler) is real
// server-persisted state and is left untouched by reset().
document.addEventListener("DOMContentLoaded", () => {
  const trigger = document.getElementById("lightbox-trigger");
  const overlay = document.getElementById("lightbox-overlay");
  if (!trigger || !overlay) return;

  const image = document.getElementById("lightbox-image");
  const brightness = document.getElementById("lightbox-brightness");
  const contrast = document.getElementById("lightbox-contrast");
  const rotateBtn = document.getElementById("lightbox-rotate");
  const resetBtn = document.getElementById("lightbox-reset");
  const closeBtn = document.getElementById("lightbox-close");

  let rotation = 0;

  function applyFilters() {
    image.style.filter = `brightness(${brightness.value}%) contrast(${contrast.value}%)`;
  }

  function applyRotation() {
    image.style.transform = `rotate(${rotation}deg)`;
  }

  function reset() {
    rotation = 0;
    brightness.value = 100;
    contrast.value = 100;
    applyFilters();
    applyRotation();
  }

  function open() {
    overlay.hidden = false;
    document.body.style.overflow = "hidden";
  }

  function close() {
    overlay.hidden = true;
    document.body.style.overflow = "";
    reset();
  }

  trigger.addEventListener("click", open);
  closeBtn.addEventListener("click", close);
  overlay.addEventListener("click", (e) => {
    if (e.target === overlay) close();
  });
  document.addEventListener("keydown", (e) => {
    if (e.key === "Escape" && !overlay.hidden) close();
  });

  brightness.addEventListener("input", applyFilters);
  contrast.addEventListener("input", applyFilters);
  rotateBtn.addEventListener("click", () => {
    rotation = (rotation + 90) % 360;
    applyRotation();
  });
  resetBtn.addEventListener("click", reset);
});
