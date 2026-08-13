// Paint/annotate image editor — vanilla JS, no dependency. A simplified port of the original
// app's canvas-based editor: freehand brush, eraser, text labels, undo, save-as-new-media. The
// photo and the annotations are two stacked canvases (background never touched by drawing), so
// erasing a stroke reveals the untouched photo rather than punching a hole through it — same
// separation the original component used, just without its 7 brush textures/arrow tool/canvas-
// expand, which are a follow-up rather than blocking this module.
(function () {
  function openEditor(mediaId, imageURL) {
    const overlay = document.createElement("div");
    overlay.className = "image-editor-overlay";
    overlay.innerHTML =
      '<div class="image-editor-panel">' +
      '<div class="image-editor-toolbar">' +
      '<button type="button" data-tool="brush" class="btn btn-secondary is-active">Brush</button>' +
      '<button type="button" data-tool="eraser" class="btn btn-secondary">Eraser</button>' +
      '<button type="button" data-tool="text" class="btn btn-secondary">Text</button>' +
      '<button type="button" id="ie-undo" class="btn btn-ghost">Undo</button>' +
      '<span class="image-editor-spacer"></span>' +
      '<button type="button" id="ie-save" class="btn btn-primary">Save as new</button>' +
      '<button type="button" id="ie-close" class="btn btn-ghost">Close</button>' +
      "</div>" +
      '<div class="image-editor-canvas-wrap">' +
      '<canvas class="image-editor-bg"></canvas>' +
      '<canvas class="image-editor-fg"></canvas>' +
      "</div>" +
      "</div>";
    document.body.appendChild(overlay);

    const bg = overlay.querySelector(".image-editor-bg");
    const fg = overlay.querySelector(".image-editor-fg");
    const bgCtx = bg.getContext("2d");
    const fgCtx = fg.getContext("2d");

    let tool = "brush";
    let drawing = false;
    const history = [];

    function pushHistory() {
      if (fg.width === 0 || fg.height === 0) return;
      history.push(fgCtx.getImageData(0, 0, fg.width, fg.height));
      if (history.length > 25) history.shift();
    }

    function undo() {
      const snapshot = history.pop();
      if (!snapshot) return;
      fgCtx.putImageData(snapshot, 0, 0);
    }

    const img = new Image();
    img.crossOrigin = "anonymous";
    img.onload = () => {
      bg.width = fg.width = img.naturalWidth;
      bg.height = fg.height = img.naturalHeight;
      bgCtx.drawImage(img, 0, 0);
    };
    img.src = imageURL;

    function pointerPos(e) {
      const rect = fg.getBoundingClientRect();
      const scaleX = fg.width / rect.width;
      const scaleY = fg.height / rect.height;
      return { x: (e.clientX - rect.left) * scaleX, y: (e.clientY - rect.top) * scaleY };
    }

    function startDraw(e) {
      const p = pointerPos(e);
      if (tool === "text") {
        const text = window.prompt("Label text:");
        if (text) {
          pushHistory();
          fgCtx.fillStyle = "#e11d48";
          fgCtx.font = "28px sans-serif";
          fgCtx.fillText(text, p.x, p.y);
        }
        return;
      }
      drawing = true;
      pushHistory();
      fgCtx.beginPath();
      fgCtx.moveTo(p.x, p.y);
      fgCtx.lineCap = "round";
      fgCtx.lineJoin = "round";
      if (tool === "eraser") {
        fgCtx.globalCompositeOperation = "destination-out";
        fgCtx.lineWidth = 28;
      } else {
        fgCtx.globalCompositeOperation = "source-over";
        fgCtx.strokeStyle = "#e11d48";
        fgCtx.lineWidth = 5;
      }
    }

    function moveDraw(e) {
      if (!drawing) return;
      const p = pointerPos(e);
      fgCtx.lineTo(p.x, p.y);
      fgCtx.stroke();
    }

    function endDraw() {
      drawing = false;
    }

    fg.addEventListener("pointerdown", startDraw);
    fg.addEventListener("pointermove", moveDraw);
    window.addEventListener("pointerup", endDraw);

    overlay.querySelectorAll("[data-tool]").forEach((btn) => {
      btn.addEventListener("click", () => {
        tool = btn.dataset.tool;
        overlay.querySelectorAll("[data-tool]").forEach((b) => b.classList.toggle("is-active", b === btn));
      });
    });
    overlay.querySelector("#ie-undo").addEventListener("click", undo);
    overlay.querySelector("#ie-close").addEventListener("click", () => overlay.remove());
    overlay.querySelector("#ie-save").addEventListener("click", () => {
      const out = document.createElement("canvas");
      out.width = bg.width;
      out.height = bg.height;
      const outCtx = out.getContext("2d");
      outCtx.drawImage(bg, 0, 0);
      outCtx.drawImage(fg, 0, 0);
      out.toBlob((blob) => {
        const fd = new FormData();
        fd.append("file", blob, "edited.png");
        fetch(`/album/view/${mediaId}/edit`, { method: "POST", body: fd }).then((res) => {
          window.location.href = res.url;
        });
      }, "image/png");
    });
  }

  document.addEventListener("DOMContentLoaded", () => {
    const btn = document.getElementById("edit-image-btn");
    if (!btn) return;
    btn.addEventListener("click", () => {
      const mediaId = btn.dataset.sourceMediaId;
      openEditor(mediaId, `/api/media/${mediaId}?variant=original`);
    });
  });
})();
