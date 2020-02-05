package gitver

import (
	"log"
	"os/user"
	"regexp"
	"strings"
	"text/template"
)

// DefaultTemplate Default template for formatting GitVersion using String()
const DefaultTemplate = "{{if .Dirty}}dirty-{{.User}}-{{end}}{{.Commits}}.{{.Hash}}"

var sanitizedUserCache = &cachedStringResponse{}

// GitVersion version information about one or more git projects
type GitVersion struct {
	Branch  string
	Commits string
	Hash    string
	Dirty   bool
	User    string
}

// FormatTemplate formats a GitVersion using a text/template string
func (ver *GitVersion) FormatTemplate(arg string) (string, error) {
	versionTemplate, err := template.New("Version Template").Parse(arg)
	if err != nil {
		return "", err
	}

	var builder strings.Builder
	if versionTemplate.Execute(&builder, ver) != nil {
		return "", err
	}
	return builder.String(), nil
}

func (ver *GitVersion) String() string {
	result, err := ver.FormatTemplate(DefaultTemplate)
	if err != nil {
		log.Fatalf("Error formatting version template: %s", err)
		return ""
	}
	return result
}

// ForProjects computes the GitVersion for a set of projects relative to the git root
func (ver *GitVersion) ForProjects(projects []string) error {
	paths, err := GitProjectPaths(projects)
	if err != nil {
		return err
	}

	if ver.Branch, err = GitBranch(); err != nil {
		return err
	}

	if ver.Commits, err = countCommits(paths); err != nil {
		return err
	}

	rev, err := headCommit(paths)
	if err != nil {
		return err
	}
	if ver.Hash, err = revParseShort(rev); err != nil {
		return err
	}

	if ver.Dirty, err = checkIsDirty(paths); err != nil {
		return err
	}

	if ver.User, err = currentUser(); err != nil {
		return err
	}
	return nil
}

func currentUser() (string, error) {
	sanitizedUserCache.Do(func() {
		var currentUser *user.User
		currentUser, sanitizedUserCache.err = user.Current()
		if sanitizedUserCache.err != nil {
			return
		}

		var alphaNumericPattern *regexp.Regexp
		alphaNumericPattern, sanitizedUserCache.err = regexp.Compile("[^a-zA-Z0-9]+")
		if sanitizedUserCache.err != nil {
			return
		}

		sanitizedUserCache.response = alphaNumericPattern.ReplaceAllString(currentUser.Username, "")
		sanitizedUserCache.err = nil
	})
	return sanitizedUserCache.response, sanitizedUserCache.err
}