# Schema Reference

## Table: `profiles`

Routing configuration profiles for different API endpoints.

| Column | Type | Description |
|--------|------|-------------|
| id | TEXT | Primary key |
| name | TEXT | Display name |
| path | TEXT | URI path (unique) |
| description | TEXT | Profile description |
| enabled | BOOLEAN | Is profile active |
| priority | INTEGER | Priority order |
| settings | TEXT | JSON settings |
| enable_compression | BOOLEAN | Enable context compression |
| compression_strategy | TEXT | rolling/summary/hybrid |
| compression_level | TEXT | session/threshold |
| compression_threshold | INTEGER | Token threshold |
| max_context_window | INTEGER | Max context size |
| enable_multi_model | BOOLEAN | Enable multi-model |
| multi_model_config | TEXT | JSON config |
| default_compression_group | TEXT | Default group name |
| created_at | DATETIME | Creation timestamp |
| updated_at | DATETIME | Update timestamp |

**Indexes:**
- `idx_profiles_name` - Name lookup
- `idx_profiles_path` - Path lookup (unique)
- `idx_profiles_enabled` - Filter enabled profiles

---

## Table: `providers`

Model provider configurations (OpenAI, Claude, Azure, etc.).

| Column | Type | Description |
|--------|------|-------------|
| id | TEXT | Primary key |
| name | TEXT | Provider name |
| type | TEXT | Provider type |
| base_url | TEXT | API base URL |
| api_key | TEXT | Encrypted API key |
| deployment_id | TEXT | Azure deployment name |
| api_version | TEXT | Azure API version |
| enabled | BOOLEAN | Is provider active |
| priority | INTEGER | Priority order |
| weight | INTEGER | Load balance weight |
| rate_limit | INTEGER | Requests per minute |
| created_at | DATETIME | Creation timestamp |
| updated_at | DATETIME | Update timestamp |

**Indexes:**
- `idx_providers_name` - Name lookup
- `idx_providers_type` - Type filter
- `idx_providers_enabled` - Filter enabled providers

---

## Table: `models`

Individual model configurations.

| Column | Type | Description |
|--------|------|-------------|
| id | TEXT | Primary key |
| profile_id | TEXT | FK to profiles |
| provider_id | TEXT | FK to providers |
| name | TEXT | Exposed model name |
| original_name | TEXT | Original provider name |
| enabled | BOOLEAN | Is model active |
| supports_func | BOOLEAN | Supports function calling |
| supports_vision | BOOLEAN | Supports vision |
| context_window | INTEGER | Context size |
| max_tokens | INTEGER | Max output tokens |
| input_price | REAL | Input price/1K tokens |
| output_price | REAL | Output price/1K tokens |
| skip_compression | BOOLEAN | Skip compression |
| scene | TEXT | Scene tag |
| long_context_threshold | INTEGER | Token threshold |
| created_at | DATETIME | Creation timestamp |
| updated_at | DATETIME | Update timestamp |

**Indexes:**
- `idx_models_profile_id` - Profile lookup
- `idx_models_provider_id` - Provider lookup
- `idx_models_name` - Name lookup
- `idx_models_enabled` - Filter enabled models

---

## Table: `compression_model_groups`

Named groups of models for compression tasks.

| Column | Type | Description |
|--------|------|-------------|
| id | TEXT | Primary key |
| profile_id | TEXT | FK to profiles |
| name | TEXT | Group name |
| models | TEXT | JSON array of ModelReference |
| priority | INTEGER | Priority order |
| enabled | BOOLEAN | Is group active |
| health_threshold | REAL | Health % threshold |
| fallback_policy | TEXT | Fallback strategy |
| created_at | DATETIME | Creation timestamp |
| updated_at | DATETIME | Update timestamp |

**Indexes:**
- `idx_compression_group_profile_name` - Composite lookup
- `idx_compression_group_enabled` - Filter enabled

---

## Table: `composite_auto_models`

Composite models with automatic routing.

| Column | Type | Description |
|--------|------|-------------|
| id | TEXT | Primary key |
| profile_id | TEXT | FK to profiles |
| name | TEXT | Composite model name |
| models | TEXT | JSON array of ModelReference |
| priority | INTEGER | Priority order |
| enabled | BOOLEAN | Is model active |
| health_threshold | REAL | Health % threshold |
| fallback_policy | TEXT | Fallback strategy |
| strategy | TEXT | Routing strategy |
| routing_rules | TEXT | JSON routing rules |
| backend_models | TEXT | JSON backend models |
| aggregation | TEXT | JSON aggregation config |
| created_at | DATETIME | Creation timestamp |
| updated_at | DATETIME | Update timestamp |

**Indexes:**
- `idx_composite_auto_model_profile_name` - Composite lookup
- `idx_composite_auto_model_enabled` - Filter enabled

---

## Table: `route_rules`

Routing rules for model selection.

| Column | Type | Description |
|--------|------|-------------|
| id | TEXT | Primary key |
| profile_id | TEXT | FK to profiles |
| name | TEXT | Rule name |
| model_pattern | TEXT | Model match pattern |
| target_models | TEXT | JSON target models |
| strategy | TEXT | Route strategy |
| fallback_enabled | BOOLEAN | Enable fallback |
| fallback_models | TEXT | JSON fallback models |
| created_at | DATETIME | Creation timestamp |
| updated_at | DATETIME | Update timestamp |

**Indexes:**
- `idx_route_rules_profile_id` - Profile lookup
- `idx_route_rules_model_pattern` - Pattern matching

---

## Table: `api_keys`

Client API key management.

| Column | Type | Description |
|--------|------|-------------|
| id | TEXT | Primary key |
| name | TEXT | Key name |
| key | TEXT | API key value (unique) |
| enabled | BOOLEAN | Is key active |
| rate_limit | INTEGER | Requests per minute |
| allowed_models | TEXT | JSON allowed models |
| allowed_profiles | TEXT | JSON allowed profiles |
| expired_at | DATETIME | Expiration date |
| created_at | DATETIME | Creation timestamp |
| updated_at | DATETIME | Update timestamp |

**Indexes:**
- `idx_api_keys_key` - Key lookup (unique)
- `idx_api_keys_enabled` - Filter enabled

---

## Table: `request_logs`

API request logging.

| Column | Type | Description |
|--------|------|-------------|
| id | TEXT | Primary key |
| request_id | TEXT | Request ID |
| model | TEXT | Model used |
| provider_id | TEXT | Provider used |
| status | TEXT | success/error/timeout |
| latency | INTEGER | Latency in ms |
| prompt_tokens | INTEGER | Input tokens |
| completion_tokens | INTEGER | Output tokens |
| total_tokens | INTEGER | Total tokens |
| error_message | TEXT | Error details |
| client_ip | TEXT | Client IP address |
| created_at | DATETIME | Request timestamp |

**Indexes:**
- `idx_request_logs_request_id` - Request lookup
- `idx_request_logs_model` - Model filter
- `idx_request_logs_provider_id` - Provider filter
- `idx_request_logs_status` - Status filter
- `idx_request_logs_created_at` - Time range queries

---

## Table: `stats`

Aggregated statistics.

| Column | Type | Description |
|--------|------|-------------|
| id | INTEGER | Primary key (auto) |
| date | TEXT | YYYY-MM-DD |
| hour | INTEGER | 0-23 |
| provider_id | TEXT | Provider ID |
| model | TEXT | Model name |
| request_count | INTEGER | Total requests |
| success_count | INTEGER | Successful requests |
| error_count | INTEGER | Failed requests |
| total_tokens | INTEGER | Total tokens |
| avg_latency | REAL | Average latency |
| created_at | DATETIME | Creation timestamp |
| updated_at | DATETIME | Update timestamp |

**Indexes:**
- `idx_stats_date` - Date filter
- `idx_stats_hour` - Hour filter
- `idx_stats_provider_id` - Provider filter
- `idx_stats_model` - Model filter

---

## Table: `test_results`

Model health check results.

| Column | Type | Description |
|--------|------|-------------|
| id | TEXT | Primary key |
| provider_id | TEXT | Provider ID |
| model | TEXT | Model name |
| success | BOOLEAN | Test result |
| latency | INTEGER | Latency in ms |
| error | TEXT | Error message |
| created_at | DATETIME | Test timestamp |

**Indexes:**
- `idx_test_results_provider_id` - Provider lookup
- `idx_test_results_model` - Model lookup
- `idx_test_results_created_at` - Time filter

---

## Table: `sessions`

Long-term session contexts.

| Column | Type | Description |
|--------|------|-------------|
| id | TEXT | Primary key |
| api_key_id | TEXT | API key ID |
| profile_id | TEXT | Profile ID |
| session_key | TEXT | Session key (unique) |
| context_window | INTEGER | Max context |
| compressed_tokens | INTEGER | Compressed count |
| last_summary_at | DATETIME | Last summary |
| summary_version | INTEGER | Summary version |
| created_at | DATETIME | Creation timestamp |
| updated_at | DATETIME | Update timestamp |

**Indexes:**
- `idx_sessions_api_key_id` - API key lookup
- `idx_sessions_profile_id` - Profile lookup
- `idx_sessions_session_key` - Session key (unique)

---

## Table: `session_messages`

Messages within sessions.

| Column | Type | Description |
|--------|------|-------------|
| id | INTEGER | Primary key (auto) |
| session_id | TEXT | FK to sessions |
| role | TEXT | user/assistant/system |
| content | TEXT | Message content |
| tokens | INTEGER | Token count |
| is_compressed | BOOLEAN | Is compressed |
| compression_level | INTEGER | Compression level |
| created_at | DATETIME | Message timestamp |

**Indexes:**
- `idx_session_messages_session_id` - Session lookup
- `idx_session_messages_created_at` - Time ordering

---

## Table: `settings`

System configuration key-value store.

| Column | Type | Description |
|--------|------|-------------|
| key | TEXT | Primary key |
| value | TEXT | Configuration value |
| created_at | DATETIME | Creation timestamp |
| updated_at | DATETIME | Update timestamp |

**Default Settings:**
- `admin_token` - Admin authentication token
- `jwt_secret` - JWT signing secret
- `log_retention_days` - Log retention period
- `max_request_size_mb` - Max request size
- `enable_stats` - Enable statistics
- `stats_retention_days` - Stats retention period
- `default_profile` - Default profile name
- `language` - UI language
