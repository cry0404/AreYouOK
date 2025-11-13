package service

import (
	"AreYouOK/config"
	"AreYouOK/internal/cache"
	"AreYouOK/internal/model"
	"AreYouOK/internal/model/dto"
	"AreYouOK/internal/repository/query"
	"AreYouOK/pkg/errors"
	"AreYouOK/pkg/logger"
	"AreYouOK/pkg/snowflake"
	"AreYouOK/pkg/token"
	"AreYouOK/utils"
	"context"
	"encoding/base64"
	"fmt"
	"sync"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	authService *AuthService
	authOnce    sync.Once
)

func Auth() *AuthService {
	authOnce.Do(func() {
		authService = &AuthService{}
	})
	return authService
}

type AuthService struct{}


// 给予授权信息但还未登录注册
// ExchangeAlipayAuthCode 支付宝授权换取
func (s *AuthService) ExchangeAlipayAuthCode(
	ctx context.Context,
	authCode string,
	device dto.DeviceInfo,
) (*dto.AuthExchangeResponse, error) {
	// TODO: 用 authCode 调用支付宝的 api 来获取用户信息
	alipayUserID := "mock_alipay_userID" + authCode
	nickname := "cry"
	//先 mock 等待贝妮那边处理好
	// 查询用户是否存在
	user, err := query.User.Where(query.User.AlipayOpenID.Eq(alipayUserID)).First()
	isNewUser := false

	if err != nil {
		if err == gorm.ErrRecordNotFound {

			userCount, err := query.User.Count()
			if err != nil {
				return nil, fmt.Errorf("failed to count users: %w", err)
			}


			if userCount >= int64(config.Cfg.WaitlistMaxUsers) {
				return nil, errors.WaitlistFull
			}

			// 生成 public_id
			publicID, err := snowflake.NextID()
			if err != nil {
				return nil, fmt.Errorf("failed to generate user ID: %w", err)
			}

			// 创建新用户， 更新 isNewuser
			user = &model.User{
				PublicID:     publicID,
				AlipayOpenID: alipayUserID,
				Nickname:     nickname,
				Status:       model.UserStatusWaitlisted, //这里最初是应该是 waitlisted
				Timezone:     "Asia/Shanghai",
			}

			if err := query.User.Create(user); err != nil {
				return nil, fmt.Errorf("failed to create user: %w", err)
			}

			isNewUser = true
			logger.Logger.Info("New user created",
				zap.Int64("public_id", publicID),
				zap.String("alipay_user_id", alipayUserID),
			)
		} else {
			return nil, fmt.Errorf("failed to query user: %w", err)
		}
	}


	userIDStr := fmt.Sprintf("%d", user.PublicID)
	accessToken, refreshToken, expiresIn, err := token.GenerateTokenPair(userIDStr)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// 存储 refresh token 到 Redis，保持缓存即可
	if err := cache.SetRefreshToken(ctx, userIDStr, refreshToken); err != nil {
		logger.Logger.Warn("Failed to store refresh token in Redis",
			zap.String("user_id", userIDStr),
			zap.Error(err),
		)
		// 不返回错误，因为 token 已经生成成功
	}

	// 检查手机号是否已验证
	phoneVerified := user.PhoneHash != nil && *user.PhoneHash != ""

	return &dto.AuthExchangeResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
		User: dto.AuthUserSnapshot{
			ID:            userIDStr,
			Nickname:      user.Nickname,
			Status:        model.StatusToStringMap[user.Status],
			PhoneVerified: phoneVerified,
			IsNewUser:     isNewUser, //默认为 false
		},
	}, nil
}


// 登录时的内容
// VerifyPhoneCaptchaAndBind 验证验证码并绑定手机号
func (s *AuthService) VerifyPhoneCaptchaAndBind(
	ctx context.Context,
	userID string, // 从 JWT token 中获取的 public_id
	phone string,
	code string,
	scene string,
) (*dto.VerifyCaptchaResponse, error) {

	verifiService := Verification()
	if err := verifiService.VerifyCaptcha(ctx, phone, scene, code); err != nil {
		return nil, err
	}

	phoneHash := utils.HashPhone(phone)

	// 查询当前用户
	var userIDInt int64
	fmt.Sscanf(userID, "%d", &userIDInt)
	user, err := query.User.Where(query.User.PublicID.Eq(userIDInt)).First()
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	// 检查手机号是否已被其他用户注册
	existingUser, err := query.User.GetByPhoneHash(phoneHash)
	if err == nil && existingUser != nil {
		// 检查是否是当前用户自己
		if existingUser.PublicID != userIDInt {
			return nil, errors.PhoneAlreadyRegistered
		}
		// 如果是当前用户自己，说明已经绑定过了，直接返回成功
	}

	// 加密手机号
	phoneCipherBase64, err := utils.EncryptPhone(phone)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt phone: %w", err)
	}

	phoneCipherBytes, err := base64.StdEncoding.DecodeString(phoneCipherBase64)
	if err != nil {
		return nil, fmt.Errorf("failed to decode phone cipher: %w", err)
	}

	// 更新用户手机号和状态
	updates := map[string]interface{}{
		"phone_cipher": phoneCipherBytes,
		"phone_hash":   phoneHash,
	}

	// 如果用户状态是 waitlisted，更新为 onboarding // 理论上能绑定的情况都可以
	if user.Status == model.UserStatusWaitlisted {
		updates["status"] = string(model.UserStatusOnboarding)

		//只有最初时才会，也有可能有用户没有创建对应的联系人，处在 contact 状态
	}


	if _, err := query.User.Where(query.User.PublicID.Eq(userIDInt)).Updates(updates); err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	// 重新查询用户获取最新状态
	user, err = query.User.Where(query.User.PublicID.Eq(userIDInt)).First()
	if err != nil {
		return nil, fmt.Errorf("failed to query updated user: %w", err)
	}


	accessToken, refreshToken, expiresIn, err := token.GenerateTokenPair(userID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}


	if err := cache.SetRefreshToken(ctx, userID, refreshToken); err != nil {
		logger.Logger.Warn("Failed to update refresh token in Redis",
			zap.String("user_id", userID),
			zap.Error(err),
		)
	}

	return &dto.VerifyCaptchaResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
		User: dto.AuthUserSnapshot{
			ID:            userID,
			Nickname:      user.Nickname,
			Status:        model.StatusToStringMap[user.Status], //根据 status 后续跳转
			PhoneVerified: true,
			IsNewUser:     false,
		},
	}, nil
}


func (s *AuthService) RefreshToken(
	ctx context.Context,
	refreshToken string,
) (*dto.AuthExchangeResponse, error) {
	//将对应的 pkg 层的方法在 service 层中具体实现
	userIDStr, err := token.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, errors.AuthCodeInvalid
	}

	//检验是否存在且匹配
	if !cache.ValidateRefreshTokenExists(ctx, userIDStr, refreshToken) {
		return nil, errors.AuthCodeInvalid
	}


	var userID int64
	fmt.Sscanf(userIDStr, "%d", &userID)
	user, err := query.User.Where(query.User.PublicID.Eq(userID)).First()
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("user not found")
		}
		return nil, fmt.Errorf("failed to query user: %w", err)
	}


	accessToken, newRefreshToken, expiresIn, err := token.GenerateTokenPair(userIDStr)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}


	if err := cache.SetRefreshToken(ctx, userIDStr, newRefreshToken); err != nil {
		logger.Logger.Warn("Failed to update refresh token in Redis",
			zap.String("user_id", userIDStr),
			zap.Error(err),
		)
	}

	phoneVerified := user.PhoneHash != nil && *user.PhoneHash != ""

	return &dto.AuthExchangeResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    expiresIn,
		User: dto.AuthUserSnapshot{
			ID:            userIDStr,
			Nickname:      user.Nickname,
			Status:        model.StatusToStringMap[user.Status],
			PhoneVerified: phoneVerified,
			IsNewUser:     false,
		},
	}, nil
}