package filters

import "pump_auto/internal/model"

type Filter interface {
	// Filter 根据交易历史进行过滤，返回token是否通过过滤
	// token参数可以为nil，表示不使用token信息
	Filter(metadata *model.TokenMetadata) bool
	Name() string
	Type() FilterType
}

type FilterType int

const (
	TwitterExist FilterType = 1
	WebsiteExist FilterType = 2
)
