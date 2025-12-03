package model

import (
	"encoding/json"
	"fmt"
	"time"
)

// 以接口方式来实现
// SMSMessage 短信消息接口
// 所有短信消息类型都需要实现此接口
type SMSMessage interface {
	// GetPhone 获取手机号
	GetPhone() string
	// SetPhone 设置手机号
	SetPhone(phone string)
	// GetSignName 获取签名名称
	GetSignName() string
	// SetSignName 设置签名名称
	SetSignName(signName string)
	// GetTemplateCode 获取模板代码
	GetTemplateCode() string
	// SetTemplateCode 设置模板代码
	SetTemplateCode(templateCode string)
	// GetTemplateParams 获取模板参数（JSON 字符串）
	GetTemplateParams() (string, error)
	// GetMessageType 获取消息类型，用于序列化/反序列化
	GetMessageType() string
}

// smsMessage 基础消息结构
type smsMessage struct {
	Phone        string `json:"phone"`
	SignName     string `json:"sign_name"`
	TemplateCode string `json:"template_code"`
}

func (m *smsMessage) GetPhone() string {
	return m.Phone
}

func (m *smsMessage) SetPhone(phone string) {
	m.Phone = phone
}

func (m *smsMessage) GetSignName() string {
	return m.SignName
}

func (m *smsMessage) SetSignName(signName string) {
	m.SignName = signName
}

func (m *smsMessage) GetTemplateCode() string {
	return m.TemplateCode
}

func (m *smsMessage) SetTemplateCode(templateCode string) {
	m.TemplateCode = templateCode
}

// CheckInReminderContactMessage 打卡通知紧急联系人
// 模板内容：您的联系人${name}，今日的平安打卡任务还未完成，请及时联系 ta 确认情况。
type CheckInReminderContactMessage struct {
	smsMessage
	Name string `json:"name"`
}

func (m *CheckInReminderContactMessage) GetTemplateParams() (string, error) {
	params := map[string]string{
		"name": m.Name,
	}
	data, err := json.Marshal(params)
	return string(data), err
}

func (m *CheckInReminderContactMessage) GetMessageType() string {
	return "checkin_reminder_contact"
}

// JourneyReminderContactMessage 旅行联系紧急联系人
// 模板内容：您的联系人${name}没有进行归来打卡，请联系 ta 确认情况。行程信息：${trip}，预计归来时间：${time}。备注: ${note}。
type JourneyReminderContactMessage struct {
	smsMessage
	Name string `json:"name"` // 用户昵称
	Trip string `json:"trip"` // 行程标题
	Time string `json:"time"` // 预计返回时间（字符串格式）
	Note string `json:"note"` // 备注
}

func (m *JourneyReminderContactMessage) GetTemplateParams() (string, error) {
	params := map[string]string{
		"name": m.Name,
		"trip": m.Trip,
		"time": m.Time,
		"note": m.Note,
	}
	data, err := json.Marshal(params)
	return string(data), err
}

func (m *JourneyReminderContactMessage) GetMessageType() string {
	return "journey_reminder_contact"
}

// CheckInReminder 打卡提醒（发送给用户本人）
// 模板内容：亲爱的${name},您今日的打卡任务还未完成,请在设定时间${time} 完成打卡,如若没有按成完成打卡,我们将按照约定提醒您的紧急联系人
type CheckInReminder struct {
	smsMessage
	Name string `json:"name"` // 用户昵称
	Time string `json:"time"` // 打卡截止时间，格式：HH:mm:ss
}

func (m *CheckInReminder) GetTemplateParams() (string, error) {
	params := map[string]string{
		"name": m.Name,
		"time": m.Time,
	}
	data, err := json.Marshal(params)
	return string(data), err
}

func (m *CheckInReminder) GetMessageType() string {
	return "checkin_reminder"
}

// JourneyTimeOut 行程超时提醒（发送给用户本人）
// 模板内容：安否温馨提示您，您进行了行程报备功能，请于 10 分钟内进行打卡；如果没有按时打卡，我们将按约定联系您的紧急联系人。
type JourneyTimeOut struct {
	smsMessage
}

func (m *JourneyTimeOut) GetTemplateParams() (string, error) {
	return "{}", nil
}

func (m *JourneyTimeOut) GetMessageType() string {
	return "journey_timeout"
}

// CheckInTimeOut 打卡超时提醒
type CheckInTimeOut struct {
	smsMessage
	Name     string    `json:"name"`
	Deadline time.Time `json:"time"`
}

func (m *CheckInTimeOut) GetTemplateParams() (string, error) {
	params := map[string]string{
		"name":     m.Name,
		"deadline": m.Deadline.Format("2006-01-02 15:04:05"),
	}
	data, err := json.Marshal(params)
	return string(data), err
}

func (m *CheckInTimeOut) GetMessageType() string {
	return "checkin_timeout"
}

// QuotaDepleted 额度耗尽提醒
// 模板内容：安否温馨提示，您的紧急联系次数已经用尽，如有新的紧急联系情况，安否将不再进行通知。
type QuotaDepleted struct {
	smsMessage
}

func (m *QuotaDepleted) GetTemplateParams() (string, error) {
	return "{}", nil
}

func (m *QuotaDepleted) GetMessageType() string {
	return "quota_depleted"
}

// ParseSMSMessage 从 map[string]interface{} 解析为具体的 SMSMessage

func ParseSMSMessage(payload map[string]interface{}) (SMSMessage, error) {
	messageType, ok := payload["type"].(string)
	if !ok {
		return nil, fmt.Errorf("missing or invalid message type in payload")
	}

	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	switch messageType {
	case "checkin_reminder":
		var msg CheckInReminder
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, fmt.Errorf("failed to parse CheckInReminder: %w", err)
		}
		return &msg, nil
	case "checkin_reminder_contact":
		var msg CheckInReminderContactMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, fmt.Errorf("failed to parse CheckInReminderContactMessage: %w", err)
		}
		return &msg, nil
	case "journey_reminder_contact":
		var msg JourneyReminderContactMessage
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, fmt.Errorf("failed to parse JourneyReminderContactMessage: %w", err)
		}
		return &msg, nil
	case "journey_timeout":
		var msg JourneyTimeOut
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, fmt.Errorf("failed to parse JourneyTimeOut: %w", err)
		}
		return &msg, nil
	case "checkin_timeout":
		var msg CheckInTimeOut
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, fmt.Errorf("failed to parse CheckInTimeOut: %w", err)
		}
		return &msg, nil
	case "quota_depleted":
		var msg QuotaDepleted
		if err := json.Unmarshal(data, &msg); err != nil {
			return nil, fmt.Errorf("failed to parse QuotaDepleted: %w", err)
		}
		return &msg, nil
	default:
		return nil, fmt.Errorf("unknown message type: %s", messageType)
	}
}
