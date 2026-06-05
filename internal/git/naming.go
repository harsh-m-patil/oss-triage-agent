package git

import (
	"regexp"
	"strconv"
	"strings"
	"unicode"

	"github.com/harsh-m-patil/oss-triage-agent/internal/issue"
)

const maxShortTitleLen = 48

var multiHyphen = regexp.MustCompile(`-+`)

// BranchName returns the agent branch for an issue per CONTEXT.md conventions.
func BranchName(iss issue.Issue) string {
	return "agent/issue-" + strconv.Itoa(iss.Number) + "-" + ShortTitleSlug(iss.Title)
}

// WorktreeDirName is the directory name under WorktreePath() for an issue worktree.
func WorktreeDirName(iss issue.Issue) string {
	return "issue-" + strconv.Itoa(iss.Number) + "-" + ShortTitleSlug(iss.Title)
}

// ShortTitleSlug derives a kebab-case slug from an issue title.
func ShortTitleSlug(title string) string {
	var b strings.Builder
	lastHyphen := false
	for _, r := range strings.ToLower(strings.TrimSpace(title)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			lastHyphen = false
			continue
		}
		if !lastHyphen && b.Len() > 0 {
			b.WriteByte('-')
			lastHyphen = true
		}
	}
	slug := strings.Trim(multiHyphen.ReplaceAllString(b.String(), "-"), "-")
	if len(slug) > maxShortTitleLen {
		slug = slug[:maxShortTitleLen]
		slug = strings.Trim(slug, "-")
		if i := strings.LastIndex(slug, "-"); i > 0 {
			slug = slug[:i]
		}
	}
	return slug
}
