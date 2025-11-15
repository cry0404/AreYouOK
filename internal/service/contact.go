package service

import (
	"AreYouOK/internal/model"
	"AreYouOK/internal/model/dto"
	"AreYouOK/internal/repository/query"
	pkgerrors "AreYouOK/pkg/errors"
	"AreYouOK/pkg/logger"
	"AreYouOK/utils"
	"encoding/base64"
	"errors"
	"sort"
	"time"

	"go.uber.org/zap"

	"context"
	"fmt"
	"sync"

	"gorm.io/gorm"
)

// api 中设计的 user_ID 是 public_id

var (
	contactService *ContactService
	contactOnce    sync.Once
)

func Contact() *ContactService {
	contactOnce.Do(func() {
		contactService = &ContactService{}
	})

	return contactService
}

type ContactService struct{}

// 应该是一个一个添加的
// CreateContact 创建一个新的联系人，但不超过三，需要更新优先级，以及注册的 status
func (s *ContactService) CreateContact(
	ctx context.Context,
	userID string,
	req dto.CreateContactRequest,
) (*dto.CreateContactResponse, error) {
	//验证现在的优先级

	if req.Priority < 1 || req.Priority > 3 {
		return nil, pkgerrors.ContactPriorityConflict
	}

	// 在 handler 层验证 if !utils.ValidatePhone(req.Phone)

	var userIDInt int64

	if _, err := fmt.Sscanf(userID, "%d", &userIDInt); err != nil {
		return nil, pkgerrors.InvalidUserID
	}

	user, err := query.User.GetByPublicID(userIDInt)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkgerrors.ErrUserNotFound
		}

		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	contacts := user.EmergencyContacts

	//三个就达到了上限
	if len(contacts) >= 3 {
		return nil, pkgerrors.ContactLimitReached
	} // 如果是 0 的话需要更新 user 的 status

	for _, contact := range contacts {
		if contact.Priority == req.Priority {
			return nil, pkgerrors.ContactPriorityConflict
		}
	}

	phoneCipherBase64, err := utils.EncryptPhone(req.Phone)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt phone: %w", err)
	}

	phoneHash := utils.HashPhone(req.Phone)

	newContact := model.EmergencyContact{
		DisplayName:       req.DisplayName,
		Relationship:      req.Relationship,
		PhoneCipherBase64: phoneCipherBase64,
		PhoneHash:         phoneHash,
		Priority:          req.Priority,
		CreatedAt:         time.Now().Format(time.RFC3339),
	}

	isFirstContact := len(contacts) == 0

	updatedContacts := append(contacts, newContact)

	// 更新用户记录
	updates := map[string]interface{}{
		"emergency_contacts": updatedContacts,
	}

	// 如果这是第一个联系人，且用户状态为 contact 或 onboarding，更新为 active
	if isFirstContact && (user.Status == model.UserStatusContact) {
		updates["status"] = string(model.UserStatusActive)
		logger.Logger.Info("User activated by adding first contact",
			zap.String("user_id", userID),
			zap.Int64("public_id", user.PublicID),
		)
	}

	if _, err := query.User.Where(query.User.PublicID.Eq(userIDInt)).Updates(updates); err != nil {
		return nil, fmt.Errorf("failed to update user contacts: %w", err)
	}

	// 脱敏手机号用于响应
	phoneMasked := utils.MaskPhone(req.Phone)

	return &dto.CreateContactResponse{
		DisplayName:  newContact.DisplayName,
		Relationship: newContact.Relationship,
		PhoneMasked:  phoneMasked,
		Priority:     newContact.Priority,
	}, nil
}

// 删除后要对优先级重新进行排序, 也只需要优先级即可找到
func (s *ContactService) ListContacts(
	ctx context.Context,
	userID string,
) ([]dto.ContactItem, error) {
	var userIDInt int64

	if _, err := fmt.Sscanf(userID, "%d", &userIDInt); err != nil {
		return nil, pkgerrors.InvalidUserID
	}

	user, err := query.User.GetByPublicID(userIDInt)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkgerrors.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	contacts := user.EmergencyContacts

	if len(contacts) == 0 {
		return []dto.ContactItem{}, nil
	}

	result := make([]dto.ContactItem, 0, len(contacts))

	for _, contact := range contacts {
		phoneCipherBytes, err := base64.StdEncoding.DecodeString(contact.PhoneCipherBase64)
		if err != nil {
			logger.Logger.Warn("Failed to decode phone cipher",
				zap.String("user_id", userID),
				zap.Int("priority", contact.Priority),
				zap.Error(err),
			)
			continue
		}

		phone, err := utils.DecryptPhone(phoneCipherBytes)

		if err != nil {
			logger.Logger.Warn("Failed to decrypt phone",
				zap.String("user_id", userID),
				zap.Int("priority", contact.Priority),
				zap.Error(err),
			)
			continue
		}

		phoneMasked := utils.MaskPhone(phone)

		createdAt, _ := time.Parse(time.RFC3339, contact.CreatedAt)

		result = append(result, dto.ContactItem{
			DisplayName:  contact.DisplayName,
			Relationship: contact.Relationship,
			PhoneMasked:  phoneMasked,
			Priority:     contact.Priority,
			CreatedAt:    createdAt,
		})
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Priority < result[j].Priority
	})
	return result, nil
}

func (s *ContactService) DeleteContact(
	ctx context.Context,
	userID string,
	priority int,
) error {
	if priority < 1 || priority > 3 {
		return pkgerrors.ContactPriorityConflict
	}

	var userIDInt int64

	if _, err := fmt.Scanf(userID, "%d", &userIDInt); err != nil {
		return pkgerrors.InvalidUserID
	}

	user, err := query.User.GetByPublicID(userIDInt)

	if err != nil {
		if err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return pkgerrors.ErrUserIDNotFound
			}
			return fmt.Errorf("failed to query user: %w", err)
		}
	}

	contacts := user.EmergencyContacts

	if len(contacts) <= 1 {
		return pkgerrors.Definition{
			Code:    "CONTACT_MIN_REQUIRED",
			Message: "At least one contact is required",
		}
	}

	newContacts := make(model.EmergencyContacts, 0, len(contacts))
	found := false

	for _, contact := range contacts {
		if contact.Priority == priority {
			found = true
			continue // 跳过要删除的联系人
		}
		newContacts = append(newContacts, contact)
	}

	if !found {
		return pkgerrors.ContactPriorityConflict
	}

	sort.Slice(newContacts, func(i, j int) bool {
		return newContacts[i].Priority < newContacts[j].Priority
	})

	// 重新分配优先级，确保连续, 避免出现 1,3 的状况
	for i := range newContacts {
		newContacts[i].Priority = i + 1
	}

	updates := map[string]interface{}{
		"emergency_contacts": newContacts,
	}

	if _, err := query.User.Where(query.User.PublicID.Eq(userIDInt)).Updates(updates); err != nil {
		return fmt.Errorf("failed to delete contact: %w", err)
	}

	logger.Logger.Info("Contact deleted",
		zap.String("user_id", userID),
		zap.Int("priority", priority),
	)

	return nil
}
