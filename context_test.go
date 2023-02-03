package gor

import "testing"

// TestRoutePattern tests correct in-the-middle wildcard removals.
func TestRoutePattern(t *testing.T) {
	routePatterns := []string{
		"/v1/*",
		"/resources/*",
		"/{resource_id}",
	}

	x := &Context{
		RoutePatterns: routePatterns,
	}

	if p := x.RoutePattern(); p != "/v1/resources/{resource_id}" {
		t.Fatal("unexpected route pattern: " + p)
	}

	x.RoutePatterns = []string{
		"/v1/*",
		"/resources/*",
		// Additional wildcard, depending on the router structure of the user
		"/*",
		"/{resource_id}",
	}

	// Correctly removes in-the-middle wildcards instead of "/v1/resources/*/{resource_id}"
	if p := x.RoutePattern(); p != "/v1/resources/{resource_id}" {
		t.Fatal("unexpected route pattern: " + p)
	}

	x.RoutePatterns = []string{
		"/v1/*",
		"/resources/*",
		// Even with many wildcards
		"/*",
		"/*",
		"/*",
		"/{resource_id}/*", // Keeping trailing wildcard
	}

	// Correctly removes in-the-middle wildcards instead of "/v1/resources/*/*/{resource_id}/*"
	if p := x.RoutePattern(); p != "/v1/resources/{resource_id}/*" {
		t.Fatal("unexpected route pattern: " + p)
	}

	x.RoutePatterns = []string{
		"/v1/*",
		"/resources/*",
		// And respects asterisks as part of the paths
		"/*special_path/*",
		"/with_asterisks*/*",
		"/{resource_id}",
	}

	// Correctly removes in-the-middle wildcards instead of "/v1/resourcesspecial_path/with_asterisks{resource_id}"
	if p := x.RoutePattern(); p != "/v1/resources/*special_path/with_asterisks*/{resource_id}" {
		t.Fatal("unexpected route pattern: " + p)
	}
}
