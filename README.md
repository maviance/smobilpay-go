# smobilpay-s3p-signature-go

Go library to generate S3P HMAC-SHA1 signatures for Smobilpay API authentication.

## 📦 Installation

```bash
go get github.com/maviance/smobilpay-go/s3p
```

## 🚀 Usage

### POST example

```go
params := map[string]string{
  "amount": "1000",
  "payItemId": "SPAY-DEV-958-AES-100013333-10010",
  "s3pAuth_nonce": "634968823463411609",
  "s3pAuth_signature_method": "HMAC-SHA1",
  "s3pAuth_timestamp": "1361281946",
  "s3pAuth_token": "xvz1evFS4wEEPTGEFPHBog",
}
signature := s3p.GenerateSignature("POST", "https://dev.smobilpay.com/s3p/v2/quotestd", params, "MySecretKey")
```

### GET example

```go
params := map[string]string{
  "serviceNumber": "TestId",
  "merchant": "TESTMERC",
  "serviceid": "99999",
  "s3pAuth_nonce": "634968823463411611",
  "s3pAuth_signature_method": "HMAC-SHA1",
  "s3pAuth_timestamp": "1361281946",
  "s3pAuth_token": "xvz1evFS4wEEPTGEFPHBog",
}
signature := s3p.GenerateSignature("GET", "https://dev.smobilpay.com/s3p/v2/bill", params, "MySecretKey")
```

## 🧪 Testing

```bash
go test ./...
```

## 📄 License

MIT © 2025 Maviance PLC
