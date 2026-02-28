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
                                <el-radio-button label="24h"
                                    >24H</el-radio-button
                                >
                                <el-radio-button label="7d">7D</el-radio-button>
                                <el-radio-button label="30d"
                                    >30D</el-radio-button
                                >
                            </el-radio-group>
                        </div>
                    </template>
                    <div class="chart-container">
                        <v-chart :option="trendChartOption" autoresize />
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
                        <v-chart :option="pieChartOption" autoresize />
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
            key: "today",
            value: (s.requests_last_hour || 0).toLocaleString(),
            label: t("dashboard.todayRequests"),
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
    const topModels = s.top_models || {};
    const labels = Object.keys(topModels);
    const data = Object.values(topModels);

    return {
        tooltip: {
            trigger: "axis",
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
            data: ["00:00", "04:00", "08:00", "12:00", "16:00", "20:00", "23:59"],
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
                name: "Requests",
                type: "line",
                smooth: true,
                symbol: "circle",
                symbolSize: 6,
                data: [120, 82, 191, 334, 290, 330, 310],
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

// 饼图配置
const pieChartOption = computed(() => ({
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
            data: [
                { value: 335, name: "GPT-4", itemStyle: { color: "#3B82F6" } },
                { value: 310, name: "GPT-3.5", itemStyle: { color: "#10B981" } },
                { value: 234, name: "Claude", itemStyle: { color: "#8B5CF6" } },
                { value: 135, name: "Gemini", itemStyle: { color: "#F59E0B" } },
                { value: 148, name: "Others", itemStyle: { color: "#6B7280" } },
            ],
        },
    ],
}));

// 最近日志
const recentLogs = ref([
    {
        request_id: "req-001",
        model: "gpt-4",
        provider_id: "openai",
        status: "success",
        latency: 234,
        created_at: new Date().toISOString(),
    },
    {
        request_id: "req-002",
        model: "claude-3",
        provider_id: "anthropic",
        status: "success",
        latency: 189,
        created_at: new Date().toISOString(),
    },
    {
        request_id: "req-003",
        model: "deepseek-chat",
        provider_id: "deepseek",
        status: "error",
        latency: 5000,
        created_at: new Date().toISOString(),
    },
    {
        request_id: "req-004",
        model: "gpt-4o",
        provider_id: "openai",
        status: "success",
        latency: 312,
        created_at: new Date().toISOString(),
    },
    {
        request_id: "req-005",
        model: "claude-3-opus",
        provider_id: "anthropic",
        status: "success",
        latency: 456,
        created_at: new Date().toISOString(),
    },
]);

function formatTime(time) {
    return new Date(time).toLocaleString();
}

function goToLogs() {
    router.push("/logs");
}

onMounted(() => {
    // 加载真实数据
    store.fetchStats();
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
