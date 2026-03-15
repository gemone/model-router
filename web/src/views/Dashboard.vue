<template>
    <div class="dashboard">
        <!-- 页面标题 -->
        <div class="page-header">
            <div class="header-left">
                <el-tag v-if="isLive" type="success" size="small" class="live-tag">
                    <el-icon><VideoPlay /></el-icon>
                    {{ $t("dashboard.realtimeStatus") }}
                </el-tag>
            </div>
            <div class="header-actions">
                <el-button-group>
                    <el-button
                        :type="autoRefresh ? 'primary' : ''"
                        @click="toggleAutoRefresh"
                        :icon="autoRefresh ? Timer : Refresh"
                    >
                        {{ autoRefresh ? $t('dashboard.autoRefreshOn') : $t('dashboard.autoRefreshOff') }}
                    </el-button>
                    <el-dropdown @command="setRefreshInterval">
                        <el-button>
                            {{ refreshInterval / 1000 }}s
                            <el-icon class="el-icon--right"><ArrowDown /></el-icon>
                        </el-button>
                        <template #dropdown>
                            <el-dropdown-menu>
                                <el-dropdown-item :command="5000">5s</el-dropdown-item>
                                <el-dropdown-item :command="10000">10s</el-dropdown-item>
                                <el-dropdown-item :command="30000">30s</el-dropdown-item>
                                <el-dropdown-item :command="60000">60s</el-dropdown-item>
                            </el-dropdown-menu>
                        </template>
                    </el-dropdown>
                </el-button-group>
                <el-button @click="exportData" :icon="Download">
                    {{ $t('dashboard.exportData') }}
                </el-button>
            </div>
        </div>

        <!-- 统计卡片 -->
        <el-row :gutter="16" class="stats-row">
            <el-col
                :xs="12"
                :sm="12"
                :md="6"
                :lg="6"
                v-for="stat in statsCards"
                :key="stat.key"
            >
                <el-card class="stat-card" shadow="hover">
                    <div class="stat-content">
                        <div
                            class="stat-icon"
                            :style="{ backgroundColor: stat.color }"
                        >
                            <el-icon :size="20"><component :is="stat.icon" /></el-icon>
                        </div>
                        <div class="stat-info">
                            <div class="stat-value">
                                {{ stat.value }}
                                <el-icon v-if="stat.trend === 'up'" class="trend-icon up"><CaretTop /></el-icon>
                                <el-icon v-else-if="stat.trend === 'down'" class="trend-icon down"><CaretBottom /></el-icon>
                            </div>
                            <div class="stat-label">{{ stat.label }}</div>
                        </div>
                    </div>
                </el-card>
            </el-col>
        </el-row>

        <!-- 图表区域 -->
        <el-row :gutter="16" class="charts-row">
            <el-col :xs="24" :lg="16">
                <el-card class="chart-card">
                    <template #header>
                        <div class="card-header">
                            <span class="card-title">{{ $t("dashboard.requestTrend") }}</span>
                            <el-radio-group v-model="timeRange" size="small" @change="fetchTrendData">
                                <el-radio-button value="1h">1H</el-radio-button>
                                <el-radio-button value="6h">6H</el-radio-button>
                                <el-radio-button value="24h">24H</el-radio-button>
                                <el-radio-button value="7d">7D</el-radio-button>
                                <el-radio-button value="30d">30D</el-radio-button>
                            </el-radio-group>
                        </div>
                    </template>
                    <div class="chart-container">
                        <v-chart v-if="hasTrendData" :option="trendChartOption" autoresize />
                        <el-empty v-else :description="$t('dashboard.noTrendData')" :image-size="100" />
                    </div>
                </el-card>
            </el-col>

            <el-col :xs="24" :lg="8">
                <el-card class="chart-card">
                    <template #header>
                        <div class="card-header">
                            <span class="card-title">{{ $t("dashboard.topModels") }}</span>
                            <el-button link size="small" @click="goToStats">
                                {{ $t("common.more") }} →
                            </el-button>
                        </div>
                    </template>
                    <div class="chart-container pie-container">
                        <v-chart v-if="hasTopModelsData" :option="pieChartOption" autoresize />
                        <el-empty v-else :description="$t('dashboard.noTopModelsData')" :image-size="80" />
                    </div>
                </el-card>
            </el-col>
        </el-row>

        <!-- Provider 健康状态 -->
        <el-row class="health-row">
            <el-col :span="24">
                <el-card class="health-card">
                    <template #header>
                        <div class="card-header">
                            <span class="card-title">{{ $t("dashboard.providerHealth") }}</span>
                            <el-button link size="small" @click="checkAllHealth">
                                <el-icon><Refresh /></el-icon>
                                {{ $t("dashboard.checkAll") }}
                            </el-button>
                        </div>
                    </template>
                    <div class="health-list">
                        <div
                            v-for="provider in healthProviders"
                            :key="provider.id"
                            class="health-item"
                            :class="provider.healthStatus"
                        >
                            <div class="health-info">
                                <el-icon class="health-icon">
                                    <component :is="getHealthIcon(provider.healthStatus)" />
                                </el-icon>
                                <div class="health-details">
                                    <div class="health-name">{{ provider.name }}</div>
                                    <div class="health-type">{{ provider.type }}</div>
                                </div>
                            </div>
                            <div class="health-stats">
                                <el-tag size="small" :type="getHealthTagType(provider.healthStatus)">
                                    {{ $t(`provider.health${capitalize(provider.healthStatus)}`) }}
                                </el-tag>
                                <span v-if="provider.latency" class="health-latency">
                                    {{ provider.latency }}ms
                                </span>
                                <span v-if="provider.lastCheck" class="health-time">
                                    {{ formatTime(provider.lastCheck) }}
                                </span>
                            </div>
                        </div>
                    </div>
                </el-card>
            </el-col>
        </el-row>

        <!-- 最近日志 -->
        <el-row class="logs-row">
            <el-col :span="24">
                <el-card class="logs-card">
                    <template #header>
                        <div class="card-header">
                            <span class="card-title">{{ $t("dashboard.recentLogs") }}</span>
                            <el-button link type="primary" @click="goToLogs">
                                {{ $t("common.more") }} →
                            </el-button>
                        </div>
                    </template>
                    <el-table :data="recentLogs" stripe size="small" class="logs-table">
                        <el-table-column
                            prop="request_id"
                            label="ID"
                            width="180"
                            show-overflow-tooltip
                        />
                        <el-table-column
                            prop="model"
                            :label="$t('logs.model')"
                            width="140"
                        />
                        <el-table-column
                            prop="provider_id"
                            :label="$t('logs.provider')"
                            width="100"
                        />
                        <el-table-column :label="$t('logs.status')" width="80" align="center">
                            <template #default="{ row }">
                                <el-tag
                                    :type="row.status === 'success' ? 'success' : 'danger'"
                                    size="small"
                                >
                                    {{ row.status === "success" ? $t("logs.success") : $t("logs.error") }}
                                </el-tag>
                            </template>
                        </el-table-column>
                        <el-table-column
                            prop="latency"
                            :label="$t('logs.latency')"
                            width="90"
                            align="right"
                        >
                            <template #default="{ row }">
                                {{ row.latency }}ms
                            </template>
                        </el-table-column>
                        <el-table-column
                            prop="created_at"
                            :label="$t('logs.timestamp')"
                            min-width="160"
                        >
                            <template #default="{ row }">
                                {{ formatTime(row.created_at) }}
                            </template>
                        </el-table-column>
                    </el-table>
                </el-card>
            </el-col>
        </el-row>
    </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from "vue";
import { useRouter } from "vue-router";
import { useI18n } from "vue-i18n";
import { use } from "echarts/core";
import { CanvasRenderer } from "echarts/renderers";
import { LineChart, PieChart } from "echarts/charts";
import {
    TitleComponent,
    TooltipComponent,
    LegendComponent,
    GridComponent,
    DataZoomComponent,
} from "echarts/components";
import VChart from "vue-echarts";
import { ElMessage } from "element-plus";

import { useAppStore } from "@/stores/app";
import { Download, Refresh } from "@element-plus/icons-vue";

use([
    CanvasRenderer,
    LineChart,
    PieChart,
    TitleComponent,
    TooltipComponent,
    LegendComponent,
    GridComponent,
    DataZoomComponent,
]);

const { t } = useI18n();
const router = useRouter();
const store = useAppStore();

const timeRange = ref("24h");
const autoRefresh = ref(false);
const refreshInterval = ref(10000);
const isLive = ref(false);
let refreshTimer = null;

// Provider 健康状态
const healthProviders = ref([]);

// 判断是否有趋势数据
const hasTrendData = computed(() => {
    const s = store.stats;
    return s.total_requests_24h > 0;
});

// 判断是否热门模型数据
const hasTopModelsData = computed(() => {
    const s = store.stats;
    const topModels = s.top_models || {};
    return Object.keys(topModels).length > 0;
});

// 从 store 获取真实统计数据
const statsCards = computed(() => {
    const s = store.stats;
    return [
        {
            key: "total",
            value: (s.total_requests_24h || 0).toLocaleString(),
            label: t("dashboard.totalRequests"),
            icon: "DataLine",
            color: "#409EFF",
            trend: s.requests_trend >= 0 ? 'up' : 'down',
        },
        {
            key: "lastHour",
            value: (s.requests_last_hour || 0).toLocaleString(),
            label: t("dashboard.lastHourRequests"),
            icon: "TrendCharts",
            color: "#67C23A",
            trend: null,
        },
        {
            key: "success",
            value: (s.success_rate || 0).toFixed(1) + "%",
            label: t("dashboard.successRate"),
            icon: "CircleCheck",
            color: "#E6A23C",
            trend: null,
        },
        {
            key: "latency",
            value: (s.avg_latency_ms || 0).toFixed(0) + "ms",
            label: t("dashboard.avgLatency"),
            icon: "Timer",
            color: "#F56C6C",
            trend: s.latency_trend <= 0 ? 'up' : 'down', // 延迟降低是好事
        },
    ];
});

// 趋势图 - 使用真实数据
const trendChartOption = computed(() => {
    const s = store.stats;
    const trend = store.trendStats;
    const hasData = s.total_requests_24h > 0;

    const hours = trend.hours || [];
    const data = trend.requests || [];

    return {
        tooltip: {
            trigger: "axis",
            formatter: hasData ? undefined : () => t('dashboard.noData'),
        },
        grid: {
            left: "3%",
            right: "4%",
            bottom: "3%",
            top: "10%",
            containLabel: true,
        },
        dataZoom: [
            {
                type: "inside",
                start: 0,
                end: 100,
            },
            {
                start: 0,
                end: 100,
                height: 20,
                bottom: 10,
            },
        ],
        xAxis: {
            type: "category",
            boundaryGap: false,
            data: hours,
            axisLine: { lineStyle: { color: "#e5e7eb" } },
            axisLabel: { color: "#6b7280", fontSize: 11 },
        },
        yAxis: {
            type: "value",
            minInterval: 1,
            axisLine: { show: false },
            axisTick: { show: false },
            splitLine: { lineStyle: { color: "#f3f4f6" } },
            axisLabel: { color: "#6b7280", fontSize: 11 },
        },
        series: [
            {
                name: t('dashboard.requests'),
                type: "line",
                smooth: true,
                symbol: "circle",
                symbolSize: 5,
                data: data,
                areaStyle: {
                    color: {
                        type: "linear",
                        x: 0, y: 0, x2: 0, y2: 1,
                        colorStops: [
                            { offset: 0, color: "rgba(64, 158, 255, 0.25)" },
                            { offset: 1, color: "rgba(64, 158, 255, 0.02)" },
                        ],
                    },
                },
                itemStyle: { color: "#409EFF" },
                lineStyle: { width: 2 },
            },
        ],
    };
});

// 饼图配置 - 使用真实数据
const pieChartOption = computed(() => {
    const s = store.stats;
    const topModels = s.top_models || {};
    const hasData = Object.keys(topModels).length > 0;

    const data = hasData
        ? Object.entries(topModels).map(([name, value], index) => ({
            value,
            name,
            itemStyle: { color: getColorByIndex(index) },
        }))
        : [];

    return {
        tooltip: {
            trigger: "item",
            formatter: "{b}: {c} ({d}%)",
        },
        legend: {
            orient: "vertical",
            right: 0,
            top: "center",
            itemWidth: 12,
            itemHeight: 12,
            textStyle: { color: "#6b7280", fontSize: 12 },
            data: hasData ? Object.keys(topModels) : [],
        },
        series: [
            {
                type: "pie",
                radius: ["50%", "72%"],
                center: ["30%", "50%"],
                avoidLabelOverlap: false,
                itemStyle: {
                    borderRadius: 6,
                    borderColor: "#fff",
                    borderWidth: 2,
                },
                label: { show: false },
                emphasis: {
                    label: {
                        show: true,
                        fontSize: 13,
                        fontWeight: "bold",
                    },
                    scale: true,
                    scaleSize: 10,
                },
                data: data,
            },
        ],
    };
});

// 颜色列表
const colors = ["#3B82F6", "#10B981", "#8B5CF6", "#F59E0B", "#6B7280", "#EC4899", "#14B8A6", "#F97316"];
function getColorByIndex(index) {
    return colors[index % colors.length];
}

// 最近日志 - 从 store 获取真实数据
const recentLogs = computed(() => {
    return store.logs.slice(0, 5) || [];
});

function formatTime(time) {
    const date = new Date(time);
    const now = new Date();
    const diff = now - date;
    const seconds = Math.floor(diff / 1000);
    const minutes = Math.floor(seconds / 60);
    const hours = Math.floor(minutes / 60);
    const days = Math.floor(hours / 24);

    if (days > 0) return `${days}d ago`;
    if (hours > 0) return `${hours}h ago`;
    if (minutes > 0) return `${minutes}m ago`;
    return 'Just now';
}

function capitalize(str) {
    return str.charAt(0).toUpperCase() + str.slice(1);
}

function getHealthIcon(status) {
    switch (status) {
        case 'healthy': return 'CircleCheck';
        case 'unhealthy': return 'CircleClose';
        default: return 'Clock';
    }
}

function getHealthTagType(status) {
    switch (status) {
        case 'healthy': return 'success';
        case 'unhealthy': return 'danger';
        default: return 'info';
    }
}

// 自动刷新控制
function toggleAutoRefresh() {
    autoRefresh.value = !autoRefresh.value;
    isLive.value = autoRefresh.value;
    if (autoRefresh.value) {
        startAutoRefresh();
    } else {
        stopAutoRefresh();
    }
}

function setRefreshInterval(interval) {
    refreshInterval.value = interval;
    if (autoRefresh.value) {
        stopAutoRefresh();
        startAutoRefresh();
    }
}

function startAutoRefresh() {
    refreshTimer = setInterval(() => {
        refreshData();
    }, refreshInterval.value);
}

function stopAutoRefresh() {
    if (refreshTimer) {
        clearInterval(refreshTimer);
        refreshTimer = null;
    }
}

async function refreshData() {
    await Promise.all([
        store.fetchStats(),
        store.fetchTrendStats(),
        store.fetchLogs(),
    ]);
    // 更新健康状态
    await updateHealthStatus();
}

async function fetchTrendData() {
    await store.fetchTrendStats();
}

async function checkAllHealth() {
    ElMessage.info(t('dashboard.checkingHealth'));
    await Promise.all(
        store.providers.map(p => testProviderHealth(p))
    );
}

async function testProviderHealth(provider) {
    const startTime = Date.now();
    try {
        const result = await store.testProvider(provider.id);
        const latency = Date.now() - startTime;
        const providerIndex = healthProviders.value.findIndex(p => p.id === provider.id);
        if (providerIndex >= 0) {
            healthProviders.value[providerIndex] = {
                ...healthProviders.value[providerIndex],
                healthStatus: result.success ? 'healthy' : 'unhealthy',
                latency,
                lastCheck: new Date().toISOString(),
            };
        }
    } catch (e) {
        const providerIndex = healthProviders.value.findIndex(p => p.id === provider.id);
        if (providerIndex >= 0) {
            healthProviders.value[providerIndex] = {
                ...healthProviders.value[providerIndex],
                healthStatus: 'unhealthy',
                lastCheck: new Date().toISOString(),
            };
        }
    }
}

async function updateHealthStatus() {
    // 从 providers 初始化健康状态列表
    if (healthProviders.value.length === 0) {
        healthProviders.value = store.providers.map(p => ({
            ...p,
            healthStatus: 'unknown',
        }));
    }
}

function exportData() {
    const data = {
        stats: store.stats,
        trend: store.trendStats,
        timestamp: new Date().toISOString(),
    };

    const blob = new Blob([JSON.stringify(data, null, 2)], { type: 'application/json' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `dashboard-stats-${new Date().toISOString().slice(0, 10)}.json`;
    a.click();
    URL.revokeObjectURL(url);

    ElMessage.success(t('message.saveSuccess'));
}

function goToLogs() {
    router.push("/logs");
}

function goToStats() {
    router.push("/stats");
}

onMounted(async () => {
    await refreshData();
    await updateHealthStatus();
});

onUnmounted(() => {
    stopAutoRefresh();
});
</script>

<style scoped>
.dashboard {
    max-width: 1400px;
    margin: 0 auto;
    padding: 0 16px;
}

.page-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
    margin-bottom: 20px;
    flex-wrap: wrap;
    gap: 16px;
}

.header-left {
    display: flex;
    align-items: center;
    gap: 12px;
}

.page-title {
    margin: 0;
    font-size: 22px;
    font-weight: 600;
    color: #1f2937;
}

.live-tag {
    display: flex;
    align-items: center;
    gap: 4px;
}

.header-actions {
    display: flex;
    gap: 12px;
}

.stats-row {
    margin-bottom: 16px;
}

.stat-card {
    margin-bottom: 0;
}

.stat-card :deep(.el-card__body) {
    padding: 16px;
}

.stat-content {
    display: flex;
    align-items: center;
}

.stat-icon {
    width: 48px;
    height: 48px;
    border-radius: 10px;
    display: flex;
    align-items: center;
    justify-content: center;
    color: #fff;
    margin-right: 12px;
    flex-shrink: 0;
}

.stat-value {
    font-size: 24px;
    font-weight: 700;
    color: #1f2937;
    line-height: 1.2;
    display: flex;
    align-items: center;
    gap: 4px;
}

.trend-icon {
    font-size: 16px;
}

.trend-icon.up {
    color: #67C23A;
}

.trend-icon.down {
    color: #F56C6C;
}

.stat-label {
    font-size: 13px;
    color: #6b7280;
    margin-top: 2px;
}

.charts-row {
    margin-bottom: 16px;
}

.chart-card {
    margin-bottom: 0;
}

.chart-card :deep(.el-card__header) {
    padding: 14px 16px;
    border-bottom: 1px solid #f0f0f0;
}

.chart-card :deep(.el-card__body) {
    padding: 16px;
}

.card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
}

.card-title {
    font-size: 15px;
    font-weight: 600;
    color: #1f2937;
}

.chart-container {
    height: 260px;
}

.pie-container {
    height: 220px;
}

.health-row {
    margin-bottom: 16px;
}

.health-card :deep(.el-card__header) {
    padding: 14px 16px;
    border-bottom: 1px solid #f0f0f0;
}

.health-card :deep(.el-card__body) {
    padding: 16px;
}

.health-list {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
    gap: 12px;
}

.health-item {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 12px 16px;
    border-radius: 8px;
    background: #f9fafb;
    border: 1px solid #e5e7eb;
    transition: all 0.3s;
}

.health-item:hover {
    box-shadow: 0 2px 8px rgba(0, 0, 0, 0.08);
}

.health-item.healthy {
    border-left: 3px solid #67C23A;
}

.health-item.unhealthy {
    border-left: 3px solid #F56C6C;
}

.health-item.unknown {
    border-left: 3px solid #909399;
}

.health-info {
    display: flex;
    align-items: center;
    gap: 12px;
}

.health-icon {
    font-size: 24px;
}

.health-icon.healthy {
    color: #67C23A;
}

.health-icon.unhealthy {
    color: #F56C6C;
}

.health-icon.unknown {
    color: #909399;
}

.health-details {
    display: flex;
    flex-direction: column;
}

.health-name {
    font-weight: 500;
    color: #1f2937;
}

.health-type {
    font-size: 12px;
    color: #6b7280;
}

.health-stats {
    display: flex;
    align-items: center;
    gap: 8px;
}

.health-latency {
    font-size: 13px;
    color: #6b7280;
}

.health-time {
    font-size: 11px;
    color: #9ca3af;
}

.logs-row {
    margin-bottom: 0;
}

.logs-card :deep(.el-card__header) {
    padding: 14px 16px;
    border-bottom: 1px solid #f0f0f0;
}

.logs-card :deep(.el-card__body) {
    padding: 0;
}

.logs-table {
    font-size: 13px;
}

.logs-table :deep(.el-table__header th) {
    background-color: #fafafa;
    font-weight: 600;
}

.logs-table :deep(.el-table__body-wrapper) {
    max-height: 280px;
    overflow-y: auto;
}

/* Responsive */
@media (max-width: 768px) {
    .page-header {
        flex-direction: column;
        align-items: flex-start;
    }

    .header-actions {
        width: 100%;
        flex-wrap: wrap;
    }

    .health-list {
        grid-template-columns: 1fr;
    }
}
</style>
