package model

// OnboardingSteps 表示各个引导步骤的完成情况。
type OnboardingSteps struct {
	PhoneRegistered   bool `json:"phone_registered"`
	ContactsCompleted bool `json:"contacts_completed"`
	Finished          bool `json:"finished"`
}

// OnboardingProgressData 表示引导进度接口的响应数据。
type OnboardingProgressData struct {
	CurrentStep string          `json:"current_step"`
	Steps       OnboardingSteps `json:"steps"`
}
