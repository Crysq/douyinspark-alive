package signer

import (
	"encoding/json"
	"fmt"
	"net/url"
)

const (
	imSendEndpoint = "https://www.douyin.com/aweme/v1/web/im/send/msg/?device_platform=webapp&aid=6383&channel=channel_pc_web&pc_client_type=1&version_code=190500&version_name=19.5.0&cookie_enabled=true&platform=PC"
	imListEndpoint = "https://www.douyin.com/aweme/v1/web/im/conversation/list/?device_platform=webapp&aid=6383&channel=channel_pc_web&pc_client_type=1&version_code=190500&version_name=19.5.0&cookie_enabled=true&platform=PC&cursor=0&count=50"
)

// IMMessage 私信接口返回体。
type IMMessage struct {
	StatusCode int    `json:"status_code"`
	StatusMsg  string `json:"status_msg"`
}

// SendIM 通过已登录浏览器发送一条文本私信。
//
// 参数与网页端点击「发送」一致：
//   - to_user_id：对方数字 user_id（不是抖音号 handle）
//   - msg_type=7：文本消息
//   - content：JSON 字符串 {"text":"..."}
func (c *Client) SendIM(accountID uint, cookie, toUserID, text string) error {
	if c == nil {
		return fmt.Errorf("签名服务未配置")
	}
	if toUserID == "" {
		return fmt.Errorf("缺少对方抖音 user_id，无法发送")
	}
	content, _ := json.Marshal(map[string]string{"text": text})
	form := url.Values{}
	form.Set("to_user_id", toUserID)
	form.Set("msg_type", "7")
	form.Set("content", string(content))
	form.Set("scene", "chat")
	form.Set("client_key", "")
	form.Set("verify_type", "")

	res, err := c.Fetch(accountID, cookie, imSendEndpoint, "POST", form.Encode(), map[string]string{
		"content-type": "application/x-www-form-urlencoded",
	})
	if err != nil {
		return err
	}
	if res.Status != 200 {
		return fmt.Errorf("私信接口返回 %d: %s", res.Status, truncate(res.Body, 160))
	}
	var out IMMessage
	if err := json.Unmarshal([]byte(res.Body), &out); err != nil {
		return fmt.Errorf("响应无法解析: %s", truncate(res.Body, 160))
	}
	if out.StatusCode != 0 {
		return fmt.Errorf("发送失败(%d): %s", out.StatusCode, out.StatusMsg)
	}
	return nil
}

// Conversation 会话列表中的好友。
type Conversation struct {
	UserID    string `json:"userId"`
	Nickname  string `json:"nickname"`
	SecUID    string `json:"secUid"`
	UniqueID  string `json:"uniqueId"`
	AvatarURL string `json:"avatarUrl"`
}

// FetchConversations 拉取私信会话列表（用于一键同步好友）。
// 抖音接口字段会变，这里做了宽松解析，解析不到就返回空列表交给前端手动添加。
func (c *Client) FetchConversations(accountID uint, cookie string) ([]Conversation, error) {
	res, err := c.Fetch(accountID, cookie, imListEndpoint, "GET", "", nil)
	if err != nil {
		return nil, err
	}
	if res.Status != 200 {
		return nil, fmt.Errorf("会话接口返回 %d", res.Status)
	}
	var raw struct {
		Data struct {
			Conversations []map[string]any `json:"conversation_list"`
			List          []map[string]any `json:"conversations"`
		} `json:"data"`
		ConversationList []map[string]any `json:"conversation_list"`
	}
	if err := json.Unmarshal([]byte(res.Body), &raw); err != nil {
		return nil, fmt.Errorf("会话数据解析失败")
	}
	rows := raw.Data.Conversations
	if len(rows) == 0 {
		rows = raw.Data.List
	}
	if len(rows) == 0 {
		rows = raw.ConversationList
	}
	out := make([]Conversation, 0, len(rows))
	for _, row := range rows {
		user, _ := row["user"].(map[string]any)
		if user == nil {
			user = row
		}
		conv := Conversation{
			UserID:    firstString(user, "user_id", "uid", "id"),
			Nickname:  firstString(user, "nickname", "name"),
			SecUID:    firstString(user, "sec_uid", "secUid"),
			UniqueID:  firstString(user, "unique_id", "short_id"),
			AvatarURL: firstAvatar(user),
		}
		if conv.UserID != "" && conv.Nickname != "" {
			out = append(out, conv)
		}
	}
	return out, nil
}

func firstString(row map[string]any, keys ...string) string {
	for _, key := range keys {
		switch value := row[key].(type) {
		case string:
			if value != "" {
				return value
			}
		case float64:
			return fmt.Sprintf("%.0f", value)
		}
	}
	return ""
}

func firstAvatar(row map[string]any) string {
	for _, key := range []string{"avatar_larger", "avatar_thumb", "avatar_medium"} {
		if obj, ok := row[key].(map[string]any); ok {
			if list, ok := obj["url_list"].([]any); ok && len(list) > 0 {
				if url, ok := list[0].(string); ok {
					return url
				}
			}
		}
	}
	return ""
}
