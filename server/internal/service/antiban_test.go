package service

import "testing"

func TestIsRiskSignal(t *testing.T) {
	cases := map[string]bool{
		"私信接口返回 403: 风控校验未通过": true,
		"操作太频繁，请稍后再试":        true,
		"需要验证码":                true,
		"too many requests":    true,
		"账号异常，已被限制":           true,
		"网络超时":                false,
		"聊天面板没有打开，输入框未出现":     false,
	}
	for msg, want := range cases {
		if got := isRiskSignal(msg); got != want {
			t.Fatalf("%q: got %v want %v", msg, got, want)
		}
	}
}
