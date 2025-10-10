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

// Key is the secret key used for signing JWT

// generateToken generates JWT token
func (j *jWTServiceImpl) GenerateToken(user mxm.User) (string, error) {
	// Set claims
	claims := jwt.MapClaims{
		"iss":    config.GetConfig().JwtIssuer,
		"userid": user.ID,
		"openid": user.OpenID,
		"exp":    time.Now().Add(time.Hour * 24).Unix(), // Token valid for 24 hours
		"iat":    time.Now().Unix(),
	}

	// Create token object using HS256 signing
	token := jwt.New(jwt.SigningMethodHS256)
	token.Claims = claims

	// Sign and get token string
	return token.SignedString(config.GetConfig().JwtKey)
}

func (j *jWTServiceImpl) ValidateToken(tokenString string) (uint, error) {
	// Parse token string, ignore "Bearer " prefix
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")

	// Parse JWT
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		// Key validation logic can be added here
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

	// 4. Verify timeliness and issuer
	if !claims.VerifyIssuer(config.GetConfig().JwtIssuer, true) {
		return 0, fmt.Errorf("issuer validation failed")
	}
	if !claims.VerifyExpiresAt(time.Now().Unix(), true) {
		return 0, fmt.Errorf("token expired")
	}

	// 5. Extract userID
	userID, ok := claims["userid"].(float64) // JSON numbers are parsed as float64 by default
	if !ok {
		return 0, errors.New("userid claim missing or invalid type")
	}

	return uint(userID), nil
}
