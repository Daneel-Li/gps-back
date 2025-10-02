package handlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/Daneel-Li/gps-back/internal/config"
	"github.com/Daneel-Li/gps-back/internal/services"
)

type Middleware func(http.HandlerFunc) http.HandlerFunc

func WithMidWare(finalHandler http.HandlerFunc, middlwares ...Middleware) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		f := finalHandler
		for _, m := range middlwares {
			f = m(f)
		}
		f(w, r)
	}
}

// Middleware 是一个中间件函数，用于校验请求
func ApiAuthCheck(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// 从请求中获取API密钥
		providedKey := r.Header.Get("appKey")

		// 校验API密钥
		if providedKey != config.GetConfig().MxmAPIKey {
			// 如果密钥不正确，返回错误响应
			http.Error(w, "Invalid appKey", http.StatusUnauthorized)
			return
		}

		// 如果密钥正确，调用下一个处理函数
		h.ServeHTTP(w, r)
	}
}

func JWTMiddleware(h http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// 从请求 Header 中获取 Authorization 令牌
		tokenString := r.Header.Get("Authorization")
		if len(tokenString) < 1 {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		userid, err := services.NewJWTService().ValidateToken(tokenString)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		slog.Debug(fmt.Sprintf("[%s] %s userid:[%v]", r.Method, r.URL.Path, userid))
		ctx := context.WithValue(r.Context(), "userid", userid)
		r = r.WithContext(ctx)

		// 调用下一个处理函数
		h.ServeHTTP(w, r)
	})
}
