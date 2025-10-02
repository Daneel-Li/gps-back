package utils

import (
	"fmt"
	"log/slog"
	"math/rand"
	"strconv"
	"sync"
)

type IDGenerator struct {
	counter int64
	mutex   sync.Mutex
}

func (g *IDGenerator) Next() int64 {
	g.mutex.Lock()
	defer g.mutex.Unlock()
	g.counter++
	g.counter %= 65535
	return g.counter
}

func ParseFloatWithDefault(value string, defaultValue float64) float64 {
	if value == "" {
		return defaultValue
	}
	result, err := strconv.ParseFloat(value, 64)
	if err != nil {
		slog.Warn(fmt.Sprintf("ParseFloatWithDefault error: %v", err))
		return defaultValue
	}
	return result
}

func RandomString(length int) string {
	const letters = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"
	ret := make([]byte, length)
	for i := 0; i < length; i++ {
		num := rand.Int() % len(letters)
		ret[i] = letters[num]
	}
	return string(ret)
}

func RandomInt(min, max int) int {
	return min + int(rand.Int63n(int64(max-min)))
}

func Deref[T any](p *T, defaultValue T) T {
	if p != nil {
		return *p
	}
	return defaultValue
}
