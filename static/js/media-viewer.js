// Media view page's plain deep-zoom embed (internal/media/views.templ's mediaViewBody) -
// OpenSeadragon tiling the media's own IIIF endpoint (internal/iiif) directly on the page, no
// click-to-open modal required. True color, unlike the editor (static/js/media-editor.js), which
// always forces grayscale to match what gets baked - this is just for looking at the media, not
// drawing on it. Its own toolbar/navigator are turned off, same clean-canvas convention as the
// editor.
document.addEventListener("DOMContentLoaded", () => {
  const stage = document.getElementById("media-view-stage");
  if (!stage) return;

  OpenSeadragon({
    id: "media-view-stage",
    prefixUrl: "/static/openseadragon/images/",
    tileSources: stage.dataset.infoUrl,
    showNavigator: false,
    showNavigationControl: false,
  });
});
