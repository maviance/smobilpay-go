package s3p

import "testing"

func TestGenerateSignature(t *testing.T) {
	postParams := map[string]string{
		"amount":                   "1000",
		"payItemId":                "SPAY-DEV-958-AES-100013333-10010",
		"s3pAuth_nonce":            "634968823463411609",
		"s3pAuth_signature_method": "HMAC-SHA1",
		"s3pAuth_timestamp":        "1361281946",
		"s3pAuth_token":            "xvz1evFS4wEEPTGEFPHBog",
	}
	expectedPost := "1CLm+TQLwelkE+5Za+Vi+7G5M8U="
	gotPost := GenerateSignature("POST", "https://dev.smobilpay.com/s3p/v2/quotestd", postParams, "MySecretKey")
	if gotPost != expectedPost {
		t.Errorf("POST signature mismatch.\nExpected: %s\nGot: %s", expectedPost, gotPost)
	}

	getParams := map[string]string{
		"serviceNumber":            "TestId",
		"merchant":                 "TESTMERC",
		"serviceid":                "99999",
		"s3pAuth_nonce":            "634968823463411611",
		"s3pAuth_signature_method": "HMAC-SHA1",
		"s3pAuth_timestamp":        "1361281946",
		"s3pAuth_token":            "xvz1evFS4wEEPTGEFPHBog",
	}
	expectedGet := "wff4LW5sueJe0K4Uzk7fHrjElGk="
	gotGet := GenerateSignature("GET", "https://dev.smobilpay.com/s3p/v2/bill", getParams, "MySecretKey")
	if gotGet != expectedGet {
		t.Errorf("GET signature mismatch.\nExpected: %s\nGot: %s", expectedGet, gotGet)
	}
}
