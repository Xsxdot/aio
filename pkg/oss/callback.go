package oss

import (
	"crypto"
	"crypto/md5"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/url"
	"strconv"

	"github.com/gofiber/fiber/v2"
)

// getPublicKey : Get PublicKey bytes from Request.URL
func getPublicKey(r *fiber.Ctx) ([]byte, error) {
	var bytePublicKey []byte

	// get PublicKey URL
	publicKeyURLBase64 := r.Get("x-oss-pub-key-url")
	if publicKeyURLBase64 == "" {
		fmt.Println("GetPublicKey from Request header failed :  No x-oss-pub-key-url field. ")
		return bytePublicKey, errors.New("no x-oss-pub-key-url field in Request header ")
	}
	publicKeyURL, _ := base64.StdEncoding.DecodeString(publicKeyURLBase64)

	// get PublicKey Content from URL
	responsePublicKeyURL, err := http.Get(string(publicKeyURL))
	if err != nil {
		fmt.Printf("Get PublicKey Content from URL failed : %s \n", err.Error())
		return bytePublicKey, err
	}

	bytePublicKey, err = ioutil.ReadAll(responsePublicKeyURL.Body)
	if err != nil {
		fmt.Printf("Read PublicKey Content from URL failed : %s \n", err.Error())
		return bytePublicKey, err
	}
	defer responsePublicKeyURL.Body.Close()

	return bytePublicKey, nil
}

// getAuthorization : decode from Base64String
func getAuthorization(r *fiber.Ctx) ([]byte, error) {
	var byteAuthorization []byte

	strAuthorizationBase64 := r.Get("authorization")
	if strAuthorizationBase64 == "" {
		fmt.Println("Failed to get authorization field from request header. ")
		return byteAuthorization, errors.New("no authorization field in Request header")
	}
	byteAuthorization, _ = base64.StdEncoding.DecodeString(strAuthorizationBase64)

	return byteAuthorization, nil
}

// getMD5FromNewAuthString : Get MD5 bytes from Newly Constructed Authrization String.
func getMD5FromNewAuthString(r *fiber.Ctx) ([]byte, error) {
	var byteMD5 []byte

	// Construct the New Auth String from URI+Query+Body
	bodyContent := r.Body()

	strCallbackBody := string(bodyContent)
	strURLPathDecode, errUnescape := unescapePath(r.OriginalURL(), encodePathSegment)
	if errUnescape != nil {
		fmt.Printf("url.PathUnescape failed : URL.Path=%s,  \n", r.OriginalURL())
		return byteMD5, errUnescape
	}

	// Generate New Auth String prepare for MD5
	query := make(url.Values)
	r.Request().URI().QueryArgs().VisitAll(func(key, value []byte) {
		query.Add(string(key), string(value))
	})
	rawQuery := query.Encode()
	strAuth := ""
	if rawQuery == "" {
		strAuth = fmt.Sprintf("%s\n%s", strURLPathDecode, strCallbackBody)
	} else {
		strAuth = fmt.Sprintf("%s?%s\n%s", strURLPathDecode, rawQuery, strCallbackBody)
	}

	// Generate MD5 from the New Auth String
	md5Ctx := md5.New()
	md5Ctx.Write([]byte(strAuth))
	byteMD5 = md5Ctx.Sum(nil)

	return byteMD5, nil
}

// 自定义RSA PKCS1v15签名验证，避开Go的密钥长度检查
func customVerifyPKCS1v15(pub *rsa.PublicKey, hash crypto.Hash, hashed []byte, sig []byte) error {
	if len(sig) != pub.Size() {
		return rsa.ErrVerification
	}

	// 将签名转换为大整数
	signature := new(big.Int).SetBytes(sig)

	// 计算s^e mod n (公钥验证)
	e := big.NewInt(int64(pub.E))
	n := pub.N
	m := new(big.Int).Exp(signature, e, n)
	em := padLeftWithZeros(m.Bytes(), pub.Size())

	// PKCS1v15验证格式
	// EM = 0x00 || 0x01 || PS || 0x00 || T
	// 其中PS是填充，T是DigestInfo编码
	if em[0] != 0 || em[1] != 1 {
		return rsa.ErrVerification
	}

	// 查找分隔符0x00
	var i int
	for i = 2; i < len(em); i++ {
		if em[i] == 0 {
			break
		}
		if em[i] != 0xff {
			return rsa.ErrVerification
		}
	}

	// 必须至少有8字节的填充
	if i < 10 {
		return rsa.ErrVerification
	}
	i++ // 跳过分隔符

	// 现在em[i:]应该包含DigestInfo
	// 对于MD5，DigestInfo由固定的ASN.1头部和摘要组成
	prefix := []byte{0x30, 0x20, 0x30, 0x0c, 0x06, 0x08, 0x2a, 0x86, 0x48, 0x86, 0xf7, 0x0d, 0x02, 0x05, 0x05, 0x00, 0x04, 0x10}

	// 检查DigestInfo前缀
	if len(em) < i+len(prefix)+len(hashed) {
		return rsa.ErrVerification
	}

	if !equalConstantTime(em[i:i+len(prefix)], prefix) {
		return rsa.ErrVerification
	}
	i += len(prefix)

	// 检查哈希值是否匹配
	if !equalConstantTime(em[i:i+len(hashed)], hashed) {
		return rsa.ErrVerification
	}

	return nil
}

// 等时间比较两个字节切片，防止定时攻击
func equalConstantTime(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var result byte = 0
	for i := 0; i < len(a); i++ {
		result |= a[i] ^ b[i]
	}
	return result == 0
}

// 用零填充字节切片左侧
func padLeftWithZeros(b []byte, size int) []byte {
	if len(b) >= size {
		return b
	}
	padded := make([]byte, size)
	copy(padded[size-len(b):], b)
	return padded
}

// verifySignature
func verifySignature(bytePublicKey []byte, byteMd5 []byte, authorization []byte) bool {
	pubBlock, _ := pem.Decode(bytePublicKey)
	if pubBlock == nil {
		fmt.Printf("Failed to parse PEM block containing the public key")
		return false
	}
	pubInterface, err := x509.ParsePKIXPublicKey(pubBlock.Bytes)
	if (pubInterface == nil) || (err != nil) {
		fmt.Printf("x509.ParsePKIXPublicKey(publicKey) failed : %s \n", err.Error())
		return false
	}
	pub := pubInterface.(*rsa.PublicKey)

	//// 使用自定义验证函数代替标准库函数
	err = customVerifyPKCS1v15(pub, crypto.MD5, byteMd5, authorization)
	if err != nil {
		fmt.Printf("Custom Signature Verification is Failed : %s \n", err.Error())
		return false
	}

	//errorVerifyPKCS1v15 := rsa.VerifyPKCS1v15(pub, crypto.MD5, byteMd5, authorization)
	//if errorVerifyPKCS1v15 != nil {
	//	fmt.Printf("Signature Verification is Failed : %s \n", errorVerifyPKCS1v15.Error())
	//	return false
	//}

	fmt.Printf("Signature Verification is Successful. \n")
	return true
}

// EscapeError Escape Error
type EscapeError string

func (e EscapeError) Error() string {
	return "invalid URL escape " + strconv.Quote(string(e))
}

// InvalidHostError Invalid Host Error
type InvalidHostError string

func (e InvalidHostError) Error() string {
	return "invalid character " + strconv.Quote(string(e)) + " in host name"
}

type encoding int

const (
	encodePath encoding = 1 + iota
	encodePathSegment
	encodeHost
	encodeZone
	encodeUserPassword
	encodeQueryComponent
	encodeFragment
)

// unescapePath : unescapes a string; the mode specifies, which section of the URL string is being unescaped.
func unescapePath(s string, mode encoding) (string, error) {
	// Count %, check that they're well-formed.
	mode = encodePathSegment
	n := 0
	hasPlus := false
	for i := 0; i < len(s); {
		switch s[i] {
		case '%':
			n++
			if i+2 >= len(s) || !ishex(s[i+1]) || !ishex(s[i+2]) {
				s = s[i:]
				if len(s) > 3 {
					s = s[:3]
				}
				return "", EscapeError(s)
			}
			// Per https://tools.ietf.org/html/rfc3986#page-21
			// in the host component %-encoding can only be used
			// for non-ASCII bytes.
			// But https://tools.ietf.org/html/rfc6874#section-2
			// introduces %25 being allowed to escape a percent sign
			// in IPv6 scoped-address literals. Yay.
			if mode == encodeHost && unhex(s[i+1]) < 8 && s[i:i+3] != "%25" {
				return "", EscapeError(s[i : i+3])
			}
			if mode == encodeZone {
				// RFC 6874 says basically "anything goes" for zone identifiers
				// and that even non-ASCII can be redundantly escaped,
				// but it seems prudent to restrict %-escaped bytes here to those
				// that are valid host name bytes in their unescaped form.
				// That is, you can use escaping in the zone identifier but not
				// to introduce bytes you couldn't just write directly.
				// But Windows puts spaces here! Yay.
				v := unhex(s[i+1])<<4 | unhex(s[i+2])
				if s[i:i+3] != "%25" && v != ' ' && shouldEscape(v, encodeHost) {
					return "", EscapeError(s[i : i+3])
				}
			}
			i += 3
		case '+':
			hasPlus = mode == encodeQueryComponent
			i++
		default:
			if (mode == encodeHost || mode == encodeZone) && s[i] < 0x80 && shouldEscape(s[i], mode) {
				return "", InvalidHostError(s[i : i+1])
			}
			i++
		}
	}

	if n == 0 && !hasPlus {
		return s, nil
	}

	t := make([]byte, len(s)-2*n)
	j := 0
	for i := 0; i < len(s); {
		switch s[i] {
		case '%':
			t[j] = unhex(s[i+1])<<4 | unhex(s[i+2])
			j++
			i += 3
		case '+':
			if mode == encodeQueryComponent {
				t[j] = ' '
			} else {
				t[j] = '+'
			}
			j++
			i++
		default:
			t[j] = s[i]
			j++
			i++
		}
	}
	return string(t), nil
}

// Please be informed that for now shouldEscape does not check all
// reserved characters correctly. See golang.org/issue/5684.
func shouldEscape(c byte, mode encoding) bool {
	// §2.3 Unreserved characters (alphanum)
	if 'A' <= c && c <= 'Z' || 'a' <= c && c <= 'z' || '0' <= c && c <= '9' {
		return false
	}

	if mode == encodeHost || mode == encodeZone {
		// §3.2.2 Host allows
		//	sub-delims = "!" / "$" / "&" / "'" / "(" / ")" / "*" / "+" / "," / ";" / "="
		// as part of reg-name.
		// We add : because we include :port as part of host.
		// We add [ ] because we include [ipv6]:port as part of host.
		// We add < > because they're the only characters left that
		// we could possibly allow, and Parse will reject them if we
		// escape them (because hosts can't use %-encoding for
		// ASCII bytes).
		switch c {
		case '!', '$', '&', '\'', '(', ')', '*', '+', ',', ';', '=', ':', '[', ']', '<', '>', '"':
			return false
		}
	}

	switch c {
	case '-', '_', '.', '~': // §2.3 Unreserved characters (mark)
		return false

	case '$', '&', '+', ',', '/', ':', ';', '=', '?', '@': // §2.2 Reserved characters (reserved)
		// Different sections of the URL allow a few of
		// the reserved characters to appear unescaped.
		switch mode {
		case encodePath: // §3.3
			// The RFC allows : @ & = + $ but saves / ; , for assigning
			// meaning to individual path segments. This package
			// only manipulates the path as a whole, so we allow those
			// last three as well. That leaves only ? to escape.
			return c == '?'

		case encodePathSegment: // §3.3
			// The RFC allows : @ & = + $ but saves / ; , for assigning
			// meaning to individual path segments.
			return c == '/' || c == ';' || c == ',' || c == '?'

		case encodeUserPassword: // §3.2.1
			// The RFC allows ';', ':', '&', '=', '+', '$', and ',' in
			// userinfo, so we must escape only '@', '/', and '?'.
			// The parsing of userinfo treats ':' as special so we must escape
			// that too.
			return c == '@' || c == '/' || c == '?' || c == ':'

		case encodeQueryComponent: // §3.4
			// The RFC reserves (so we must escape) everything.
			return true

		case encodeFragment: // §4.1
			// The RFC text is silent but the grammar allows
			// everything, so escape nothing.
			return false
		}
	}

	// Everything else must be escaped.
	return true
}

func ishex(c byte) bool {
	switch {
	case '0' <= c && c <= '9':
		return true
	case 'a' <= c && c <= 'f':
		return true
	case 'A' <= c && c <= 'F':
		return true
	}
	return false
}

func unhex(c byte) byte {
	switch {
	case '0' <= c && c <= '9':
		return c - '0'
	case 'a' <= c && c <= 'f':
		return c - 'a' + 10
	case 'A' <= c && c <= 'F':
		return c - 'A' + 10
	}
	return 0
}
