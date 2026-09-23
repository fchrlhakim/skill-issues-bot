package ticket

import (
	"regexp"
	"strings"
	"unicode"
)

type sensitivePattern struct {
	Label    string
	Re       *regexp.Regexp
	Validate func(string) bool
}

func labelledSecret(label string) string {
	return `(?i)\b(?:` + label + `)\b["'\x60*]*\s*(?::|=|\bis\b|\badalah\b|[ \t]+-[ \t]+)[\s"'\x60*]*[^\s"'\x60*,;:=]+`
}

var sensitivePatterns = []sensitivePattern{
	{Label: "Discord token", Re: regexp.MustCompile(`(?i)(?:mfa\.[\w-]{20,}|[\w-]{24,28}\.[\w-]{6}\.[\w-]{27,40})`)},
	{Label: "private or API key", Re: regexp.MustCompile(`(?i)(?:-----BEGIN (?:RSA |EC |OPENSSH |DSA |ENCRYPTED )?PRIVATE KEY-----|(?:sk|pk)-(?:live|test|proj)?[_-]?[a-z0-9_-]{20,}|` + labelledSecret(`(?:api|access|client)[ _-]*(?:key|token|secret)|private[ _-]*key`) + `|\bbearer[ \t]+[a-z0-9._~+/=-]{12,}={0,2})`)},
	{Label: "full card number", Validate: hasCardNumber},
	{Label: "OTP, PIN, or CVV", Re: regexp.MustCompile(`(?i)\b(?:otp|2fa|cvv|cvc|pin)\b(?:[ \t]+code)?["'\x60*]*\s*(?:(?:is|adalah)\b|[=:-])?[\s"'\x60*]*\d{3,8}\b`)},
	{Label: "seed or recovery phrase", Re: regexp.MustCompile(`(?i)\b(?:seed|recovery)\s+phrase\s*(?:is|=|:|-).{8,}`)},
	{Label: "password", Re: regexp.MustCompile(labelledSecret(`password|passwd|(?:kata\s+)?sandi`))},
}

func hasCardNumber(text string) bool {
	cleaned := strings.ReplaceAll(text, "<@", " ")
	var b strings.Builder
	for _, r := range cleaned {
		if unicode.IsDigit(r) || r == ' ' || r == '\t' || r == '-' {
			b.WriteRune(r)
		} else {
			b.WriteByte(' ')
		}
	}
	fields := strings.FieldsFunc(b.String(), func(r rune) bool {
		return r != ' ' && r != '\t' && r != '-' && !unicode.IsDigit(r)
	})
	joined := strings.Join(fields, " ")
	run := make([]byte, 0, 32)
	flush := func() bool {
		if len(run) < 13 || len(run) > 19 || run[0] == '0' {
			run = run[:0]
			return false
		}
		sum := 0
		for i := len(run) - 1; i >= 0; i-- {
			d := int(run[i] - '0')
			if (len(run)-1-i)%2 == 1 {
				d *= 2
				if d > 9 {
					d -= 9
				}
			}
			sum += d
		}
		run = run[:0]
		return sum%10 == 0
	}
	for i := 0; i < len(joined); i++ {
		c := joined[i]
		if c >= '0' && c <= '9' {
			run = append(run, c)
			continue
		}
		if c == ' ' || c == '\t' || c == '-' {
			continue
		}
		if flush() {
			return true
		}
	}
	return flush()
}

func SensitiveFindings(data map[string]string) []string {
	var labels []string
	seen := map[string]bool{}
	for _, p := range sensitivePatterns {
		for _, text := range data {
			hit := false
			if p.Validate != nil {
				hit = p.Validate(text)
			} else if p.Re != nil {
				hit = p.Re.MatchString(text)
			}
			if !hit {
				continue
			}
			if !seen[p.Label] {
				seen[p.Label] = true
				labels = append(labels, p.Label)
			}
			break
		}
	}
	return labels
}

var StaffRoleNames = map[string]bool{
	"Server Owner": true, "Platform Administrator": true, "Operations Lead": true,
	"Intermediary Lead": true, "Intermediary": true, "Dispute & Compliance": true,
	"Security & Fraud": true, "Finance & Risk": true, "Support & Verification": true,
	"Regional & Translation": true, "Moderator": true,
}

func IsStaffRole(name string) bool {
	return StaffRoleNames[name]
}
