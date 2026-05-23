package transport

import (
	"fmt"
	"strings"
)

// pathToCommand парсит RCI-путь в JSON-дерево для batch-POST'а.
//
// Поддерживает формы:
//
//	"/show/interface/"                            → {"show": {"interface": nil}}
//	"/show/interface/Wireguard0"                  → {"show": {"interface": {"name": "Wireguard0"}}}
//	"/show/interface/system-name?name=Wireguard0" → {"show": {"interface": {"system-name": {"name": "Wireguard0"}}}}
//	"/show/sc/dns-proxy/route"                    → {"show": {"sc": {"dns-proxy": {"route": nil}}}}
//
// Эвристика: последний "содержательный" сегмент это leaf. Если последний
// сегмент — name-like (без `/` после, без `?`) после path/segments — он
// становится {name: <value>} параметром предыдущего узла. Если последний
// сегмент содержит `?k=v` — параметры применяются к leaf'у. Trailing slash
// означает leaf без параметров (nil value).
func pathToCommand(path string) (any, error) {
	path = strings.TrimPrefix(path, "/")
	if path == "" {
		return nil, fmt.Errorf("pathToCommand: empty path")
	}

	hasTrailingSlash := strings.HasSuffix(path, "/")
	path = strings.TrimSuffix(path, "/")

	segments := strings.Split(path, "/")
	if len(segments) == 0 {
		return nil, fmt.Errorf("pathToCommand: no segments")
	}

	last := segments[len(segments)-1]
	var leafParams map[string]any

	if idx := strings.Index(last, "?"); idx >= 0 {
		paramStr := last[idx+1:]
		last = last[:idx]
		if last == "" || paramStr == "" {
			return nil, fmt.Errorf("pathToCommand: malformed query in %q", path)
		}
		segments[len(segments)-1] = last

		leafParams = map[string]any{}
		for _, kv := range strings.Split(paramStr, "&") {
			eq := strings.Index(kv, "=")
			if eq <= 0 || eq == len(kv)-1 {
				return nil, fmt.Errorf("pathToCommand: malformed param %q in %q", kv, path)
			}
			leafParams[kv[:eq]] = kv[eq+1:]
		}
	}

	var leafKey string
	var leafValue any

	if leafParams != nil {
		// "/show/interface/system-name?name=Wireguard0"
		// → segments [show, interface, system-name], leafParams {name: Wireguard0}
		leafKey = last
		leafValue = leafParams
		segments = segments[:len(segments)-1]
	} else if hasTrailingSlash {
		// "/show/interface/" → leaf is "interface", value nil
		leafKey = last
		leafValue = nil
		segments = segments[:len(segments)-1]
	} else if len(segments) == 3 {
		// "/show/interface/Wireguard0" → last "Wireguard0" is name-value of "interface"
		leafKey = segments[len(segments)-2]
		leafValue = map[string]any{"name": last}
		segments = segments[:len(segments)-2]
	} else {
		// len == 2 (например "/show/running-config")
		leafKey = last
		leafValue = nil
		segments = segments[:len(segments)-1]
	}

	tree := map[string]any{leafKey: leafValue}
	for i := len(segments) - 1; i >= 0; i-- {
		tree = map[string]any{segments[i]: tree}
	}
	return tree, nil
}
