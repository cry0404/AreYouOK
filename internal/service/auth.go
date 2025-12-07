package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"math/big"
	"sync"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"AreYouOK/config"
	"AreYouOK/internal/cache"
	"AreYouOK/internal/model"
	"AreYouOK/internal/model/dto"
	"AreYouOK/internal/repository/query"
	pkgerrors "AreYouOK/pkg/errors"
	"AreYouOK/pkg/logger"
	"AreYouOK/pkg/snowflake"
	"AreYouOK/pkg/token"
	"AreYouOK/storage/database"
	"AreYouOK/utils"
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

// 给予授权信息但还未登录注册， 现在是可以直接获取手机号了，直接获取手机号就不用一定要验证码注册了
// ExchangeAlipayAuthCode 支付宝授权换取（通过加密手机号）
func (s *AuthService) ExchangeAlipayAuthCode(
	ctx context.Context,
	encryptedData string,
	device dto.DeviceInfo,
	alipayOpenID string,
) (*dto.AuthExchangeResponse, error) {

	if alipayOpenID == "" {
		return nil, fmt.Errorf("alipay_open_id is required")
	}

	phone, err := utils.DecryptAlipayPhone(encryptedData)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt alipay phone: %w", err)
	}

	if !utils.ValidatePhone(phone) {
		return nil, pkgerrors.InvalidPhone
	}

	phoneHash := utils.HashPhone(phone)

	db := database.DB().WithContext(ctx)
	q := query.Use(db)

	user, err := q.User.Where(q.User.AlipayOpenID.Eq(alipayOpenID)).Where(q.User.PhoneHash.Eq(phoneHash)).First()
	isNewUser := false

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 未找到匹配手机号的记录，再按 open_id 查找是否已有未绑定手机号的用户
			user, err = q.User.Where(q.User.AlipayOpenID.Eq(alipayOpenID)).First()
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return nil, fmt.Errorf("failed to query user by alipay_open_id: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to query user by alipay_open_id and phone: %w", err)
		}
	}

	// phone_hash 全局唯一，若已被其他用户占用则直接报错，避免撞唯一键
	if existingByPhone, phoneErr := q.User.GetByPhoneHash(phoneHash); phoneErr == nil && existingByPhone != nil {
		if user == nil || existingByPhone.ID != user.ID {
			return nil, pkgerrors.PhoneAlreadyRegistered
		}
	} else if phoneErr != nil && !errors.Is(phoneErr, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("failed to query user by phone_hash: %w", phoneErr)
	}

	if user == nil {
		userCount, countErr := query.User.Count()
		if countErr != nil {
			return nil, fmt.Errorf("failed to count users: %w", countErr)
		}

		if userCount >= int64(config.Cfg.WaitlistMaxUsers) {
			return nil, pkgerrors.WaitlistFull
		}

		publicID, err := snowflake.NextID(snowflake.GeneratorTypeUser)
		if err != nil {
			return nil, fmt.Errorf("failed to generate user ID: %w", err)
		}

		phoneCipherBase64, err := utils.EncryptPhone(phone)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt phone: %w", err)
		}

		phoneCipherBytes, err := base64.StdEncoding.DecodeString(phoneCipherBase64)
		if err != nil {
			return nil, fmt.Errorf("failed to decode phone cipher: %w", err)
		}

		max := big.NewInt(100000)
		n, err := rand.Int(rand.Reader, max)
		nickname := fmt.Sprintf("安否用户%d", n.Int64()) // 默认值
		if err == nil {
			nickname = fmt.Sprintf("用户%d", n.Int64())
		}

		user = &model.User{
			PublicID:     publicID,
			AlipayOpenID: alipayOpenID,
			Nickname:     nickname,
			Status:       model.UserStatusContact, // 已绑定手机号，进入填写紧急联系人阶段
			Timezone:     "Asia/Shanghai",
			PhoneHash:    &phoneHash,
		}

		user.PhoneCipher = phoneCipherBytes

		err = db.Transaction(func(tx *gorm.DB) error {
			txQ := query.Use(tx)

			if err := txQ.User.Create(user); err != nil {
				return fmt.Errorf("failed to create user: %w", err)
			}

			defaultQuotaCents := config.Cfg.DefaultSMSQuota
			if defaultQuotaCents <= 0 {
				defaultQuotaCents = 100
			}

			wallet := &model.QuotaWallet{
				UserID:          user.ID,
				Channel:         model.QuotaChannelSMS,
				AvailableAmount: defaultQuotaCents,
				FrozenAmount:    0,
				UsedAmount:      0,
				TotalGranted:    defaultQuotaCents,
			}

			if err := txQ.QuotaWallet.Create(wallet); err != nil {
				return fmt.Errorf("failed to create quota wallet: %w", err)
			}

			quotaTransaction := &model.QuotaTransaction{
				UserID:          user.ID,
				Channel:         model.QuotaChannelSMS,
				TransactionType: model.TransactionTypeGrant,
				Reason:          "new_user_bonus",
				Amount:          defaultQuotaCents,
				BalanceAfter:    defaultQuotaCents,
			}

			if err := txQ.QuotaTransaction.Create(quotaTransaction); err != nil {
				return fmt.Errorf("failed to grant default SMS quota: %w", err)
			}

			logger.Logger.Info("User created with default SMS quota in transaction",
				zap.Int64("public_id", publicID),
				zap.Int64("user_id", user.ID),
				zap.Int("quota_cents", defaultQuotaCents),
			)

			return nil
		})

		if err != nil {
			return nil, fmt.Errorf("failed to create user with quota: %w", err)
		}

		isNewUser = true
		logger.Logger.Info("New user created via alipay phone",
			zap.Int64("public_id", publicID),
			zap.String("phone_hash", phoneHash),
		)
	} else {
		phoneCipherBase64, err := utils.EncryptPhone(phone)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt phone: %w", err)
		}

		phoneCipherBytes, err := base64.StdEncoding.DecodeString(phoneCipherBase64)
		if err != nil {
			return nil, fmt.Errorf("failed to decode phone cipher: %w", err)
		}

		updates := map[string]interface{}{
			"phone_cipher": phoneCipherBytes,
			"phone_hash":   phoneHash,
		}

		if user.Status == model.UserStatusWaitlisted || user.Status == model.UserStatusOnboarding {
			updates["status"] = string(model.UserStatusContact)
		}

		if user.AlipayOpenID == "" {
			updates["alipay_open_id"] = alipayOpenID
		}

		needInit := user.PhoneHash == nil || *user.PhoneHash == ""

		if needInit {
			err = db.Transaction(func(tx *gorm.DB) error {
				txQ := query.Use(tx)

				if _, updateErr := txQ.User.Where(txQ.User.ID.Eq(user.ID)).Updates(updates); updateErr != nil {
					return fmt.Errorf("failed to update user: %w", updateErr)
				}

				// 初次绑定手机号时初始化默认钱包
				if _, walletErr := txQ.QuotaWallet.Where(txQ.QuotaWallet.UserID.Eq(user.ID)).First(); walletErr != nil {
					if !errors.Is(walletErr, gorm.ErrRecordNotFound) {
						return fmt.Errorf("failed to query wallet: %w", walletErr)
					}

					defaultQuotaCents := config.Cfg.DefaultSMSQuota
					if defaultQuotaCents <= 0 {
						defaultQuotaCents = 100
					}

					wallet := &model.QuotaWallet{
						UserID:          user.ID,
						Channel:         model.QuotaChannelSMS,
						AvailableAmount: defaultQuotaCents,
						FrozenAmount:    0,
						UsedAmount:      0,
						TotalGranted:    defaultQuotaCents,
					}

					if err := txQ.QuotaWallet.Create(wallet); err != nil {
						return fmt.Errorf("failed to create quota wallet: %w", err)
					}

					quotaTransaction := &model.QuotaTransaction{
						UserID:          user.ID,
						Channel:         model.QuotaChannelSMS,
						TransactionType: model.TransactionTypeGrant,
						Reason:          "new_user_bonus",
						Amount:          defaultQuotaCents,
						BalanceAfter:    defaultQuotaCents,
					}

					if err := txQ.QuotaTransaction.Create(quotaTransaction); err != nil {
						return fmt.Errorf("failed to grant default SMS quota: %w", err)
					}
				}

				return nil
			})
			if err != nil {
				return nil, err
			}

			isNewUser = true
			logger.Logger.Info("New user initialized by alipay phone",
				zap.Int64("user_id", user.ID),
				zap.Int64("public_id", user.PublicID),
				zap.String("alipay_open_id", user.AlipayOpenID),
				zap.String("phone_hash", phoneHash),
			)
		} else {
			if _, updateErr := q.User.Where(q.User.ID.Eq(user.ID)).Updates(updates); updateErr != nil {
				return nil, fmt.Errorf("failed to update user: %w", updateErr)
			}
		}

		user.PhoneHash = &phoneHash
		user.PhoneCipher = phoneCipherBytes
		if newStatus, ok := updates["status"]; ok {
			user.Status = model.UserStatus(newStatus.(string))
		}
		if newOID, ok := updates["alipay_open_id"]; ok {
			user.AlipayOpenID = newOID.(string)
		}
	}

	userIDStr := fmt.Sprintf("%d", user.PublicID)
	accessToken, refreshToken, expiresIn, err := token.GenerateTokenPair(userIDStr)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// 存储 refresh token 到 Redis
	if err := cache.SetRefreshToken(ctx, userIDStr, refreshToken); err != nil {
		logger.Logger.Warn("Failed to store refresh token in Redis",
			zap.String("user_id", userIDStr),
			zap.Error(err),
		)
		// 不返回错误，因为 token 已经生成成功
	}

	// 手机号已验证（因为是从支付宝获取的）
	phoneVerified := true

	nextStep := resolveNextStep(user.Status, phoneVerified)

	return &dto.AuthExchangeResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
		User: dto.AuthUserSnapshot{
			ID:            userIDStr,
			Nickname:      user.Nickname,
			Status:        model.StatusToStringMap[user.Status],
			NextStep:      nextStep,
			PhoneVerified: phoneVerified,
			IsNewUser:     isNewUser,
			AlipayOpenID:  user.AlipayOpenID,
		},
	}, nil
}

// // VerifyPhoneCaptchaAndBind 验证验证码并绑定手机号（已登录用户）
// func (s *AuthService) VerifyPhoneCaptchaAndBind(
// 	ctx context.Context,
// 	userID string, // 从 JWT token 中获取的 public_id
// 	phone string,
// 	code string,
// ) (*dto.VerifyCaptchaResponse, error) {
// 	verifiService := Verification()
// 	if err := verifiService.VerifyCaptcha(ctx, phone, code); err != nil {
// 		return nil, err
// 	}

// 	phoneHash := utils.HashPhone(phone)

// 	db := database.DB().WithContext(ctx)
// 	q := query.Use(db)

// 	// 查询当前用户
// 	var userIDInt int64
// 	fmt.Sscanf(userID, "%d", &userIDInt)
// 	user, err := q.User.Where(q.User.PublicID.Eq(userIDInt)).First()
// 	if err != nil {
// 		if errors.Is(err, gorm.ErrRecordNotFound) {
// 			return nil, pkgerrors.ErrUserNotFound
// 		}
// 		return nil, fmt.Errorf("failed to query user: %w", err)
// 	}

// 	isNewUser := user.PhoneHash == nil || *user.PhoneHash == ""

// 	// 检查手机号是否已被其他用户注册
// 	existingUser, err := q.User.GetByPhoneHash(phoneHash)
// 	if err == nil && existingUser != nil {
// 		// 检查是否是当前用户自己
// 		if existingUser.PublicID != userIDInt {
// 			return nil, pkgerrors.PhoneAlreadyRegistered
// 		}
// 		// 如果是当前用户自己，说明已经绑定过了，直接返回成功
// 	}

// 	// 加密手机号
// 	phoneCipherBase64, err := utils.EncryptPhone(phone)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to encrypt phone: %w", err)
// 	}

// 	phoneCipherBytes, err := base64.StdEncoding.DecodeString(phoneCipherBase64)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to decode phone cipher: %w", err)
// 	}

// 	updates := map[string]interface{}{
// 		"phone_cipher": phoneCipherBytes,
// 		"phone_hash":   phoneHash,
// 	}

// 	// 状态流转：绑定手机号后的状态转换
// 	// waitlisted → contact：绑定手机号后，直接进入填写紧急联系人阶段
// 	// onboarding → contact：如果用户之前已经是 onboarding 状态（已授权但未绑定手机号），绑定后进入填写紧急联系人阶段
// 	if user.Status == model.UserStatusWaitlisted || user.Status == model.UserStatusOnboarding {
// 		updates["status"] = string(model.UserStatusContact)
// 	}

// 	if err := db.Transaction(func(tx *gorm.DB) error {
// 		txQ := query.Use(tx)

// 		if _, updateErr := txQ.User.Where(txQ.User.PublicID.Eq(userIDInt)).Updates(updates); updateErr != nil {
// 			return fmt.Errorf("failed to update user: %w", updateErr)
// 		}

// 		if isNewUser {
// 			// 首次绑定手机号时初始化默认钱包
// 			if _, walletErr := txQ.QuotaWallet.Where(txQ.QuotaWallet.UserID.Eq(user.ID)).First(); walletErr != nil {
// 				if !errors.Is(walletErr, gorm.ErrRecordNotFound) {
// 					return fmt.Errorf("failed to query wallet: %w", walletErr)
// 				}

// 				defaultQuotaCents := config.Cfg.DefaultSMSQuota
// 				if defaultQuotaCents <= 0 {
// 					defaultQuotaCents = 100
// 				}

// 				wallet := &model.QuotaWallet{
// 					UserID:          user.ID,
// 					Channel:         model.QuotaChannelSMS,
// 					AvailableAmount: defaultQuotaCents,
// 					FrozenAmount:    0,
// 					UsedAmount:      0,
// 					TotalGranted:    defaultQuotaCents,
// 				}

// 				if err := txQ.QuotaWallet.Create(wallet); err != nil {
// 					return fmt.Errorf("failed to create quota wallet: %w", err)
// 				}

// 				quotaTransaction := &model.QuotaTransaction{
// 					UserID:          user.ID,
// 					Channel:         model.QuotaChannelSMS,
// 					TransactionType: model.TransactionTypeGrant,
// 					Reason:          "new_user_bonus",
// 					Amount:          defaultQuotaCents,
// 					BalanceAfter:    defaultQuotaCents,
// 				}

// 				if err := txQ.QuotaTransaction.Create(quotaTransaction); err != nil {
// 					return fmt.Errorf("failed to grant default SMS quota: %w", err)
// 				}
// 			}
// 		}

// 		return nil
// 	}); err != nil {
// 		return nil, err
// 	}

// 	// 重新查询用户获取最新状态
// 	user, err = q.User.GetByPublicID(userIDInt)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to query updated user: %w", err)
// 	}

// 	accessToken, refreshToken, expiresIn, err := token.GenerateTokenPair(userID)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to generate token: %w", err)
// 	}

// 	if err := cache.SetRefreshToken(ctx, userID, refreshToken); err != nil {
// 		logger.Logger.Warn("Failed to update refresh token in Redis",
// 			zap.String("user_id", userID),
// 			zap.Error(err),
// 		)
// 	}

// 	return &dto.VerifyCaptchaResponse{
// 		AccessToken:  accessToken,
// 		RefreshToken: refreshToken,
// 		ExpiresIn:    expiresIn,
// 		User: dto.AuthUserSnapshot{
// 			ID:            userID,
// 			Nickname:      user.Nickname,
// 			Status:        model.StatusToStringMap[user.Status], // 根据 status 后续跳转
// 			NextStep:      resolveNextStep(user.Status, true),
// 			PhoneVerified: true,
// 			IsNewUser:     false,
// 			AlipayOpenID:  user.AlipayOpenID,
// 		},
// 	}, nil
// }

// VerifyPhoneCaptchaAndLogin 验证验证码并注册/登录（无需已登录）
func (s *AuthService) VerifyPhoneCaptchaAndLogin(
	ctx context.Context,
	phone string,
	code string,
	alipayOpenID string,
) (*dto.VerifyCaptchaResponse, error) {
	// 验证验证码
	verifiService := Verification()
	if err := verifiService.VerifyCaptcha(ctx, phone, code); err != nil {
		return nil, err
	}

	if alipayOpenID == "" {
		return nil, fmt.Errorf("alipay_open_id is required")
	}

	phoneHash := utils.HashPhone(phone)

	db := database.DB().WithContext(ctx)
	q := query.Use(db)

	// 先确保 phone_hash 未被其他用户占用
	if existingByPhone, phoneErr := q.User.GetByPhoneHash(phoneHash); phoneErr == nil && existingByPhone != nil {
		// 如已存在其它用户占用该手机号，则直接报错
		if existingByPhone.AlipayOpenID != alipayOpenID {
			return nil, pkgerrors.PhoneAlreadyRegistered
		}
	}

	// 查询用户：优先 open_id，再 fallback phone_hash（兼容极少数历史缓存场景）
	var user *model.User
	user, err := q.User.Where(q.User.AlipayOpenID.Eq(alipayOpenID)).First()
	isNewUser := false

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			user, err = q.User.GetByPhoneHash(phoneHash)
		}
	}

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// 新用户注册（要求 open_id）
			userCount, countErr := q.User.Count()
			if countErr != nil {
				return nil, fmt.Errorf("failed to count users: %w", countErr)
			}

			if userCount >= int64(config.Cfg.WaitlistMaxUsers) {
				return nil, pkgerrors.WaitlistFull
			}

			publicID, err := snowflake.NextID(snowflake.GeneratorTypeUser)
			if err != nil {
				return nil, fmt.Errorf("failed to generate user ID: %w", err)
			}

			phoneCipherBase64, err := utils.EncryptPhone(phone)
			if err != nil {
				return nil, fmt.Errorf("failed to encrypt phone: %w", err)
			}

			phoneCipherBytes, err := base64.StdEncoding.DecodeString(phoneCipherBase64)
			if err != nil {
				return nil, fmt.Errorf("failed to decode phone cipher: %w", err)
			}

			max := big.NewInt(100000)
			n, err := rand.Int(rand.Reader, max)

			nickname := fmt.Sprintf("安否用户%d", n.Int64()) // 默认值
			if err == nil {
				nickname = fmt.Sprintf("用户%d", n.Int64())
			}

			user = &model.User{
				PublicID:     publicID,
				AlipayOpenID: alipayOpenID,
				Nickname:     nickname,
				Status:       model.UserStatusContact,
				Timezone:     "Asia/Shanghai",
				PhoneHash:    &phoneHash,
				PhoneCipher:  phoneCipherBytes,
			}

			err = db.Transaction(func(tx *gorm.DB) error {
				txQ := query.Use(tx)

				if err := txQ.User.Create(user); err != nil {
					return fmt.Errorf("failed to create user: %w", err)
				}

				defaultQuotaCents := config.Cfg.DefaultSMSQuota
				if defaultQuotaCents <= 0 {
					defaultQuotaCents = 100 // 默认 100 cents = 20 次短信
				}

				wallet := &model.QuotaWallet{
					UserID:          user.ID,
					Channel:         model.QuotaChannelSMS,
					AvailableAmount: defaultQuotaCents,
					FrozenAmount:    0,
					UsedAmount:      0,
					TotalGranted:    defaultQuotaCents,
				}

				if err := txQ.QuotaWallet.Create(wallet); err != nil {
					return fmt.Errorf("failed to create quota wallet: %w", err)
				}

				quotaTransaction := &model.QuotaTransaction{
					UserID:          user.ID,
					Channel:         model.QuotaChannelSMS,
					TransactionType: model.TransactionTypeGrant,
					Reason:          "new_user_bonus", // 新用户奖励
					Amount:          defaultQuotaCents,
					BalanceAfter:    defaultQuotaCents, // 首次充值，余额等于充值金额
				}

				if err := txQ.QuotaTransaction.Create(quotaTransaction); err != nil {
					return fmt.Errorf("failed to grant default SMS quota: %w", err)
				}

				logger.Logger.Info("User created with default SMS quota in transaction",
					zap.Int64("public_id", publicID),
					zap.Int64("user_id", user.ID),
					zap.Int("quota_cents", defaultQuotaCents),
				)

				return nil
			})

			if err != nil {
				return nil, fmt.Errorf("failed to create user with quota: %w", err)
			}

			isNewUser = true
			logger.Logger.Info("New user registered via phone verification",
				zap.Int64("public_id", publicID),
				zap.String("phone_hash", phoneHash),
				zap.String("alipay_open_id", alipayOpenID),
			)
		} else {
			return nil, fmt.Errorf("failed to query user: %w", err)
		}
	} else {
		// 老用户：如缺少 open_id 则补写
		if user.AlipayOpenID == "" {
			if _, updateErr := q.User.Where(q.User.ID.Eq(user.ID)).Update(q.User.AlipayOpenID, alipayOpenID); updateErr != nil {
				return nil, fmt.Errorf("failed to update alipay_open_id: %w", updateErr)
			}
			user.AlipayOpenID = alipayOpenID
		}

		// 若老用户缺少手机号，则视作新用户初始化钱包
		needInit := user.PhoneHash == nil || *user.PhoneHash == ""

		phoneCipherBase64, err := utils.EncryptPhone(phone)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt phone: %w", err)
		}

		phoneCipherBytes, err := base64.StdEncoding.DecodeString(phoneCipherBase64)
		if err != nil {
			return nil, fmt.Errorf("failed to decode phone cipher: %w", err)
		}

		updates := map[string]interface{}{
			"phone_cipher": phoneCipherBytes,
			"phone_hash":   phoneHash,
		}

		if user.Status == model.UserStatusWaitlisted || user.Status == model.UserStatusOnboarding {
			updates["status"] = string(model.UserStatusContact)
		}

		if needInit {
			err = db.Transaction(func(tx *gorm.DB) error {
				txQ := query.Use(tx)

				if _, updateErr := txQ.User.Where(txQ.User.ID.Eq(user.ID)).Updates(updates); updateErr != nil {
					return fmt.Errorf("failed to update user: %w", updateErr)
				}

				if _, walletErr := txQ.QuotaWallet.Where(txQ.QuotaWallet.UserID.Eq(user.ID)).First(); walletErr != nil {
					if !errors.Is(walletErr, gorm.ErrRecordNotFound) {
						return fmt.Errorf("failed to query wallet: %w", walletErr)
					}

					defaultQuotaCents := config.Cfg.DefaultSMSQuota
					if defaultQuotaCents <= 0 {
						defaultQuotaCents = 100
					}

					wallet := &model.QuotaWallet{
						UserID:          user.ID,
						Channel:         model.QuotaChannelSMS,
						AvailableAmount: defaultQuotaCents,
						FrozenAmount:    0,
						UsedAmount:      0,
						TotalGranted:    defaultQuotaCents,
					}

					if err := txQ.QuotaWallet.Create(wallet); err != nil {
						return fmt.Errorf("failed to create quota wallet: %w", err)
					}

					quotaTransaction := &model.QuotaTransaction{
						UserID:          user.ID,
						Channel:         model.QuotaChannelSMS,
						TransactionType: model.TransactionTypeGrant,
						Reason:          "new_user_bonus",
						Amount:          defaultQuotaCents,
						BalanceAfter:    defaultQuotaCents,
					}

					if err := txQ.QuotaTransaction.Create(quotaTransaction); err != nil {
						return fmt.Errorf("failed to grant default SMS quota: %w", err)
					}
				}

				return nil
			})
			if err != nil {
				return nil, err
			}

			isNewUser = true
			logger.Logger.Info("New user initialized via phone verification",
				zap.Int64("user_id", user.ID),
				zap.Int64("public_id", user.PublicID),
				zap.String("alipay_open_id", user.AlipayOpenID),
				zap.String("phone_hash", phoneHash),
			)
		} else {
			if _, updateErr := q.User.Where(q.User.ID.Eq(user.ID)).Updates(updates); updateErr != nil {
				return nil, fmt.Errorf("failed to update user: %w", updateErr)
			}
		}

		user.PhoneHash = &phoneHash
		user.PhoneCipher = phoneCipherBytes
		if newStatus, ok := updates["status"]; ok {
			user.Status = model.UserStatus(newStatus.(string))
		}
	}

	// 生成 token
	userIDStr := fmt.Sprintf("%d", user.PublicID)
	accessToken, refreshToken, expiresIn, err := token.GenerateTokenPair(userIDStr)
	if err != nil {
		return nil, fmt.Errorf("failed to generate token: %w", err)
	}

	// 存储 refresh token 到 Redis
	if err := cache.SetRefreshToken(ctx, userIDStr, refreshToken); err != nil {
		logger.Logger.Warn("Failed to store refresh token in Redis",
			zap.String("user_id", userIDStr),
			zap.Error(err),
		)
	}

	return &dto.VerifyCaptchaResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresIn:    expiresIn,
		User: dto.AuthUserSnapshot{
			ID:            userIDStr,
			Nickname:      user.Nickname,
			Status:        model.StatusToStringMap[user.Status],
			NextStep:      resolveNextStep(user.Status, true),
			PhoneVerified: true,
			IsNewUser:     isNewUser,
			AlipayOpenID:  user.AlipayOpenID,
		},
	}, nil
}

func (s *AuthService) RefreshToken(
	ctx context.Context,
	refreshToken string,
) (*dto.AuthExchangeResponse, error) {
	// 将对应的 pkg 层的方法在 service 层中具体实现
	userIDStr, err := token.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, pkgerrors.AuthCodeInvalid
	}

	// 检验是否存在且匹配
	if !cache.ValidateRefreshTokenExists(ctx, userIDStr, refreshToken) {
		return nil, pkgerrors.AuthCodeInvalid
	}

	var userID int64
	fmt.Sscanf(userIDStr, "%d", &userID)
	user, err := query.User.Where(query.User.PublicID.Eq(userID)).First()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
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
			NextStep:      resolveNextStep(user.Status, phoneVerified),
			PhoneVerified: phoneVerified,
			IsNewUser:     false,
		},
	}, nil
}
