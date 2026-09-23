package ticket

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var idRe = regexp.MustCompile(`^([A-Z]+)-([A-Z]{2})-([a-z0-9._-]+)-(\d{5,16})$`)
var userCleanRe = regexp.MustCompile(`[^a-z0-9._-]`)

func FormatID(code, region, username string, seq int) string {
	safeRegion := strings.ToUpper(strings.TrimSpace(region))
	if !ValidRegion(safeRegion) {
		safeRegion = "XX"
	}
	safeUser := userCleanRe.ReplaceAllString(strings.ToLower(username), "")
	if len(safeUser) > 20 {
		safeUser = safeUser[:20]
	}
	if safeUser == "" {
		safeUser = "member"
	}
	return fmt.Sprintf("%s-%s-%s-%05d", code, safeRegion, safeUser, seq)
}

func ParseID(id string) (code, region, username string, seq int, ok bool) {
	m := idRe.FindStringSubmatch(id)
	if m == nil {
		return "", "", "", 0, false
	}
	n, err := strconv.Atoi(m[4])
	if err != nil || n < 0 {
		return "", "", "", 0, false
	}
	return m[1], m[2], m[3], n, true
}
