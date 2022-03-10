package mq_mail

import (
	"encoding/base64"
	"encoding/json"
	"github.com/Zzaniu/zrpc/tool/mq/rabbit"
	"golang.org/x/xerrors"
	"io/ioutil"
	"path"
)

type MailInfo struct {
	Mail Mail `json:"mail"`
}

type Mail struct {
	To         string      `json:"to"`
	Cc         string      `json:"cc"`
	Bcc        string      `json:"bcc"`
	Title      string      `json:"title"`
	Body       string      `json:"body"`
	Attachment [][2]string `json:"attachment"`
	Foreign    bool        `json:"foreign"`
}

// SendMail 基于路径发送邮件附件
func SendMail(product *rabbit.RbMqClient, to, cc, bcc, title, body string, filePath []string, foreign bool) (bool, error) {
	var attachment [][2]string
	for _, fileName := range filePath {
		file, err := ioutil.ReadFile(fileName)
		if err != nil {
			return false, xerrors.Errorf("ioutil.ReadFile error, err = %w", err)
		}
		fileStr := base64.StdEncoding.EncodeToString(file)
		attachment = append(attachment, [2]string{path.Base(fileName), fileStr})
	}
	m := MailInfo{Mail: Mail{To: to, Cc: cc, Bcc: bcc, Title: title, Body: body, Attachment: attachment, Foreign: foreign}}
	mStr, err := json.Marshal(m)
	if err != nil {
		return false, xerrors.Errorf("json.Marshal error, err = %w", err)
	}
	if product.Publish(mStr) {
		return true, nil
	} else {
		return false, nil
	}
}

// SendMailByte 基于二进制数据发送邮件附件
func SendMailByte(product *rabbit.RbMqClient, to, cc, bcc, title, body string, fileInfo map[string][]byte, foreign bool) (bool, error) {
	var attachment [][2]string
	for fineName, fileContent := range fileInfo {
		fileStr := base64.StdEncoding.EncodeToString(fileContent)
		attachment = append(attachment, [2]string{fineName, fileStr})
	}
	m := MailInfo{Mail: Mail{To: to, Cc: cc, Bcc: bcc, Title: title, Body: body, Attachment: attachment, Foreign: foreign}}
	mStr, err := json.Marshal(m)
	if err != nil {
		return false, xerrors.Errorf("json.Marshal error, err = %w", err)
	}
	if product.Publish(mStr) {
		return true, nil
	} else {
		return false, nil
	}
}
