package service

import (
	"errors"
	"regexp"
	"strings"
	"unicode"

	"github.com/QuantumNous/new-api/common"
	"github.com/QuantumNous/new-api/model"
)

const (
	ContactMessageNameMaxLength    = 64
	ContactMessagePhoneMaxLength   = 32
	ContactMessageMessageMaxLength = 1000
	ContactMessageRemarkMaxLength  = 500
)

var contactMessagePhonePattern = regexp.MustCompile(`^[0-9+\-()\s]{5,32}$`)

type ContactMessageSubmitRequest struct {
	Name     string `json:"name"`
	Phone    string `json:"phone"`
	Message  string `json:"message"`
	ClientIP string `json:"-"`
}

type ContactMessageUpdateRequest struct {
	Status string `json:"status"`
	Remark string `json:"remark"`
}

// SubmitContactMessage 校验并保存首页访客留言。
func SubmitContactMessage(req ContactMessageSubmitRequest) (*model.ContactMessage, error) {
	name := strings.TrimSpace(req.Name)
	phone := strings.TrimSpace(req.Phone)
	message := strings.TrimSpace(req.Message)
	clientIP := strings.TrimSpace(req.ClientIP)

	if err := validateContactMessageSubmit(name, phone, message); err != nil {
		return nil, err
	}

	record := &model.ContactMessage{
		Name:     name,
		Phone:    phone,
		Message:  message,
		ClientIp: clientIP,
		Status:   model.ContactMessageStatusPending,
	}
	if err := model.CreateContactMessage(record); err != nil {
		return nil, err
	}
	return record, nil
}

// ListContactMessages 查询管理员留言列表。
func ListContactMessages(pageInfo *common.PageInfo, status string) ([]*model.ContactMessage, int64, error) {
	status = strings.TrimSpace(status)
	if status != "" && !model.IsValidContactMessageStatus(status) {
		return nil, 0, errors.New("留言状态无效")
	}
	return model.ListContactMessages(pageInfo, status)
}

// UpdateContactMessage 更新管理员处理状态和备注。
func UpdateContactMessage(id int, adminId int, req ContactMessageUpdateRequest) (*model.ContactMessage, error) {
	status := strings.TrimSpace(req.Status)
	remark := strings.TrimSpace(req.Remark)
	if !model.IsValidContactMessageStatus(status) {
		return nil, errors.New("留言状态无效")
	}
	if len([]rune(remark)) > ContactMessageRemarkMaxLength {
		return nil, errors.New("备注不能超过 500 个字符")
	}
	return model.UpdateContactMessageProcessing(id, status, remark, adminId, common.GetTimestamp())
}

// DeleteContactMessage 删除留言记录。
func DeleteContactMessage(id int) error {
	return model.DeleteContactMessageById(id)
}

func validateContactMessageSubmit(name string, phone string, message string) error {
	if name == "" {
		return errors.New("姓名不能为空")
	}
	if len([]rune(name)) > ContactMessageNameMaxLength {
		return errors.New("姓名不能超过 64 个字符")
	}
	if phone == "" {
		return errors.New("电话不能为空")
	}
	if len([]rune(phone)) > ContactMessagePhoneMaxLength || !isValidContactMessagePhone(phone) {
		return errors.New("电话格式不正确")
	}
	if len([]rune(message)) > ContactMessageMessageMaxLength {
		return errors.New("留言不能超过 1000 个字符")
	}
	return nil
}

func isValidContactMessagePhone(phone string) bool {
	if !contactMessagePhonePattern.MatchString(phone) {
		return false
	}
	digitCount := 0
	for _, r := range phone {
		if unicode.IsDigit(r) {
			digitCount++
		}
	}
	return digitCount >= 5
}
