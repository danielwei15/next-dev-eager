package main

import (
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	baseURL = "http://localhost:3000"
)

// main is the entry point of the application.
// It finds Next.js routes, sorts them, and makes HTTP requests to "warm them up".
func main() {
	log.SetFlags(0) // Keep log output clean.

	// 1. Find the Next.js 'app' directory.
	appPath := findAppDirectory()

	// 2. Discover all static, non-dynamic routes.
	fmt.Println("Discovering routes...")
	routes, err := findStaticRoutes(appPath)
	if err != nil {
		log.Fatalf("Error discovering routes: %v", err)
	}

	if len(routes) == 0 {
		fmt.Println("No static routes found to warm up.")
		return
	}

	// 3. Sort routes by path length, ascending, to warm up base paths first.
	sort.Slice(routes, func(i, j int) bool {
		return len(routes[i]) < len(routes[j])
	})

	fmt.Printf("Found %d static routes. Warming them up...\n\n", len(routes))

	// 4. Sequentially warm up each route.
	client := &http.Client{
		Timeout: 15 * time.Second, // Generous timeout for potentially slow server-side rendering on first load.
	}

	for _, route := range routes {
		url := baseURL + route
		fmt.Printf("GET %s ... ", url)

		start := time.Now()
		resp, err := client.Get(url)
		if err != nil {
			fmt.Printf("Error: %v\n", err)
			// Don't stop for a single failed request; continue to the next.
			time.Sleep(1 * time.Second)
			continue
		}

		// It's good practice to close the body, even if we don't read it.
		resp.Body.Close()

		fmt.Printf("[%s] in %v\n", resp.Status, time.Since(start).Round(time.Millisecond))

		// 5. Wait for 1 second before the next request as requested.
		time.Sleep(1 * time.Second)
	}

	fmt.Println("\nWarm-up complete.")
}

// findAppDirectory searches for the Next.js 'app' directory in common locations.
// It checks for 'app/' and 'src/app/' and returns the path if found.
// If neither is found, it terminates the program with a fatal error.
func findAppDirectory() string {
	possibleAppDirs := []string{"app", "src/app"}
	for _, dir := range possibleAppDirs {
		if _, err := os.Stat(dir); err == nil {
			log.Printf("Found app directory at: ./%s", dir)
			return dir
		}
	}

	log.Fatalf("Error: Could not find 'app' or 'src/app' directory. This tool must be run from the root of a Next.js app router project.")
	return "" // Unreachable, but satisfies compiler.
}

// findStaticRoutes recursively scans the 'app' directory to find all static Next.js routes.
// It identifies routes by looking for 'page.tsx', 'page.js', etc., files.
// It correctly interprets Next.js App Router conventions, filtering out dynamic routes,
// route groups, parallel routes, and private folders, as these cannot be "woken up"
// without specific parameters or are not part of the standard URL structure.
func findStaticRoutes(root string) ([]string, error) {
	routeSet := make(map[string]struct{})

	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			// Propagate errors from walking the directory.
			return err
		}

		// A route is defined by the presence of a 'page.*' file.
		fileName := d.Name()
		isPageFile := !d.IsDir() && strings.HasPrefix(fileName, "page.") &&
			(strings.HasSuffix(fileName, ".js") || strings.HasSuffix(fileName, ".jsx") ||
				strings.HasSuffix(fileName, ".ts") || strings.HasSuffix(fileName, ".tsx"))

		if !isPageFile {
			return nil
		}

		// The directory containing the page file defines the route's path.
		routePath := filepath.Dir(path)

		// Get path relative to the 'app' directory root.
		relPath, err := filepath.Rel(root, routePath)
		if err != nil {
			// This is unexpected if the path is from WalkDir starting at root.
			return fmt.Errorf("failed to get relative path for %s: %w", path, err)
		}

		// Normalize to forward slashes for URLs, regardless of OS.
		route := filepath.ToSlash(relPath)

		// The root of the walk is '.', which corresponds to the root route '/'.
		if route == "." {
			route = "/"
		} else {
			route = "/" + route
		}

		// Process path segments to handle Next.js conventions.
		segments := strings.Split(route, "/")
		var finalSegments []string
		for _, segment := range segments {
			if segment == "" {
				continue
			}
			// Private folders (e.g., `_components`) are not part of the route path.
			// Any path containing such a segment is not a public route.
			if strings.HasPrefix(segment, "_") {
				log.Printf("Info: Skipping path with private segment: %s", route)
				return nil // Skip this entire path.
			}
			// Route groups (e.g., `(marketing)`) are for organization and don't affect the URL.
			if strings.HasPrefix(segment, "(") && strings.HasSuffix(segment, ")") && !strings.HasPrefix(segment, "(...") {
				continue
			}
			// Intercepting routes are a special case and do not define a canonical URL to be warmed up.
			if segment == "(.)" || segment == "(..)" || segment == "(...)" {
				log.Printf("Info: Skipping intercepting route: %s", route)
				return nil
			}
			// Parallel routes (e.g., `@team`) are rendered in the same URL and are not separate routes.
			if strings.HasPrefix(segment, "@") {
				log.Printf("Info: Skipping parallel route slot: %s", route)
				return nil
			}
			// Dynamic routes (e.g., `[id]` or `[...slug]`) cannot be warmed up without specific params.
			if strings.HasPrefix(segment, "[") && strings.HasSuffix(segment, "]") {
				log.Printf("Info: Skipping dynamic route: %s", route)
				return nil
			}
			finalSegments = append(finalSegments, segment)
		}

		// Reconstruct the clean, final route.
		finalRoute := "/" + strings.Join(finalSegments, "/")
		// Handle cases where all segments were stripped (e.g., root page in a group).
		if finalRoute == "//" {
			finalRoute = "/"
		}

		routeSet[finalRoute] = struct{}{}
		return nil
	})

	if err != nil {
		return nil, err
	}

	// Convert the set of unique routes to a slice for sorting and iteration.
	routes := make([]string, 0, len(routeSet))
	for r := range routeSet {
		routes = append(routes, r)
	}

	return routes, nil
}
