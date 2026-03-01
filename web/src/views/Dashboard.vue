<template>
    <div class="dashboard">
        <!-- 面包屑导航 -->
        <div class="breadcrumb-wrapper">
            <el-breadcrumb separator="/">
                <el-breadcrumb-item :to="{ path: '/' }">Home</el-breadcrumb-item>
                <el-breadcrumb-item>{{ $t("dashboard.title") }}</el-breadcrumb-item>
            </el-breadcrumb>
        </div>

        <div class="page-header">
            <h2 class="page-title">{{ $t("dashboard.title") }}</h2>
        </div>

        <!-- 统计卡片 -->
        <el-row :gutter="20" class="stats-row">
            <el-col
                :xs="24"
                :sm="12"
                :md="8"
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
                            <el-icon :size="24"
                                ><component :is="stat.icon"
                            /></el-icon>
                        </div>
                        <div class="stat-info">
                            <div class="stat-value">{{ stat.value }}</div>
                            <div class="stat-label">{{ stat.label }}</div>
                        </div>
                    </div>
                </el-card>
            </el-col>
        </el-row>

        <!-- 图表区域 -->
        <el-row :gutter="20" class="charts-row">
            <el-col :xs="24" :lg="16">
                <el-card class="chart-card">
                    <template #header>
                        <div class="card-header">
                            <span>{{ $t("dashboard.requestTrend") }}</span>
                            <el-radio-group v-model="timeRange" size="small">
                                <el-radio-button value="24h"
                                    >24H</el-radio-button
                                >
                                <el-radio-button value="7d">7D</el-radio-button>
                                <el-radio-button value="30d"
                                    >30D</el-radio-button
                                >
                            </el-radio-group>
                        </div>
                    </template>
                    <div class="chart-container">
                        <v-chart v-if="hasTrendData" :option="trendChartOption" autoresize />
                        <el-empty v-else :description="$t('dashboard.noTrendData')" />
                    </div>
                </el-card>
            </el-col>

            <el-col :xs="24" :lg="8">
                <el-card class="chart-card">
                    <template #header>
                        <div class="card-header">
                            <span>{{ $t("dashboard.topModels") }}</span>
                        </div>
                    </template>
                    <div class="chart-container">
                        <v-chart v-if="hasTopModelsData" :option="pieChartOption" autoresize />
                        <el-empty v-else :description="$t('dashboard.noTopModelsData')" />
                    </div>
                </el-card>
            </el-col>
        </el-row>

        <!-- 最近日志 -->
        <el-row class="logs-row">
            <el-col :span="24">
                <el-card>
                    <template #header>
                        <div class="card-header">
                            <span>{{ $t("dashboard.recentLogs") }}</span>
                            <el-button link type="primary" @click="goToLogs">
                                {{ $t("common.more") }}
                            </el-button>
                        </div>
                    </template>
                    <el-table :data="recentLogs" stripe size="small">
                        <el-table-column
                            prop="request_id"
                            label="ID"
                            width="200"
                            show-overflow-tooltip
                        />
                        <el-table-column
                            prop="model"
                            :label="$t('logs.model')"
                            width="150"
                        />
                        <el-table-column
                            prop="provider_id"
                            :label="$t('logs.provider')"
                            width="120"
                        />
                        <el-table-column :label="$t('logs.status')" width="100">
                            <template #default="{ row }">
                                <el-tag
                                    :type="
                                        row.status === 'success'
                                            ? 'success'
                                            : 'danger'
                                    "
                                    size="small"
                                >
                                    {{
                                        row.status === "success" ? $t("logs.success") : $t("logs.error")
                                    }}
                                </el-tag>
                            </template>
                        </el-table-column>
                        <el-table-column
                            prop="latency"
                            :label="$t('logs.latency')"
                            width="100"
                        >
                            <template #default="{ row }">
                                {{ row.latency }}ms
                            </template>
                        </el-table-column>
                        <el-table-column
                            prop="created_at"
                            :label="$t('logs.timestamp')"
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
import { ref, computed, onMounted } from "vue";
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
} from "echarts/components";
import VChart from "vue-echarts";

import { useAppStore } from "@/stores/app";

use([
    CanvasRenderer,
    LineChart,
    PieChart,
    TitleComponent,
    TooltipComponent,
    LegendComponent,
    GridComponent,
]);

const { t } = useI18n();

const router = useRouter();
const store = useAppStore();

const timeRange = ref("24h");

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
        },
        {
            key: "lastHour",
            value: (s.requests_last_hour || 0).toLocaleString(),
            label: t("dashboard.lastHourRequests"),
            icon: "TrendCharts",
            color: "#67C23A",
        },
        {
            key: "success",
            value: (s.success_rate || 0) + "%",
            label: t("dashboard.successRate"),
            icon: "CircleCheck",
            color: "#E6A23C",
        },
        {
            key: "latency",
            value: (s.avg_latency_ms || 0) + "ms",
            label: t("dashboard.avgLatency"),
            icon: "Timer",
            color: "#F56C6C",
        },
    ];
});

// 趋势图 - 使用真实数据
const trendChartOption = computed(() => {
    const s = store.stats;
    const trend = store.trendStats;
    const hasData = s.total_requests_24h > 0;
    
    // 使用后端返回的趋势数据
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
            containLabel: true,
        },
        xAxis: {
            type: "category",
            boundaryGap: false,
            data: hours,
            axisLine: {
                lineStyle: {
                    color: "#e5e7eb",
                },
            },
            axisLabel: {
                color: "#6b7280",
            },
        },
        yAxis: {
            type: "value",
            minInterval: 1,
            axisLine: {
                show: false,
            },
            axisTick: {
                show: false,
            },
            splitLine: {
                lineStyle: {
                    color: "#f3f4f6",
                },
            },
            axisLabel: {
                color: "#6b7280",
            },
        },
        series: [
            {
                name: t('dashboard.requests'),
                type: "line",
                smooth: true,
                symbol: "circle",
                symbolSize: 6,
                data: data,
                areaStyle: {
                    color: {
                        type: "linear",
                        x: 0,
                        y: 0,
                        x2: 0,
                        y2: 1,
                        colorStops: [
                            { offset: 0, color: "rgba(64, 158, 255, 0.3)" },
                            { offset: 1, color: "rgba(64, 158, 255, 0.05)" },
                        ],
                    },
                },
                itemStyle: {
                    color: "#409EFF",
                },
                lineStyle: {
                    width: 2,
                },
            },
        ],
    };
});

// 饼图配置 - 使用真实数据
const pieChartOption = computed(() => {
    const s = store.stats;
    const topModels = s.top_models || {};
    const hasData = Object.keys(topModels).length > 0;
    
    // 如果有真实数据，使用真实数据；否则显示空状态
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
            right: "5%",
            top: "center",
            textStyle: {
                color: "#6b7280",
            },
            data: hasData ? Object.keys(topModels) : [],
        },
        series: [
            {
                type: "pie",
                radius: ["45%", "70%"],
                center: ["35%", "50%"],
                avoidLabelOverlap: false,
                itemStyle: {
                    borderRadius: 8,
                    borderColor: "#fff",
                    borderWidth: 2,
                },
                label: {
                    show: false,
                },
                emphasis: {
                    label: {
                        show: true,
                        fontSize: 14,
                        fontWeight: "bold",
                    },
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
    return store.logs.slice(0, 5) || []
})

function formatTime(time) {
    return new Date(time).toLocaleString();
}

function goToLogs() {
    router.push("/logs");
}

onMounted(() => {
    // 加载真实数据
    store.fetchStats();
    store.fetchTrendStats();
});
</script>

<style scoped>
.dashboard {
    max-width: 100%;
}

.breadcrumb-wrapper {
    margin-bottom: 16px;
}

.page-header {
    margin-bottom: 24px;
}

.page-title {
    margin: 0;
    font-size: 24px;
    font-weight: 600;
    color: #1f2937;
}

.stats-row {
    margin-bottom: 20px;
}

.stat-card {
    margin-bottom: 20px;
}

.stat-content {
    display: flex;
    align-items: center;
}

.stat-icon {
    width: 56px;
    height: 56px;
    border-radius: 12px;
    display: flex;
    align-items: center;
    justify-content: center;
    color: #fff;
    margin-right: 16px;
}

.stat-value {
    font-size: 28px;
    font-weight: 700;
    color: #1f2937;
}

.stat-label {
    font-size: 14px;
    color: #6b7280;
    margin-top: 4px;
}

.charts-row {
    margin-bottom: 20px;
}

.chart-card {
    margin-bottom: 20px;
}

.card-header {
    display: flex;
    justify-content: space-between;
    align-items: center;
}

.card-header span {
    font-size: 16px;
    font-weight: 600;
    color: #1f2937;
}

.chart-container {
    height: 300px;
}

.chart {
    width: 100%;
    height: 100%;
}

.logs-row {
    margin-bottom: 20px;
}
</style>
