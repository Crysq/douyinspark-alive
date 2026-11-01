package signer

import (
	"encoding/json"
	"fmt"
	"net/url"
)

// 抖音 Web 端当前参数（旧的 19.5.0 已被网关以 "url doesn't match" 拒绝）
const webQuery = "device_platform=webapp&aid=6383&channel=channel_pc_web&pc_client_type=1&version_code=170400&version_name=17.4.0&cookie_enabled=true&platform=PC&update_version_code=170400"

const (
	imSendEndpoint     = "https://www.douyin.com/aweme/v1/web/im/send/msg/?" + webQuery
	imUserInfoEndpoint = "https://www.douyin.com/aweme/v1/web/im/user/info/?" + webQuery
	emojiListEndpoint  = "https://www.douyin.com/aweme/v1/web/emoji/list?" + webQuery + "&need_all=true"
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

// FriendProfile 会话好友资料（由 sec_uid 批量换取）。
type FriendProfile struct {
	UID       string `json:"uid"`
	SecUID    string `json:"secUid"`
	UniqueID  string `json:"uniqueId"`
	Nickname  string `json:"nickname"`
	AvatarURL string `json:"avatarUrl"`
}

// FetchFriendSecUIDs 从聊天页截获当前账号的全部会话 sec_uid。
func (c *Client) FetchFriendSecUIDs(accountID uint) ([]string, error) {
	var out struct {
		SecUserIDs []string `json:"secUserIds"`
	}
	if err := c.post("/friends", map[string]any{"accountId": accountID, "waitMs": 9000}, &out); err != nil {
		return nil, err
	}
	if len(out.SecUserIDs) == 0 {
		return nil, fmt.Errorf("聊天页没有返回会话，请确认账号已登录且能打开 douyin.com/chat")
	}
	return out.SecUserIDs, nil
}

type imUserInfoRow struct {
	Nickname    string `json:"nickname"`
	SecUID      string `json:"sec_uid"`
	UniqueID    string `json:"unique_id"`
	ShortID     string `json:"short_id"`
	UID         any    `json:"uid"`
	AvatarThumb struct {
		URLList []string `json:"url_list"`
	} `json:"avatar_thumb"`
	AvatarSmall struct {
		URLList []string `json:"url_list"`
	} `json:"avatar_small"`
}

// FetchFriendProfiles 用 sec_uid 批量换取昵称 / 数字 user_id / 头像。
func (c *Client) FetchFriendProfiles(accountID uint, cookie string, secUIDs []string) ([]FriendProfile, error) {
	if len(secUIDs) == 0 {
		return nil, nil
	}
	out := make([]FriendProfile, 0, len(secUIDs))
	// 接口按批处理，控制单次数量，避免 URL/表单过大
	const batch = 20
	for start := 0; start < len(secUIDs); start += batch {
		end := start + batch
		if end > len(secUIDs) {
			end = len(secUIDs)
		}
		raw, _ := json.Marshal(secUIDs[start:end])
		form := url.Values{}
		form.Set("sec_user_ids", string(raw))
		res, err := c.Fetch(accountID, cookie, imUserInfoEndpoint, "POST", form.Encode(), map[string]string{
			"content-type": "application/x-www-form-urlencoded",
		})
		if err != nil {
			return out, err
		}
		if res.Status != 200 {
			return out, fmt.Errorf("用户资料接口返回 %d", res.Status)
		}
		var parsed struct {
			Data []imUserInfoRow `json:"data"`
		}
		if err := json.Unmarshal([]byte(res.Body), &parsed); err != nil {
			return out, fmt.Errorf("用户资料解析失败")
		}
		for _, row := range parsed.Data {
			uid := ""
			switch v := row.UID.(type) {
			case float64:
				uid = fmt.Sprintf("%.0f", v)
			case string:
				uid = v
			}
			avatar := ""
			if len(row.AvatarThumb.URLList) > 0 {
				avatar = row.AvatarThumb.URLList[0]
			} else if len(row.AvatarSmall.URLList) > 0 {
				avatar = row.AvatarSmall.URLList[0]
			}
			uniqueID := row.UniqueID
			if uniqueID == "" {
				uniqueID = row.ShortID
			}
			out = append(out, FriendProfile{UID: uid, SecUID: row.SecUID, UniqueID: uniqueID, Nickname: row.Nickname, AvatarURL: avatar})
		}
	}
	return out, nil
}

// Emoji 聊天框表情。
type Emoji struct {
	Name string `json:"name"` // 例如 [鲸鱼点赞]
	URI  string `json:"uri"`
	URL  string `json:"url"`
}

// FetchEmojis 拉取聊天框表情面板（原生 emoji 列表）。
func (c *Client) FetchEmojis(accountID uint, cookie string) ([]Emoji, error) {
	res, err := c.Fetch(accountID, cookie, emojiListEndpoint, "GET", "", nil)
	if err != nil {
		return nil, err
	}
	if res.Status != 200 {
		return nil, fmt.Errorf("表情接口返回 %d", res.Status)
	}
	var parsed struct {
		EmojiList []struct {
			DisplayName string `json:"display_name"`
			OriginURI   string `json:"origin_uri"`
			EmojiURL    struct {
				URLList []string `json:"url_list"`
			} `json:"emoji_url"`
		} `json:"emoji_list"`
	}
	if err := json.Unmarshal([]byte(res.Body), &parsed); err != nil {
		return nil, fmt.Errorf("表情数据解析失败")
	}
	out := make([]Emoji, 0, len(parsed.EmojiList))
	for _, item := range parsed.EmojiList {
		url := ""
		if len(item.EmojiURL.URLList) > 0 {
			url = item.EmojiURL.URLList[0]
		}
		if item.DisplayName == "" {
			continue
		}
		out = append(out, Emoji{Name: item.DisplayName, URI: item.OriginURI, URL: url})
	}
	return out, nil
}
