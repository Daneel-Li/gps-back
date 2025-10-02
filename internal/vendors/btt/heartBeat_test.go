package btt

import (
	"flag"
	"testing"

	"github.com/stretchr/testify/assert"
)

var testVoltage = flag.Int("vol", 3800, "测试电压值(mV)")

func TestVol2Percent_WithVoltageInDefinedRange(t *testing.T) {
	result := Vol2Percent(*testVoltage)

	t.Logf("电压: %dmV -> 电量百分比: %d%%", *testVoltage, result)

	assert.NotEqual(t, 0, result)
	assert.GreaterOrEqual(t, result, 0)
	assert.LessOrEqual(t, result, 100)
}
