package network

import "testing"

func TestGlobalBind(t *testing.T) {
	_, err := GlobalBind("127.0.0.1:2000")
	if err != nil {
		t.Error("Wrong with global bind")
	}
	_, err = GlobalBind("127.0.0.12000")
	if err == nil {
		t.Error("Wrong with global bind")
	}
}
