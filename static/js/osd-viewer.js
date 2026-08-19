// Read-only deep-zoom viewer: OpenSeadragon (static/openseadragon/, vendored — see that
// directory's LICENSE.txt, New BSD — the one deliberate exception to this app's usual
// "no vendored libraries" convention, since a real IIIF deep-zoom experience needs OpenSeadragon's
// tile scheduling/caching rather than something worth hand-rolling) pointed straight at the
// existing IIIF Image API info.json (internal/iiif) — OpenSeadragon's built-in IIIF tile source
// support generates region/size requests against that API on its own, no custom tile-source code
// needed here.
//
// The stored annotation regions (same rows the pattern-layer editor draws — internal/media/
// annotations.go) are already rendered server-side into one 0-100 viewBox <svg id=
// "osd-annotation-overlay"> sibling of the OSD mount div (see internal/media/views.templ's
// iiifViewerModal). Rather than hand-computing a CSS transform on every pan/zoom tick, this hands
// that whole SVG to OpenSeadragon's own overlay system (viewer.addOverlay) sized to the full image
// bounds — OpenSeadragon repositions/rescales it automatically as the viewport changes.
document.addEventListener("DOMContentLoaded", () => {
  const trigger = document.getElementById("iiif-viewer-trigger");
  const overlay = document.getElementById("iiif-viewer-overlay");
  const closeBtn = document.getElementById("iiif-viewer-close");
  if (!trigger || !overlay) return;

  const stage = document.getElementById("iiif-viewer-stage");
  const svg = document.getElementById("osd-annotation-overlay");
  let viewer = null;

  function openViewer() {
    overlay.hidden = false;
    if (viewer) return;

    viewer = OpenSeadragon({
      id: "osd-viewer",
      prefixUrl: "/static/openseadragon/images/",
      tileSources: stage.dataset.infoUrl,
      showNavigator: true,
      navigatorPosition: "BOTTOM_RIGHT",
      gestureSettingsMouse: { clickToZoom: false },
    });

    viewer.addHandler("open", () => {
      if (!svg) return;
      const tiledImage = viewer.world.getItemAt(0);
      viewer.addOverlay({ element: svg, location: tiledImage.getBounds() });
    });
  }

  function closeViewer() {
    overlay.hidden = true;
  }

  trigger.addEventListener("click", openViewer);
  closeBtn.addEventListener("click", closeViewer);
});
