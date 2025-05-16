package analyzer

import (
	"pump_auto/internal/analyzer/filters"
	"pump_auto/internal/model"
	"time"
)

// FilterResult 过滤结果
type FilterResult struct {
	TokenAddress string
	TokenURI     string
	Metadata     *model.TokenMetadata
	IsFiltered   bool
	FilteredBy   []string
	AnalysisTime time.Time
}

// FilterProgress 过滤进度
type FilterProgress struct {
	FilterName    string
	TotalCount    int     // 总代币数量
	PassedCount   int     // 通过数量
	FilteredCount int     // 被过滤数量
	PassRate      float64 // 通过率
	LastUpdate    time.Time
}

// AnalysisCallback 分析回调函数类型
type AnalysisCallback func(results []FilterResult)

// fetchMetadata 从IPFS或其他来源获取代币元数据

// ProcessToken 处理代币并应用过滤器
func ProcessToken(tokenAddress string, tokenURI string, metadata *model.TokenMetadata, config *Config) *FilterResult {
	result := &FilterResult{
		TokenAddress: tokenAddress,
		TokenURI:     tokenURI,
		Metadata:     metadata,
		IsFiltered:   false,
		FilteredBy:   []string{},
		AnalysisTime: time.Now(),
	}

	// 检查元数据是否存在
	if metadata == nil {
		result.IsFiltered = true
		result.FilteredBy = append(result.FilteredBy, "NoMetadata")
		return result
	}

	// 应用所有过滤器
	for _, filter := range config.Filters {
		// 根据过滤器的Filter方法检查元数据是否通过过滤
		if !filter.Filter(metadata) {
			result.IsFiltered = true
			result.FilteredBy = append(result.FilteredBy, filter.Name())
		}
	}

	return result
}

// Config 过滤器配置
type Config struct {
	Filters []filters.Filter // 过滤器列表
	// 可以根据需要添加其他配置参数，例如：
	// ProcessTimeout time.Duration // 处理超时时间
	// RetryCount     int           // 重试次数
}

// DefaultConfig 返回默认过滤器配置
func DefaultConfig() *Config {
	return &Config{
		// 可以根据需要设置其他配置参数
		Filters: []filters.Filter{
			filters.NewTwitterFilter(), // Twitter过滤器
			filters.NewWebsiteFilter(), // 网站过滤器
			// 后续可以根据需要添加更多过滤器
		},
	}
}
