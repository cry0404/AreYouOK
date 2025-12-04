package service

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"AreYouOK/config"
	"AreYouOK/internal/model"
	"AreYouOK/internal/model/dto"
	"AreYouOK/internal/repository/query"
	pkgerrors "AreYouOK/pkg/errors"
	"AreYouOK/pkg/logger"
	"AreYouOK/storage/database"
	"AreYouOK/utils"
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

// 生产环境中紧急联系人还不应该是自己
func (s *ContactService) CreateContact(
	ctx context.Context,
	userID string,
	req dto.CreateContactRequest,
) (*dto.CreateContactResponse, error) {
	// 验证现在的优先级

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

	// 三个就达到了上限
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

	// 防止将自己设为紧急联系人
	if config.Cfg.Environment == "production" {
		if phoneHash == *user.PhoneHash {
			return nil, fmt.Errorf("emergencyContact can not be yourself")
		}
	}

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

	// 脱敏手机号用于响应, 不再处理
	phoneMasked := req.Phone

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

		phoneMasked := phone //直接返回不保密的部分

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

	if _, err := fmt.Sscanf(userID, "%d", &userIDInt); err != nil {
		return pkgerrors.InvalidUserID
	}

	user, err := query.User.GetByPublicID(userIDInt)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return pkgerrors.ErrUserIDNotFound
		}
		return fmt.Errorf("failed to query user: %w", err)
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

func (s *ContactService) UpdateContact(
	ctx context.Context,
	userID string,
	req *dto.UpdateContactRequest,
) (*dto.UpdateContactResponse, error) {
	var userIDInt int64
	if _, err := fmt.Sscanf(userID, "%d", &userIDInt); err != nil {
		return nil, pkgerrors.InvalidUserID
	}

	db := database.DB().WithContext(ctx)
	q := query.Use(db)

	user, err := q.User.GetByPublicID(userIDInt)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkgerrors.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	contacts := user.EmergencyContacts
	if len(contacts) == 0 {
		return nil, pkgerrors.Definition{
			Code:    "CONTACT_NOT_FOUND",
			Message: "No contacts to update",
		}
	}

	// 按 priority 找到要更新的联系人
	targetIdx := -1
	for i, c := range contacts {
		if c.Priority == req.Priority {
			targetIdx = i
			break
		}
	}
	if targetIdx == -1 {
		return nil, pkgerrors.ContactPriorityConflict // 或者自定义 CONTACT_NOT_FOUND
	}

	updatedContacts := make(model.EmergencyContacts, len(contacts))
	copy(updatedContacts, contacts)

	target := &updatedContacts[targetIdx]

	// 按需更新字段
	if req.DisplayName != "" {
		target.DisplayName = req.DisplayName
	}
	if req.Relationship != "" {
		target.Relationship = req.Relationship
	}

	phoneForResponse := ""
	if req.Phone != "" {
		// 加密手机号
		phoneCipherBase64, err := utils.EncryptPhone(req.Phone)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt phone: %w", err)
		}
		phoneHash := utils.HashPhone(req.Phone)

		// 防止将自己设为紧急联系人（仅生产环境）
		if config.Cfg.Environment == "production" && user.PhoneHash != nil {
			if phoneHash == *user.PhoneHash {
				return nil, fmt.Errorf("emergencyContact can not be yourself")
			}
		}

		target.PhoneCipherBase64 = phoneCipherBase64
		target.PhoneHash = phoneHash
		phoneForResponse = req.Phone
	} else {
		// 没改手机号，从已有数据解密，用于响应
		if target.PhoneCipherBase64 != "" {
			if cipherBytes, err := base64.StdEncoding.DecodeString(target.PhoneCipherBase64); err == nil {
				if phone, err := utils.DecryptPhone(cipherBytes); err == nil {
					phoneForResponse = phone
				}
			}
		}
	}

	// 写回 contacts
	if _, err := q.User.
		Where(q.User.PublicID.Eq(userIDInt)).
		Updates(map[string]interface{}{"emergency_contacts": updatedContacts}); err != nil {
		return nil, fmt.Errorf("failed to update user contacts: %w", err)
	}

	logger.Logger.Info("Contact updated",
		zap.String("user_id", userID),
		zap.Int("priority", req.Priority),
	)

	return &dto.UpdateContactResponse{
		DisplayName:  target.DisplayName,
		Relationship: target.Relationship,
		PhoneMasked:  phoneForResponse,
		Priority:     target.Priority,
	}, nil
}

// ReplaceContacts 全量替换紧急联系人
// 规则：
// 1. 删除所有现有联系人，用新列表完全替换
// 2. 联系人数量 1-3 个
// 3. 优先级必须唯一且在 1-3 范围内
// 4. 联系人手机号不能是用户自己（生产环境）
// 5. 如果是第一次添加联系人，更新用户状态为 active
func (s *ContactService) ReplaceContacts(
	ctx context.Context,
	userID string,
	req dto.ReplaceContactsRequest,
) ([]dto.ContactItem, error) {
	var userIDInt int64
	if _, err := fmt.Sscanf(userID, "%d", &userIDInt); err != nil {
		return nil, pkgerrors.InvalidUserID
	}

	db := database.DB().WithContext(ctx)
	q := query.Use(db)

	user, err := q.User.GetByPublicID(userIDInt)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, pkgerrors.ErrUserNotFound
		}
		return nil, fmt.Errorf("failed to query user: %w", err)
	}

	if len(req.Contacts) < 1 || len(req.Contacts) > 3 {
		return nil, pkgerrors.Definition{
			Code:    "INVALID_CONTACTS_COUNT",
			Message: "Contacts count must be between 1 and 3",
		}
	}

	prioritySet := make(map[int]bool)
	for _, contact := range req.Contacts {
		if contact.Priority < 1 || contact.Priority > 3 {
			return nil, pkgerrors.ContactPriorityConflict
		}
		if prioritySet[contact.Priority] {
			return nil, pkgerrors.Definition{
				Code:    "DUPLICATE_PRIORITY",
				Message: fmt.Sprintf("Duplicate priority: %d", contact.Priority),
			}
		}
		prioritySet[contact.Priority] = true
	}


	newContacts := make(model.EmergencyContacts, 0, len(req.Contacts))
	now := time.Now().Format(time.RFC3339)

	for _, contact := range req.Contacts {

		if !utils.ValidatePhone(contact.Phone) {
			return nil, pkgerrors.Definition{
				Code:    "INVALID_PHONE",
				Message: fmt.Sprintf("Invalid phone number for contact priority %d", contact.Priority),
			}
		}


		phoneCipherBase64, err := utils.EncryptPhone(contact.Phone)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt phone: %w", err)
		}

		phoneHash := utils.HashPhone(contact.Phone)


		if config.Cfg.Environment == "production" && user.PhoneHash != nil {
			if phoneHash == *user.PhoneHash {
				return nil, pkgerrors.Definition{
					Code:    "SELF_AS_CONTACT",
					Message: "Emergency contact cannot be yourself",
				}
			}
		}

		newContacts = append(newContacts, model.EmergencyContact{
			DisplayName:       contact.DisplayName,
			Relationship:      contact.Relationship,
			PhoneCipherBase64: phoneCipherBase64,
			PhoneHash:         phoneHash,
			Priority:          contact.Priority,
			CreatedAt:         now,
		})
	}

	// 按优先级排序
	sort.Slice(newContacts, func(i, j int) bool {
		return newContacts[i].Priority < newContacts[j].Priority
	})

	// 准备更新
	updates := map[string]interface{}{
		"emergency_contacts": newContacts,
	}

	// 如果之前没有联系人，且用户状态为 contact，更新为 active
	wasEmpty := len(user.EmergencyContacts) == 0
	if wasEmpty && user.Status == model.UserStatusContact {
		updates["status"] = string(model.UserStatusActive)
		logger.Logger.Info("User activated by replacing contacts",
			zap.String("user_id", userID),
			zap.Int64("public_id", user.PublicID),
		)
	}

	// 执行更新
	if _, err := q.User.Where(q.User.PublicID.Eq(userIDInt)).Updates(updates); err != nil {
		return nil, fmt.Errorf("failed to replace contacts: %w", err)
	}

	logger.Logger.Info("Contacts replaced",
		zap.String("user_id", userID),
		zap.Int("count", len(newContacts)),
	)

	// 构建响应（返回完整的联系人列表）
	result := make([]dto.ContactItem, 0, len(newContacts))
	for _, contact := range newContacts {
		// 解密手机号用于响应
		phoneCipherBytes, _ := base64.StdEncoding.DecodeString(contact.PhoneCipherBase64)
		phone, _ := utils.DecryptPhone(phoneCipherBytes)

		createdAt, _ := time.Parse(time.RFC3339, contact.CreatedAt)

		result = append(result, dto.ContactItem{
			DisplayName:  contact.DisplayName,
			Relationship: contact.Relationship,
			PhoneMasked:  phone, // 返回完整手机号
			Priority:     contact.Priority,
			CreatedAt:    createdAt,
		})
	}

	return result, nil
}
