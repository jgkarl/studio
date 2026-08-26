package iiif

// BuildInfoJSON is the IIIF Image API 3.0 `info.json` descriptor —
// see https://iiif.io/api/image/3/#5-image-information.
func BuildInfoJSON(mediaID string, width, height int, origin string) map[string]any {
	return map[string]any{
		"@context":  "http://iiif.io/api/image/3/context.json",
		"id":        origin + "/api/iiif/" + mediaID,
		"type":      "ImageService3",
		"protocol":  "http://iiif.io/api/image",
		"profile":   "level2",
		"width":     width,
		"height":    height,
		"maxWidth":  width,
		"maxHeight": height,
		// Documents exactly what TransformImage (imageapi.go) actually implements — not a claim
		// of official validator-certified conformance.
		"extraFeatures": []string{
			"regionByPx",
			"regionByPct",
			"regionSquare",
			"sizeByW",
			"sizeByH",
			"sizeByWh",
			"sizeByPct",
			"sizeByConfinedWh",
			"rotationArbitrary",
			"mirroring",
		},
		"extraFormats":   []string{"png", "webp"},
		"extraQualities": []string{"color", "gray", "bitonal"},
	}
}
