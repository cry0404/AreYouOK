package model

import "time"

// 不同场景下需要定义的消息，模板 id， 签名 id，需要配置的参数，根据阿里云上的配置决定

type smsMessage struct {
	Phone        string
	SignName     string
	TemplateCode string
}

// 模板内容
// 您的联系人${name}，今日的平安打卡任务还未完成，请及时联系 ta 确认情况。
type CheckInReminderContactMessage struct { // 打卡通知紧急联系人
	smsMessage
	Name string
}

// 您的联系人${name}没有进行归来打卡，请联系 ta 确认情况。行程信息：${trip}，预计归来时间：${time}。备注: ${note}。
type JourneyReminderContactMessage struct { //旅行联系紧急联系人
	smsMessage

	Name     string
	Trip     string
	Deadline time.Time
	Note     string
}

// 安否温馨提示您，您进行了行程报备功能，请于 10 分钟内进行打卡；如果没有按时打卡，我们将按约定联系您的紧急联系人。
type JourneyTimeOut struct {
	smsMessage
}

type CheckInTimeOut struct {
	smsMessage

	Name     string
	Deadline time.Time
}
