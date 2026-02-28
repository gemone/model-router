<template>
    <div class="profiles-page">
        <!-- 面包屑导航 -->
        <div class="breadcrumb-wrapper">
            <el-breadcrumb separator="/">
                <el-breadcrumb-item :to="{ path: '/' }">Home</el-breadcrumb-item>
                <el-breadcrumb-item>{{ $t("profile.title") }}</el-breadcrumb-item>
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
                :style="{ '--accent-color': accentColors[index % accentColors.length] }"
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
                            <span class="detail-label">{{ $t("profile.path") }}</span>
                            <div class="endpoint-wrapper">
                                <code class="endpoint">/api/{{ profile.path }}</code>
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
                            <span class="detail-label">{{ $t("profile.modelsCount") }}</span>
                            <span class="detail-value">
                                <el-icon><Cpu /></el-icon>
                                {{ profile.models?.length || 0 }}
                            </span>
                        </div>
                        <div class="detail-item">
                            <span class="detail-label">{{ $t("common.priority") }}</span>
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
    '#3B82F6', // blue
    '#10B981', // green
    '#F59E0B', // amber
    '#EF4444', // red
    '#8B5CF6', // violet
    '#EC4899', // pink
    '#06B6D4', // cyan
    '#F97316', // orange
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

onMounted(() => store.fetchProfiles());
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
    font-family: 'Monaco', 'Menlo', monospace;
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

/* Responsive */
@media (max-width: 768px) {
    .profiles-grid {
        grid-template-columns: 1fr;
    }
}
</style>
