package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Daneel-Li/gps-back/internal/config"
	mxm "github.com/Daneel-Li/gps-back/internal/models"

	"github.com/dgrijalva/jwt-go"
)

type JWTService interface {
	GenerateToken(user mxm.User) (string, error)
	ValidateToken(tokenString string) (uint, error)
}
type jWTServiceImpl struct {
}

func NewJWTService() JWTService {
	return &jWTServiceImpl{}
}

// Key 是用于签名JWT的密钥

// generateToken 生成JWT令牌
func (j *jWTServiceImpl) GenerateToken(user mxm.User) (string, error) {
	// 设置claims
	claims := jwt.MapClaims{
		"iss":    config.GetConfig().JwtIssuer,
		"userid": user.ID,
		"openid": user.OpenID,
		"exp":    time.Now().Add(time.Hour * 24).Unix(), // 令牌有效期为24小时
		"iat":    time.Now().Unix(),
	}

	// 创建token对象，使用HS256签名
	token := jwt.New(jwt.SigningMethodHS256)
	token.Claims = claims

	// 签名并获取token字符串
	return token.SignedString(config.GetConfig().JwtKey)
}

func (j *jWTServiceImpl) ValidateToken(tokenString string) (uint, error) {
	// 解析令牌字符串，忽略 "Bearer " 前缀
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")

	// 解析 JWT
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// 这里可以添加密钥验证逻辑
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return config.GetConfig().JwtKey, nil
	})

	if err != nil {
		return 0, fmt.Errorf("token parse failed: %w", err)
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok || !token.Valid {
		return 0, fmt.Errorf("invalid token claims")
	}

	// 4. 校验时效性和发行人
	if !claims.VerifyIssuer(config.GetConfig().JwtIssuer, true) {
		return 0, fmt.Errorf("issuer validation failed")
	}
	if !claims.VerifyExpiresAt(time.Now().Unix(), true) {
		return 0, fmt.Errorf("token expired")
	}

	// 5. 提取userID
	userID, ok := claims["userid"].(float64) // JSON数字默认解析为float64
	if !ok {
		return 0, errors.New("userid claim missing or invalid type")
	}

	return uint(userID), nil
}
