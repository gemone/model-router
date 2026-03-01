-- Migration: 001_destroy_rebuild.up.sql
-- Description: Destructive rebuild - DROP all old tables and CREATE new schema
-- WARNING: This will DELETE ALL EXISTING DATA
-- Usage: sqlite3 data.db < migrations/001_destroy_rebuild.up.sql

-- ============================================
-- STEP 1: DROP ALL EXISTING TABLES
-- ============================================

-- Drop tables in reverse dependency order to avoid foreign key constraints
DROP TABLE IF EXISTS session_messages;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS test_results;
DROP TABLE IF EXISTS stats;
DROP TABLE IF EXISTS request_logs;
DROP TABLE IF EXISTS api_keys;
DROP TABLE IF EXISTS route_rules;
DROP TABLE IF EXISTS models;
DROP TABLE IF EXISTS compression_model_groups;
DROP TABLE IF EXISTS composite_auto_models;
DROP TABLE IF EXISTS providers;
DROP TABLE IF EXISTS profiles;
DROP TABLE IF EXISTS settings;

-- ============================================
-- STEP 2: CREATE NEW TABLES
-- ============================================

-- Table: settings
-- System-wide configuration key-value store
CREATE TABLE settings (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Table: profiles
-- Defines routing configuration profiles (different API endpoints)
CREATE TABLE profiles (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    path TEXT UNIQUE NOT NULL,
    description TEXT,
    enabled BOOLEAN DEFAULT 1,
    priority INTEGER DEFAULT 0,
    settings TEXT,
    enable_compression BOOLEAN DEFAULT 0,
    compression_strategy TEXT,
    compression_level TEXT,
    compression_threshold INTEGER DEFAULT 0,
    max_context_window INTEGER DEFAULT 4096,
    enable_multi_model BOOLEAN DEFAULT 0,
    multi_model_config TEXT,
    default_compression_group TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Index for profile lookups
CREATE INDEX idx_profiles_name ON profiles(name);
CREATE INDEX idx_profiles_path ON profiles(path);
CREATE INDEX idx_profiles_enabled ON profiles(enabled) WHERE enabled = 1;

-- Table: providers
-- Model provider configurations (OpenAI, Claude, Azure, etc.)
CREATE TABLE providers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    type TEXT NOT NULL,
    base_url TEXT,
    api_key TEXT,
    deployment_id TEXT,
    api_version TEXT,
    enabled BOOLEAN DEFAULT 1,
    priority INTEGER DEFAULT 0,
    weight INTEGER DEFAULT 100,
    rate_limit INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Index for provider lookups
CREATE INDEX idx_providers_name ON providers(name);
CREATE INDEX idx_providers_type ON providers(type);
CREATE INDEX idx_providers_enabled ON providers(enabled) WHERE enabled = 1;

-- Table: models
-- Individual model configurations
CREATE TABLE models (
    id TEXT PRIMARY KEY,
    profile_id TEXT NOT NULL,
    provider_id TEXT NOT NULL,
    name TEXT NOT NULL,
    original_name TEXT,
    enabled BOOLEAN DEFAULT 1,
    supports_func BOOLEAN DEFAULT 0,
    supports_vision BOOLEAN DEFAULT 0,
    context_window INTEGER DEFAULT 4096,
    max_tokens INTEGER DEFAULT 4096,
    input_price REAL DEFAULT 0,
    output_price REAL DEFAULT 0,
    skip_compression BOOLEAN DEFAULT 0,
    scene TEXT,
    long_context_threshold INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE
);

-- Indexes for model lookups
CREATE INDEX idx_models_profile_id ON models(profile_id);
CREATE INDEX idx_models_provider_id ON models(provider_id);
CREATE INDEX idx_models_name ON models(name);
CREATE INDEX idx_models_enabled ON models(enabled) WHERE enabled = 1;

-- Table: compression_model_groups
-- Named groups of models for compression tasks
CREATE TABLE compression_model_groups (
    id TEXT PRIMARY KEY,
    profile_id TEXT NOT NULL,
    name TEXT NOT NULL,
    models TEXT,
    priority INTEGER DEFAULT 1,
    enabled BOOLEAN DEFAULT 1,
    health_threshold REAL DEFAULT 70.0,
    fallback_policy TEXT DEFAULT 'same_model',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE
);

-- Indexes for compression group lookups
CREATE INDEX idx_compression_group_profile_name ON compression_model_groups(profile_id, name);
CREATE INDEX idx_compression_group_enabled ON compression_model_groups(enabled) WHERE enabled = 1;

-- Table: composite_auto_models
-- Composite models for automatic routing with multiple backends
CREATE TABLE composite_auto_models (
    id TEXT PRIMARY KEY,
    profile_id TEXT NOT NULL,
    name TEXT NOT NULL,
    models TEXT,
    priority INTEGER DEFAULT 1,
    enabled BOOLEAN DEFAULT 1,
    health_threshold REAL DEFAULT 70.0,
    fallback_policy TEXT DEFAULT 'same_model',
    strategy TEXT,
    routing_rules TEXT,
    backend_models TEXT,
    aggregation TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE
);

-- Indexes for composite auto model lookups
CREATE INDEX idx_composite_auto_model_profile_name ON composite_auto_models(profile_id, name);
CREATE INDEX idx_composite_auto_model_enabled ON composite_auto_models(enabled) WHERE enabled = 1;

-- Table: route_rules
-- Routing rules for model selection
CREATE TABLE route_rules (
    id TEXT PRIMARY KEY,
    profile_id TEXT NOT NULL,
    name TEXT NOT NULL,
    model_pattern TEXT NOT NULL,
    target_models TEXT,
    strategy TEXT DEFAULT 'priority',
    fallback_enabled BOOLEAN DEFAULT 1,
    fallback_models TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (profile_id) REFERENCES profiles(id) ON DELETE CASCADE
);

-- Indexes for route rule lookups
CREATE INDEX idx_route_rules_profile_id ON route_rules(profile_id);
CREATE INDEX idx_route_rules_model_pattern ON route_rules(model_pattern);

-- Table: api_keys
-- API key management for client authentication
CREATE TABLE api_keys (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    key TEXT UNIQUE NOT NULL,
    enabled BOOLEAN DEFAULT 1,
    rate_limit INTEGER DEFAULT 0,
    allowed_models TEXT,
    allowed_profiles TEXT,
    expired_at DATETIME,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for API key lookups
CREATE INDEX idx_api_keys_key ON api_keys(key);
CREATE INDEX idx_api_keys_enabled ON api_keys(enabled) WHERE enabled = 1;

-- Table: request_logs
-- API request logging for statistics and debugging
CREATE TABLE request_logs (
    id TEXT PRIMARY KEY,
    request_id TEXT NOT NULL,
    model TEXT NOT NULL,
    provider_id TEXT NOT NULL,
    status TEXT NOT NULL,
    latency INTEGER NOT NULL,
    prompt_tokens INTEGER DEFAULT 0,
    completion_tokens INTEGER DEFAULT 0,
    total_tokens INTEGER DEFAULT 0,
    error_message TEXT,
    client_ip TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for request log queries
CREATE INDEX idx_request_logs_request_id ON request_logs(request_id);
CREATE INDEX idx_request_logs_model ON request_logs(model);
CREATE INDEX idx_request_logs_provider_id ON request_logs(provider_id);
CREATE INDEX idx_request_logs_status ON request_logs(status);
CREATE INDEX idx_request_logs_created_at ON request_logs(created_at);

-- Table: stats
-- Aggregated statistics for analytics
CREATE TABLE stats (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    date TEXT NOT NULL,
    hour INTEGER NOT NULL,
    provider_id TEXT NOT NULL,
    model TEXT NOT NULL,
    request_count INTEGER DEFAULT 0,
    success_count INTEGER DEFAULT 0,
    error_count INTEGER DEFAULT 0,
    total_tokens INTEGER DEFAULT 0,
    avg_latency REAL DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for stats queries
CREATE INDEX idx_stats_date ON stats(date);
CREATE INDEX idx_stats_hour ON stats(hour);
CREATE INDEX idx_stats_provider_id ON stats(provider_id);
CREATE INDEX idx_stats_model ON stats(model);

-- Table: test_results
-- Model testing results for health checks
CREATE TABLE test_results (
    id TEXT PRIMARY KEY,
    provider_id TEXT NOT NULL,
    model TEXT NOT NULL,
    success BOOLEAN NOT NULL,
    latency INTEGER NOT NULL,
    error TEXT,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for test result queries
CREATE INDEX idx_test_results_provider_id ON test_results(provider_id);
CREATE INDEX idx_test_results_model ON test_results(model);
CREATE INDEX idx_test_results_created_at ON test_results(created_at);

-- Table: sessions
-- Long-term session context management
CREATE TABLE sessions (
    id TEXT PRIMARY KEY,
    api_key_id TEXT NOT NULL,
    profile_id TEXT NOT NULL,
    session_key TEXT UNIQUE NOT NULL,
    context_window INTEGER DEFAULT 4096,
    compressed_tokens INTEGER DEFAULT 0,
    last_summary_at DATETIME,
    summary_version INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

-- Indexes for session lookups
CREATE INDEX idx_sessions_api_key_id ON sessions(api_key_id);
CREATE INDEX idx_sessions_profile_id ON sessions(profile_id);
CREATE INDEX idx_sessions_session_key ON sessions(session_key);

-- Table: session_messages
-- Individual messages within sessions
CREATE TABLE session_messages (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id TEXT NOT NULL,
    role TEXT NOT NULL,
    content TEXT NOT NULL,
    tokens INTEGER DEFAULT 0,
    is_compressed BOOLEAN DEFAULT 0,
    compression_level INTEGER DEFAULT 0,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (session_id) REFERENCES sessions(id) ON DELETE CASCADE
);

-- Indexes for session message queries
CREATE INDEX idx_session_messages_session_id ON session_messages(session_id);
CREATE INDEX idx_session_messages_created_at ON session_messages(created_at);

-- ============================================
-- STEP 3: INSERT DEFAULT SETTINGS
-- ============================================

INSERT INTO settings (key, value) VALUES
    ('admin_token', 'admin-token-change-me'),
    ('jwt_secret', 'jwt-secret-change-me'),
    ('log_retention_days', '30'),
    ('max_request_size_mb', '10'),
    ('enable_stats', 'true'),
    ('stats_retention_days', '90'),
    ('default_profile', 'default'),
    ('language', 'en');

-- ============================================
-- MIGRATION COMPLETE
-- ============================================

-- Print completion message
SELECT 'Migration 001_destroy_rebuild completed successfully!' AS message;
SELECT 'All tables have been recreated with the new schema.' AS details;
