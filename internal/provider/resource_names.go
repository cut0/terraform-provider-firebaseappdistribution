package provider

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	projectNumberRegexp = regexp.MustCompile(`^[0-9]+$`)
	groupNameRegexp     = regexp.MustCompile(`^projects/([^/]+)/groups/([^/]+)$`)
	groupMemberIDRegexp = regexp.MustCompile(`^projects/([^/]+)/groups/([^/]+)/testers/([^/]+)$`)
)

func isProjectNumber(project string) bool {
	return projectNumberRegexp.MatchString(project)
}

func projectParent(project string) string {
	return "projects/" + project
}

func groupName(project, groupID string) string {
	return fmt.Sprintf("projects/%s/groups/%s", project, groupID)
}

func parseGroupName(name string) (project, groupID string, ok bool) {
	matches := groupNameRegexp.FindStringSubmatch(strings.TrimSpace(name))
	if matches == nil {
		return "", "", false
	}
	return matches[1], matches[2], true
}

func groupMemberID(project, groupID, email string) string {
	return fmt.Sprintf("%s/testers/%s", groupName(project, groupID), email)
}

func parseGroupMemberID(id string) (project, groupID, email string, ok bool) {
	matches := groupMemberIDRegexp.FindStringSubmatch(strings.TrimSpace(id))
	if matches == nil {
		return "", "", "", false
	}
	return matches[1], matches[2], matches[3], true
}

func testerEmail(name string) string {
	if i := strings.LastIndex(name, "/testers/"); i >= 0 {
		return name[i+len("/testers/"):]
	}
	return name
}
