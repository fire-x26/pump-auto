package filters

import (
	"pump_auto/internal/model"
	"strings"
)

type TwitterFilter struct{}

func NewTwitterFilter() *TwitterFilter {
	return &TwitterFilter{}
}

func (f *TwitterFilter) Name() string {
	return "twitterFilter"
}
func (f *TwitterFilter) Type() FilterType {
	return TwitterExist
}

// HasTwitter 检查代币元数据是否包含Twitter信息
func (f *TwitterFilter) Filter(metadata *model.TokenMetadata) bool {
	// 检查传入的元数据
	if metadata == nil {
		return false
	}

	// 检查Twitter字段
	if metadata.Twitter != "" {
		return true
	}

	// 检查website字段是否包含Twitter链接
	if metadata.Website != "" && (strings.Contains(strings.ToLower(metadata.Website), "twitter") ||
		strings.Contains(strings.ToLower(metadata.Website), "x.com")) {
		return true
	}

	return false
}
