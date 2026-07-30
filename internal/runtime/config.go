package runtime

import (
	"context"
	"encoding/json"
	"os"
	"strings"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcfg"
)

// EnableEnvironmentConfig makes the repository's documented GF_* variables
// authoritative over the file adapter. GoFrame's Config.MustGet reads only the
// configured adapter; without this layer those variables never reach Identity's
// application settings or nested database/Redis maps.
func EnableEnvironmentConfig() {
	config := g.Cfg()
	if _, enabled := config.GetAdapter().(*environmentAdapter); enabled {
		return
	}
	config.SetAdapter(&environmentAdapter{base: config.GetAdapter()})
}

type environmentAdapter struct {
	base gcfg.Adapter
}

func (adapter *environmentAdapter) Available(
	ctx context.Context,
	resource ...string,
) bool {
	return adapter.base.Available(ctx, resource...)
}

func (adapter *environmentAdapter) Get(
	ctx context.Context,
	pattern string,
) (any, error) {
	if raw, found := os.LookupEnv(environmentKey(pattern)); found {
		return decodeEnvironmentValue(raw), nil
	}
	value, err := adapter.base.Get(ctx, pattern)
	if err != nil {
		return nil, err
	}
	return overlayEnvironment(pattern, value), nil
}

func (adapter *environmentAdapter) Data(ctx context.Context) (map[string]any, error) {
	data, err := adapter.base.Data(ctx)
	if err != nil {
		return nil, err
	}
	overlaid, _ := overlayEnvironment("", data).(map[string]any)
	return overlaid, nil
}

func (adapter *environmentAdapter) AddWatcher(name string, fn gcfg.WatcherFunc) {
	if watcher, ok := adapter.base.(gcfg.WatcherAdapter); ok {
		watcher.AddWatcher(name, fn)
	}
}

func (adapter *environmentAdapter) RemoveWatcher(name string) {
	if watcher, ok := adapter.base.(gcfg.WatcherAdapter); ok {
		watcher.RemoveWatcher(name)
	}
}

func (adapter *environmentAdapter) GetWatcherNames() []string {
	if watcher, ok := adapter.base.(gcfg.WatcherAdapter); ok {
		return watcher.GetWatcherNames()
	}
	return nil
}

func (adapter *environmentAdapter) IsWatching(name string) bool {
	if watcher, ok := adapter.base.(gcfg.WatcherAdapter); ok {
		return watcher.IsWatching(name)
	}
	return false
}

func overlayEnvironment(pattern string, value any) any {
	if pattern != "" {
		if raw, found := os.LookupEnv(environmentKey(pattern)); found {
			return decodeEnvironmentValue(raw)
		}
	}
	switch current := value.(type) {
	case map[string]any:
		copied := make(map[string]any, len(current))
		for key, child := range current {
			childPattern := key
			if pattern != "" {
				childPattern = pattern + "." + key
			}
			copied[key] = overlayEnvironment(childPattern, child)
		}
		return copied
	case []any:
		copied := make([]any, len(current))
		for index, child := range current {
			copied[index] = overlayEnvironment(pattern, child)
		}
		return copied
	default:
		return value
	}
}

func environmentKey(pattern string) string {
	return "GF_" + strings.ToUpper(strings.ReplaceAll(pattern, ".", "_"))
}

func decodeEnvironmentValue(raw string) any {
	var decoded any
	if json.Valid([]byte(raw)) && json.Unmarshal([]byte(raw), &decoded) == nil {
		return decoded
	}
	return raw
}
