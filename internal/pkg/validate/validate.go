package validate

import (
	"strconv"
	"strings"
	"unicode/utf8"
)

const (
	MaxShortRunes = 50
	MaxLongRunes  = 1000
)

type Checker struct {
	MaxShort int
	MaxLong  int

	errs []string
}

func New() *Checker {
	return &Checker{
		MaxShort: MaxShortRunes,
		MaxLong:  MaxLongRunes,
	}
}

func (c *Checker) Err() string { return strings.Join(c.errs, "；") }

func (c *Checker) HasError() bool { return len(c.errs) > 0 }

func (c *Checker) Fail(msg string) *Checker {
	c.errs = append(c.errs, msg)
	return c
}

func (c *Checker) Required(field, value string) string {
	v := strings.TrimSpace(value)
	if v == "" {
		c.errs = append(c.errs, field+"不能为空")
	}
	return v
}

func (c *Checker) Short(field, value string) string {
	v := c.Required(field, value)
	c.MaxRunes(field, v, c.MaxShort)
	return v
}

func (c *Checker) ShortOptional(field, value string) string {
	v := strings.TrimSpace(value)
	if v != "" {
		c.MaxRunes(field, v, c.MaxShort)
	}
	return v
}

func (c *Checker) Long(field, value string) string {
	v := strings.TrimSpace(value)
	c.MaxRunes(field, v, c.MaxLong)
	return v
}

func (c *Checker) Discussion(field, value string) string {
	v := strings.TrimSpace(value)
	if v == "" {
		c.errs = append(c.errs, field+"不能为空")
		return v
	}
	c.MaxRunes(field, v, c.MaxLong)
	return v
}

func (c *Checker) MaxRunes(field, value string, max int) *Checker {
	if max > 0 && utf8.RuneCountInString(value) > max {
		c.errs = append(c.errs, field+"长度不能超过"+itoa(max)+"字")
	}
	return c
}

func (c *Checker) Phone(field, value string) string {
	v := strings.TrimSpace(value)
	if v == "" {
		c.errs = append(c.errs, field+"不能为空")
		return v
	}
	if len(v) != 11 || v[0] != '1' || !isDigits(v) {
		c.errs = append(c.errs, field+"格式不正确")
	}
	return v
}

func (c *Checker) Email(field, value string) string {
	v := strings.TrimSpace(value)
	if v == "" {
		return v
	}
	if len(v) > 128 || !strings.Contains(v, "@") || strings.HasPrefix(v, "@") || strings.HasSuffix(v, "@") {
		c.errs = append(c.errs, field+"格式不正确")
	}
	return v
}

func (c *Checker) Digits(field, value string, max int) string {
	v := strings.TrimSpace(value)
	if v == "" {
		return v
	}
	if !isDigits(v) || len(v) > max {
		c.errs = append(c.errs, field+"必须是数字且不超过"+itoa(max)+"位")
	}
	return v
}

func (c *Checker) Password(field, value string) string {
	if len(value) < 8 {
		c.errs = append(c.errs, field+"长度至少 8 位")
		return value
	}
	if len(value) > 64 {
		c.errs = append(c.errs, field+"长度不能超过 64 位")
		return value
	}
	var hasLetter, hasDigit bool
	for _, r := range value {
		switch {
		case r >= '0' && r <= '9':
			hasDigit = true
		case (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z'):
			hasLetter = true
		}
	}
	if !hasLetter || !hasDigit {
		c.errs = append(c.errs, field+"必须同时包含字母和数字")
	}
	return value
}

func (c *Checker) Positive(field string, value, max int) int {
	if value <= 0 {
		c.errs = append(c.errs, field+"必须大于 0")
		return value
	}
	if max > 0 && value > max {
		c.errs = append(c.errs, field+"不能超过"+itoa(max))
	}
	return value
}

func isDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func itoa(n int) string { return strconv.Itoa(n) }
