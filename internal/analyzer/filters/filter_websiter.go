package filters

import (
	"pump_auto/internal/model"
	"strings"
)

type WebsiteFilter struct{}

func NewWebsiteFilter() *WebsiteFilter {
	return &WebsiteFilter{}
}

func (f *WebsiteFilter) Name() string {
	return "WebsiteFilter"
}
func (f *WebsiteFilter) Type() FilterType {
	return WebsiteExist
}

// HasWebsite 检查代币元数据是否包含有效的网站链接（非空且不是默认值）
func (f *WebsiteFilter) Filter(metadata *model.TokenMetadata) bool {
	// 检查传入的元数据
	if metadata == nil {
		return false
	}

	// 检查Website字段是否存在且不为空
	if metadata.Website == "" {
		return false
	}

	// 排除一些无效URL或默认值
	// 例如 javascript:void(0), #, about:blank 等
	invalidPatterns := []string{
		"javascript:",
		"#",
		"about:blank",
		"mailto:",
		"tel:",
		"file:",
		"undefined",
		"null",
	}

	website := strings.ToLower(metadata.Website)
	for _, pattern := range invalidPatterns {
		if strings.Contains(website, pattern) {
			return false
		}
	}

	// 检查是否包含有效域名部分
	return strings.Contains(website, ".")
}
