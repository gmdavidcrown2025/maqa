package maqa

import (
	"fmt"
	"math"
)

// Config 定义 MAQA 排序公式涉及的全部控制参数。
type Config struct {
	WFit     float64
	WQ       float64
	WB       float64
	WSrv     float64
	AlphaQ   float64
	EpsilonQ float64
	DeltaB   float64
	EpsilonB float64
	BMax     float64
	Beta     float64
	Eta      float64
	NoiseEps float64
}

// DefaultConfig 返回与 Python 版本保持一致的默认参数。
func DefaultConfig() Config {
	return Config{
		WFit:     0.50,
		WQ:       0.25,
		WB:       0.15,
		WSrv:     0.10,
		AlphaQ:   2.0,
		EpsilonQ: 0.2,
		DeltaB:   0.5,
		EpsilonB: 0.5,
		BMax:     2.0,
		Beta:     0.8,
		Eta:      2.0,
		NoiseEps: 0.001,
	}
}

// Validate 校验配置是否合法，当前重点约束 4 个权重之和必须为 1。
func (c Config) Validate() error {
	weightSum := c.WFit + c.WQ + c.WB + c.WSrv
	if math.Abs(weightSum-1.0) > 1e-9 {
		return fmt.Errorf("w_fit + w_q + w_b + w_srv must equal 1.0")
	}
	return nil
}
