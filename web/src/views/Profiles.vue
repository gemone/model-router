<template>
    <div class="profiles-page">
        <!-- 面包屑导航 -->
        <div class="breadcrumb-wrapper">
            <el-breadcrumb separator="/">
                <el-breadcrumb-item :to="{ path: '/' }"
                    >Home</el-breadcrumb-item
                >
                <el-breadcrumb-item>{{
                    $t("profile.title")
                }}</el-breadcrumb-item>
            </el-breadcrumb>
        </div>

        <div class="page-header">
            <h2>{{ $t("profile.title") }}</h2>
            <el-button type="primary" class="add-btn" @click="showAddDialog">
                <el-icon><Plus /></el-icon>
                {{ $t("profile.addProfile") }}
            </el-button>
        </div>

        <div class="profiles-grid">
            <div
                v-for="(profile, index) in profiles"
                :key="profile.id"
                class="profile-card"
                :style="{
                    '--accent-color': accentColors[index % accentColors.length],
                }"
            >
                <div class="card-accent"></div>
                <div class="card-content">
                    <div class="card-header">
                        <div class="profile-name">
                            <el-icon><Grid /></el-icon>
                            {{ profile.name }}
                        </div>
                        <el-tag
                            :type="profile.enabled ? 'success' : 'info'"
                            size="small"
                            class="status-tag"
                        >
                            {{
                                profile.enabled
                                    ? $t("common.enabled")
                                    : $t("common.disabled")
                            }}
                        </el-tag>
                    </div>

                    <div class="profile-details">
                        <div class="detail-item">
                            <span class="detail-label">{{
                                $t("profile.path")
                            }}</span>
                            <div class="endpoint-wrapper">
                                <code class="endpoint"
                                    >/api/{{ profile.path }}</code
                                >
                                <el-button
                                    link
                                    size="small"
                                    class="copy-btn"
                                    @click="copyEndpoint(profile.path)"
                                >
                                    <el-icon><CopyDocument /></el-icon>
                                </el-button>
                            </div>
                        </div>
                        <div class="detail-item">
                            <span class="detail-label">API 端点格式</span>
                            <div class="api-endpoints">
                                <div class="endpoint-item">
                                    <code class="endpoint-small"
                                        >POST /api/{{
                                            profile.path
                                        }}/v1/chat/completions</code
                                    >
                                </div>
                                <div class="endpoint-item">
                                    <code class="endpoint-small"
                                        >POST /api/openai/{{
                                            profile.path
                                        }}/v1/chat/completions</code
                                    >
                                </div>
                                <div class="endpoint-item">
                                    <code class="endpoint-small"
                                        >POST /{{
                                            profile.path
                                        }}/v1/chat/completions</code
                                    >
                                </div>
                                <el-divider class="endpoint-divider"
                                    >其他格式</el-divider
                                >
                                <div class="endpoint-item">
                                    <code class="endpoint-small"
                                        >POST /api/claude/{{
                                            profile.path
                                        }}/v1/messages</code
                                    >
                                </div>
                                <div class="endpoint-item">
                                    <code class="endpoint-small"
                                        >POST /api/ollama/{{
                                            profile.path
                                        }}/api/chat</code
                                    >
                                </div>
                                <div class="endpoint-item">
                                    <code class="endpoint-small"
                                        >POST /api/ollama/{{
                                            profile.path
                                        }}/api/generate</code
                                    >
                                </div>
                            </div>
                        </div>
                        <div class="detail-item" v-if="profile.api_token_enc">
                            <span class="detail-label">认证状态</span>
                            <el-tag type="warning" size="small"
                                >需要 Token</el-tag
                            >
                        </div>
                        <div class="detail-item">
                            <span class="detail-label">{{
                                $t("profile.modelsCount")
                            }}</span>
                            <span class="detail-value">
                                <el-icon><Cpu /></el-icon>
                                {{ profile.model_ids?.length || 0 }}
                            </span>
                        </div>
                        <div
                            class="detail-item"
                            v-if="profile.route_ids?.length > 0"
                        >
                            <span class="detail-label">{{
                                $t("profile.routesCount") || "路由"
                            }}</span>
                            <span class="detail-value">
                                <el-icon><Share /></el-icon>
                                {{ profile.route_ids?.length || 0 }}
                            </span>
                        </div>
                        <div class="detail-item">
                            <span class="detail-label">{{
                                $t("common.priority")
                            }}</span>
                            <el-rate
                                v-model="profile.priority"
                                disabled
                                :max="5"
                                class="priority-rate"
                            />
                        </div>
                    </div>

                    <div class="card-actions">
                        <el-button
                            link
                            type="primary"
                            size="small"
                            @click="editProfile(profile)"
                        >
                            <el-icon><Edit /></el-icon>
                            {{ $t("common.edit") }}
                        </el-button>
                        <el-button
                            link
                            type="danger"
                            size="small"
                            @click="confirmDelete(profile)"
                        >
                            <el-icon><Delete /></el-icon>
                            {{ $t("common.delete") }}
                        </el-button>
                    </div>
                </div>
            </div>
        </div>

        <!-- Add/Edit Dialog -->
        <el-dialog
            v-model="dialogVisible"
            :title="
                isEdit ? $t('profile.editProfile') : $t('profile.addProfile')
            "
            width="500px"
        >
            <el-form
                :model="form"
                :rules="rules"
                ref="formRef"
                label-width="120px"
            >
                <el-form-item :label="$t('profile.name')" prop="name">
                    <el-input v-model="form.name" />
                </el-form-item>
                <el-form-item :label="$t('profile.path')" prop="path">
                    <el-input v-model="form.path">
                        <template #prefix>/api/</template>
                    </el-input>
                    <div class="form-tip">{{ $t("profile.pathTip") }}</div>
                </el-form-item>
                <el-form-item :label="$t('common.description')">
                    <el-input
                        v-model="form.description"
                        type="textarea"
                        :rows="2"
                    />
                </el-form-item>
                <el-form-item :label="$t('common.priority')">
                    <el-slider v-model="form.priority" :max="10" show-stops />
                </el-form-item>
                <el-form-item :label="$t('common.status')">
                    <el-switch v-model="form.enabled" />
                </el-form-item>
                <el-form-item :label="$t('profile.models')">
                    <el-select
                        v-model="form.model_ids"
                        multiple
                        collapse-tags
                        collapse-tags-tooltip
                        :placeholder="$t('profile.selectModels')"
                        style="width: 100%"
                    >
                        <el-option
                            v-for="model in store.models"
                            :key="model.id"
                            :label="
                                model.name +
                                ' (' +
                                (model.original_name || model.name) +
                                ')'
                            "
                            :value="model.id"
                        />
                    </el-select>
                </el-form-item>
                <el-form-item :label="$t('profile.routes') || '绑定路由'">
                    <el-select
                        v-model="form.route_ids"
                        multiple
                        collapse-tags
                        collapse-tags-tooltip
                        :placeholder="
                            $t('profile.selectRoutes') || '选择路由策略'
                        "
                        style="width: 100%"
                    >
                        <el-option
                            v-for="route in store.routeRules"
                            :key="route.id"
                            :label="
                                route.name +
                                ' (' +
                                (route.strategy || 'auto') +
                                ')'
                            "
                            :value="route.id"
                        />
                    </el-select>
                    <div class="form-tip">
                        {{
                            $t("profile.routesTip") ||
                            "通过路由策略动态选择模型"
                        }}
                    </div>
                </el-form-item>
                <el-form-item label="API Token">
                    <el-input
                        v-model="form.api_token"
                        type="password"
                        show-password
                        placeholder="留空则不需要认证"
                    />
                    <div class="form-tip">
                        为此 Profile 设置独立的访问令牌。如果设置了
                        Token，客户端需要在请求头中提供
                        <code>Authorization: Bearer &lt;token&gt;</code>
                        或查询参数 <code>?token=&lt;token&gt;</code>
                    </div>
                </el-form-item>

                <!-- 压缩配置 -->
                <el-divider content-position="left">{{
                    $t("profile.compressionSettings") || "上下文压缩配置"
                }}</el-divider>
                <el-form-item
                    :label="$t('profile.enableCompression') || '启用压缩'"
                >
                    <el-switch v-model="form.enable_compression" />
                </el-form-item>
                <template v-if="form.enable_compression">
                    <el-form-item
                        :label="$t('profile.compressionStrategy') || '压缩策略'"
                    >
                        <el-select
                            v-model="form.compression_strategy"
                            style="width: 100%"
                        >
                            <el-option
                                :label="
                                    $t('profile.strategyRolling') ||
                                    '滚动窗口 (rolling)'
                                "
                                value="rolling"
                            />
                            <el-option
                                :label="
                                    $t('profile.strategySummary') ||
                                    '摘要总结 (summary)'
                                "
                                value="summary"
                            />
                            <el-option
                                :label="
                                    $t('profile.strategyHybrid') ||
                                    '混合模式 (hybrid)'
                                "
                                value="hybrid"
                            />
                        </el-select>
                    </el-form-item>
                    <el-form-item
                        :label="$t('profile.compressionLevel') || '压缩级别'"
                    >
                        <el-select
                            v-model="form.compression_level"
                            style="width: 100%"
                        >
                            <el-option
                                :label="
                                    $t('profile.levelSession') ||
                                    '每次会话 (session)'
                                "
                                value="session"
                            />
                            <el-option
                                :label="
                                    $t('profile.levelThreshold') ||
                                    '达到阈值 (threshold)'
                                "
                                value="threshold"
                            />
                        </el-select>
                    </el-form-item>
                    <el-form-item
                        :label="
                            $t('profile.compressionThreshold') || '压缩阈值'
                        "
                        v-if="form.compression_level === 'threshold'"
                    >
                        <el-input-number
                            v-model="form.compression_threshold"
                            :min="1000"
                            :max="100000"
                            :step="1000"
                        />
                        <div class="form-tip">
                            {{
                                $t("profile.thresholdTip") ||
                                "当 token 数量超过此阈值时触发压缩"
                            }}
                        </div>
                    </el-form-item>
                    <el-form-item
                        :label="
                            $t('profile.maxContextWindow') || '最大上下文窗口'
                        "
                    >
                        <el-input-number
                            v-model="form.max_context_window"
                            :min="1000"
                            :max="200000"
                            :step="1000"
                        />
                        <div class="form-tip">
                            {{
                                $t("profile.maxContextTip") ||
                                "最大允许的上下文窗口大小"
                            }}
                        </div>
                    </el-form-item>
                </template>
            </el-form>
            <template #footer>
                <el-button @click="dialogVisible = false">{{
                    $t("common.cancel")
                }}</el-button>
                <el-button type="primary" @click="saveProfile">{{
                    $t("common.save")
                }}</el-button>
            </template>
        </el-dialog>
    </div>
</template>

<script setup>
import { ref, computed, onMounted } from "vue";
import { ElMessage, ElMessageBox } from "element-plus";
import { useI18n } from "vue-i18n";
import { useAppStore } from "@/stores/app";

const { t } = useI18n();
const store = useAppStore();
const profiles = computed(() => store.profiles);

// Accent colors for profile cards
const accentColors = [
    "#3B82F6", // blue
    "#10B981", // green
    "#F59E0B", // amber
    "#EF4444", // red
    "#8B5CF6", // violet
    "#EC4899", // pink
    "#06B6D4", // cyan
    "#F97316", // orange
];

const dialogVisible = ref(false);
const isEdit = ref(false);
const formRef = ref();
const form = ref({
    name: "",
    path: "",
    description: "",
    priority: 0,
    enabled: true,
    model_ids: [],
    route_ids: [], // 绑定的路由ID列表
    api_token: "", // API token for profile authentication
    // Compression settings
    enable_compression: false,
    compression_strategy: "rolling",
    compression_level: "threshold",
    compression_threshold: 8000,
    max_context_window: 16000,
});

const rules = {
    name: [
        {
            required: true,
            message: t("message.inputRequired"),
            trigger: "blur",
        },
    ],
    path: [
        {
            required: true,
            message: t("message.inputRequired"),
            trigger: "blur",
        },
    ],
};

function showAddDialog() {
    isEdit.value = false;
    form.value = {
        name: "",
        path: "",
        description: "",
        priority: 0,
        enabled: true,
        model_ids: [],
        route_ids: [],
        api_token: "",
        // Compression settings
        enable_compression: false,
        compression_strategy: "rolling",
        compression_level: "threshold",
        compression_threshold: 8000,
        max_context_window: 16000,
    };
    dialogVisible.value = true;
}

function editProfile(profile) {
    isEdit.value = true;
    form.value = { ...profile };
    dialogVisible.value = true;
}

async function saveProfile() {
    await formRef.value.validate();
    try {
        if (isEdit.value) {
            await store.updateProfile(form.value.id, form.value);
        } else {
            await store.createProfile(form.value);
        }
        ElMessage.success(t("message.saveSuccess"));
        dialogVisible.value = false;
    } catch (e) {
        ElMessage.error(t("message.saveFailed"));
    }
}

function confirmDelete(profile) {
    ElMessageBox.confirm(t("message.confirmDelete"), "Warning", {
        type: "warning",
    })
        .then(() => store.deleteProfile(profile.id))
        .then(() => ElMessage.success(t("message.deleteSuccess")))
        .catch(() => {});
}

function copyEndpoint(path) {
    const url = `${window.location.origin}/api/${path}/v1/chat/completions`;
    navigator.clipboard.writeText(url);
    ElMessage.success(t("message.copySuccess"));
}

onMounted(() => {
    store.fetchProfiles();
    store.fetchModels();
    store.fetchRouteRules();
});
</script>

<style scoped>
.breadcrumb-wrapper {
    margin-bottom: 16px;
}

.profiles-page {
    max-width: 1400px;
}

.page-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 24px;
}

.page-header h2 {
    margin: 0;
    font-size: 20px;
    font-weight: 600;
    color: #1f2937;
}

.add-btn {
    display: flex;
    align-items: center;
    gap: 6px;
    font-weight: 500;
}

.profiles-grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(360px, 1fr));
    gap: 20px;
}

.profile-card {
    position: relative;
    display: flex;
    background: #fff;
    border-radius: 12px;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.08);
    overflow: hidden;
    transition: all 0.3s ease;
}

.profile-card:hover {
    box-shadow: 0 4px 12px rgba(0, 0, 0, 0.12);
    transform: translateY(-2px);
}

.card-accent {
    width: 6px;
    background: var(--accent-color);
    flex-shrink: 0;
}

.card-content {
    flex: 1;
    padding: 20px;
}

.card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 16px;
}

.profile-name {
    display: flex;
    align-items: center;
    gap: 8px;
    font-weight: 600;
    font-size: 16px;
    color: #1f2937;
}

.status-tag {
    font-size: 12px;
}

.profile-details {
    display: flex;
    flex-direction: column;
    gap: 12px;
    margin-bottom: 16px;
}

.detail-item {
    display: flex;
    align-items: center;
    gap: 12px;
}

.detail-label {
    width: 70px;
    font-size: 13px;
    color: #6b7280;
    flex-shrink: 0;
}

.detail-value {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 14px;
    color: #374151;
    font-weight: 500;
}

.endpoint-wrapper {
    display: flex;
    align-items: center;
    gap: 6px;
}

.endpoint {
    font-size: 13px;
    color: #4f46e5;
    background: #eef2ff;
    padding: 2px 8px;
    border-radius: 4px;
    font-family: "Monaco", "Menlo", monospace;
}

.copy-btn {
    padding: 4px;
    color: #6b7280;
}

.copy-btn:hover {
    color: #4f46e5;
}

.priority-rate {
    --el-rate-void-color: #e5e7eb;
    --el-rate-fill-color: #f59e0b;
}

.card-actions {
    display: flex;
    justify-content: flex-end;
    gap: 12px;
    padding-top: 12px;
    border-top: 1px solid #f3f4f6;
}

.form-tip {
    font-size: 12px;
    color: #909399;
    margin-top: 4px;
}

.api-endpoints {
    display: flex;
    flex-direction: column;
    gap: 4px;
}

.endpoint-item {
    display: flex;
    align-items: center;
}

.endpoint-small {
    font-size: 11px;
    background: #f3f4f6;
    padding: 2px 6px;
    border-radius: 3px;
    color: #606266;
}

/* Responsive */
@media (max-width: 768px) {
    .profiles-grid {
        grid-template-columns: 1fr;
    }
}
</style>
