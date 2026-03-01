package model

// RouteStrategy 定义路由策略
type RouteStrategy string

const (
	RouteStrategyPriority     RouteStrategy = "priority"       // 按优先级路由
	RouteStrategyWeighted     RouteStrategy = "weighted"       // 加权轮询
	RouteStrategyLeastLatency RouteStrategy = "least_latency"  // 最低延迟
	RouteStrategyHighestHealth RouteStrategy = "highest_health" // 最高健康度
	RouteStrategyLowestCost   RouteStrategy = "lowest_cost"    // 最低成本
	RouteStrategyAuto         RouteStrategy = "auto"           // 自动选择
)

// Note: RouteRule has been removed as part of the v3 destructive refactoring.
// Routing now uses Profile.ModelIDs and direct model lookup.
