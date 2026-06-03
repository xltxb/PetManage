package member

import (
	"sync"
	"testing"

	"gorm.io/gorm/schema"
)

func TestCustomerWechatOpenIDMapsToSchemaColumn(t *testing.T) {
	parsed, err := schema.Parse(&Customer{}, &sync.Map{}, schema.NamingStrategy{})
	if err != nil {
		t.Fatalf("parse Customer schema: %v", err)
	}
	field := parsed.LookUpField("WechatOpenID")
	if field == nil {
		t.Fatal("WechatOpenID field not found")
	}
	if field.DBName != "wechat_openid" {
		t.Fatalf("WechatOpenID DBName = %q, want wechat_openid", field.DBName)
	}
}

func TestCustomerUpdateFieldsOmitEmptyWechatOpenID(t *testing.T) {
	fields := customerUpdateFields(&Customer{ID: 1, Name: "陈睿", WechatOpenID: ""})
	if _, ok := fields["wechat_openid"]; ok {
		t.Fatalf("empty wechat_openid should not be included in customer updates: %#v", fields)
	}

	fields = customerUpdateFields(&Customer{ID: 1, Name: "陈睿", WechatOpenID: "openid-1"})
	if fields["wechat_openid"] != "openid-1" {
		t.Fatalf("wechat_openid = %#v, want openid-1", fields["wechat_openid"])
	}
}
