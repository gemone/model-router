// Package plugin provides a plugin system for extensibility.
package plugin

import (
	"fmt"
	"os"
	"path/filepath"
	"plugin"
	"sync"

	"github.com/gemone/model-router/internal/model"
)

// Plugin defines the interface for all plugins
type Plugin interface {
	// Name returns the plugin name
	Name() string

	// Version returns the plugin version
	Version() string

	// Init initializes the plugin with configuration
	Init(config map[string]interface{}) error

	// Close cleans up plugin resources
	Close() error

	// Handles returns the API paths this plugin handles
	Handles() []string

	// Handle processes an API request
	Handle(ctx *PluginContext) (*PluginResponse, error)
}

// PluginContext provides context for plugin execution
type PluginContext struct {
	RequestID  string
	Method     string
	Path       string
	Headers    map[string]string
	Body       []byte
	Profile    *model.Profile
	Provider   *model.Provider
	Metadata   map[string]interface{}
}

// PluginResponse represents a plugin's response
type PluginResponse struct {
	StatusCode int
	Headers    map[string]string
	Body       []byte
	Error      error
}

// Manager manages plugin lifecycle
type Manager struct {
	mu      sync.RWMutex
	plugins map[string]Plugin
	config  map[string]map[string]interface{}
	paths   []string // Registered API paths
}

// NewManager creates a new plugin manager
func NewManager() *Manager {
	return &Manager{
		plugins: make(map[string]Plugin),
		config:  make(map[string]map[string]interface{}),
		paths:   make([]string, 0),
	}
}

// Register registers a plugin programmatically
func (m *Manager) Register(p Plugin) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name := p.Name()
	if _, exists := m.plugins[name]; exists {
		return fmt.Errorf("plugin already registered: %s", name)
	}

	// Initialize plugin with empty config
	if err := p.Init(nil); err != nil {
		return fmt.Errorf("failed to initialize plugin %s: %w", name, err)
	}

	m.plugins[name] = p

	// Register plugin paths
	for _, path := range p.Handles() {
		m.paths = append(m.paths, path)
	}

	return nil
}

// LoadFromFile loads a plugin from a .so file
func (m *Manager) LoadFromFile(soPath string, config map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Load the plugin
	plug, err := plugin.Open(soPath)
	if err != nil {
		return fmt.Errorf("failed to load plugin from %s: %w", soPath, err)
	}

	// Look up the Plugin symbol
	sym, err := plug.Lookup("Plugin")
	if err != nil {
		return fmt.Errorf("plugin does not export Plugin symbol: %w", err)
	}

	// Type assert to Plugin interface
	pluginImpl, ok := sym.(Plugin)
	if !ok {
		return fmt.Errorf("plugin does not implement Plugin interface")
	}

	// Initialize plugin
	if err := pluginImpl.Init(config); err != nil {
		return fmt.Errorf("failed to initialize plugin: %w", err)
	}

	name := pluginImpl.Name()
	m.plugins[name] = pluginImpl
	m.config[name] = config

	// Register plugin paths
	for _, path := range pluginImpl.Handles() {
		m.paths = append(m.paths, path)
	}

	return nil
}

// LoadFromDirectory loads all plugins from a directory
func (m *Manager) LoadFromDirectory(dir string, configs map[string]map[string]interface{}) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("failed to read plugin directory: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// Look for .so files
		if filepath.Ext(entry.Name()) == ".so" {
			soPath := filepath.Join(dir, entry.Name())
			config := configs[entry.Name()]
			if err := m.LoadFromFile(soPath, config); err != nil {
				// Log error but continue loading other plugins
				fmt.Printf("Warning: failed to load plugin %s: %v\n", entry.Name(), err)
			}
		}
	}

	return nil
}

// Unload unloads a plugin by name
func (m *Manager) Unload(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, exists := m.plugins[name]
	if !exists {
		return fmt.Errorf("plugin not found: %s", name)
	}

	// Close plugin
	if err := p.Close(); err != nil {
		return fmt.Errorf("failed to close plugin %s: %w", name, err)
	}

	// Remove from registry
	delete(m.plugins, name)
	delete(m.config, name)

	// Rebuild paths
	m.rebuildPaths()

	return nil
}

// Get retrieves a plugin by name
func (m *Manager) Get(name string) (Plugin, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.plugins[name]
	return p, ok
}

// List returns all registered plugin names
func (m *Manager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.plugins))
	for name := range m.plugins {
		names = append(names, name)
	}
	return names
}

// ListPlugins returns all registered plugins with their info
func (m *Manager) ListPlugins() []PluginInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	info := make([]PluginInfo, 0, len(m.plugins))
	for _, p := range m.plugins {
		info = append(info, PluginInfo{
			Name:    p.Name(),
			Version: p.Version(),
			Handles: p.Handles(),
		})
	}
	return info
}

// Handle routes a request to the appropriate plugin
func (m *Manager) Handle(ctx *PluginContext) (*PluginResponse, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, p := range m.plugins {
		for _, path := range p.Handles() {
			if ctx.Path == path {
				resp, _ := p.Handle(ctx)
				return resp, true
			}
		}
	}

	return nil, false
}

// GetPaths returns all registered API paths from plugins
func (m *Manager) GetPaths() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	paths := make([]string, len(m.paths))
	copy(paths, m.paths)
	return paths
}

// rebuildPaths rebuilds the paths list from all plugins
func (m *Manager) rebuildPaths() {
	m.paths = make([]string, 0)
	for _, p := range m.plugins {
		for _, path := range p.Handles() {
			m.paths = append(m.paths, path)
		}
	}
}

// Close closes all plugins
func (m *Manager) Close() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	var lastErr error
	for name, p := range m.plugins {
		if err := p.Close(); err != nil {
			lastErr = fmt.Errorf("error closing plugin %s: %w", name, err)
		}
	}

	m.plugins = make(map[string]Plugin)
	m.config = make(map[string]map[string]interface{})
	m.paths = make([]string, 0)

	return lastErr
}

// PluginInfo contains information about a plugin
type PluginInfo struct {
	Name    string
	Version string
	Handles []string
}

// BasePlugin provides a base implementation for plugins
type BasePlugin struct {
	name    string
	version string
	handles []string
}

// NewBasePlugin creates a new base plugin
func NewBasePlugin(name, version string, handles []string) *BasePlugin {
	return &BasePlugin{
		name:    name,
		version: version,
		handles: handles,
	}
}

// Name returns the plugin name
func (b *BasePlugin) Name() string {
	return b.name
}

// Version returns the plugin version
func (b *BasePlugin) Version() string {
	return b.version
}

// Handles returns the API paths this plugin handles
func (b *BasePlugin) Handles() []string {
	return b.handles
}

// Init initializes the plugin (no-op for base)
func (b *BasePlugin) Init(config map[string]interface{}) error {
	return nil
}

// Close cleans up plugin resources (no-op for base)
func (b *BasePlugin) Close() error {
	return nil
}

// Handle processes an API request (must be implemented by derived plugins)
func (b *BasePlugin) Handle(ctx *PluginContext) (*PluginResponse, error) {
	return nil, fmt.Errorf("not implemented")
}
