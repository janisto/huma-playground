package github

import (
	"bytes"
	"encoding/json"
	"net/url"
	"sort"
	"strconv"
	"time"
	"unicode/utf8"

	"github.com/janisto/huma-playground/internal/platform/portable"
)

const maximumSafeInteger uint64 = 9_007_199_254_740_991

func projectOwner(document []byte) (Owner, error) {
	object, err := decodeObject(document)
	if err != nil {
		return Owner{}, err
	}
	id, err := requiredSafeInteger(object, "id")
	if err != nil {
		return Owner{}, err
	}
	login, err := requiredNonEmptyString(object, "login")
	if err != nil {
		return Owner{}, err
	}
	accountType, err := requiredNonEmptyString(object, "type")
	if err != nil {
		return Owner{}, err
	}
	avatarURL, err := requiredURL(object, "avatar_url")
	if err != nil {
		return Owner{}, err
	}
	htmlURL, err := requiredURL(object, "html_url")
	if err != nil {
		return Owner{}, err
	}
	publicRepos, err := requiredSafeInteger(object, "public_repos")
	if err != nil {
		return Owner{}, err
	}
	followers, err := requiredSafeInteger(object, "followers")
	if err != nil {
		return Owner{}, err
	}
	following, err := requiredSafeInteger(object, "following")
	if err != nil {
		return Owner{}, err
	}
	createdAt, err := requiredTimestamp(object, "created_at")
	if err != nil {
		return Owner{}, err
	}
	updatedAt, err := requiredTimestamp(object, "updated_at")
	if err != nil {
		return Owner{}, err
	}
	name, err := optionalDisplayString(object, "name")
	if err != nil {
		return Owner{}, err
	}
	company, err := optionalDisplayString(object, "company")
	if err != nil {
		return Owner{}, err
	}
	blog, err := optionalDisplayString(object, "blog")
	if err != nil {
		return Owner{}, err
	}
	location, err := optionalDisplayString(object, "location")
	if err != nil {
		return Owner{}, err
	}
	bio, err := optionalDisplayString(object, "bio")
	if err != nil {
		return Owner{}, err
	}
	return Owner{
		ID: id, Login: login, Type: accountType, Name: name, AvatarURL: avatarURL,
		HTMLURL: htmlURL, Company: company, Blog: blog, Location: location, Bio: bio,
		PublicRepos: publicRepos, Followers: followers, Following: following,
		CreatedAt: createdAt, UpdatedAt: updatedAt,
	}, nil
}

func projectRepositorySummaries(document []byte, limit int) ([]RepositorySummary, error) {
	entries, err := decodeArray(document)
	if err != nil || len(entries) > limit {
		return nil, ErrUpstream
	}
	repositories := make([]RepositorySummary, len(entries))
	for index, entry := range entries {
		object, err := rawObject(entry)
		if err != nil {
			return nil, err
		}
		repositories[index], err = repositorySummary(object)
		if err != nil {
			return nil, err
		}
	}
	return repositories, nil
}

func projectRepository(document []byte) (Repository, error) {
	object, err := decodeObject(document)
	if err != nil {
		return Repository{}, err
	}
	summary, err := repositorySummary(object)
	if err != nil {
		return Repository{}, err
	}
	language, err := requiredNullableString(object, "language")
	if err != nil {
		return Repository{}, err
	}
	stargazers, err := requiredSafeInteger(object, "stargazers_count")
	if err != nil {
		return Repository{}, err
	}
	forks, err := requiredSafeInteger(object, "forks_count")
	if err != nil {
		return Repository{}, err
	}
	openIssues, err := requiredSafeInteger(object, "open_issues_count")
	if err != nil {
		return Repository{}, err
	}
	archived, err := requiredBool(object, "archived")
	if err != nil {
		return Repository{}, err
	}
	createdAt, err := requiredTimestamp(object, "created_at")
	if err != nil {
		return Repository{}, err
	}
	updatedAt, err := requiredTimestamp(object, "updated_at")
	if err != nil {
		return Repository{}, err
	}
	pushedAt, err := requiredNullableTimestamp(object, "pushed_at")
	if err != nil {
		return Repository{}, err
	}
	defaultBranch, err := requiredNonEmptyString(object, "default_branch")
	if err != nil {
		return Repository{}, err
	}
	license, err := optionalLicense(object)
	if err != nil {
		return Repository{}, err
	}
	topics, err := optionalTopics(object)
	if err != nil {
		return Repository{}, err
	}
	disabled, err := requiredBool(object, "disabled")
	if err != nil {
		return Repository{}, err
	}
	return Repository{
		RepositorySummary: summary, Language: language, StargazersCount: stargazers,
		ForksCount: forks, OpenIssuesCount: openIssues, Archived: archived,
		CreatedAt: createdAt, UpdatedAt: updatedAt, PushedAt: pushedAt,
		DefaultBranch: defaultBranch, License: license, Topics: topics, Disabled: disabled,
	}, nil
}

func repositorySummary(object map[string]json.RawMessage) (RepositorySummary, error) {
	private, err := requiredBool(object, "private")
	if err != nil || private {
		return RepositorySummary{}, ErrUpstream
	}
	visibility, err := requiredString(object, "visibility")
	if err != nil || visibility != "public" {
		return RepositorySummary{}, ErrUpstream
	}
	id, err := requiredSafeInteger(object, "id")
	if err != nil {
		return RepositorySummary{}, err
	}
	name, err := requiredNonEmptyString(object, "name")
	if err != nil {
		return RepositorySummary{}, err
	}
	fullName, err := requiredNonEmptyString(object, "full_name")
	if err != nil {
		return RepositorySummary{}, err
	}
	description, err := requiredNullableString(object, "description")
	if err != nil {
		return RepositorySummary{}, err
	}
	htmlURL, err := requiredURL(object, "html_url")
	if err != nil {
		return RepositorySummary{}, err
	}
	fork, err := requiredBool(object, "fork")
	if err != nil {
		return RepositorySummary{}, err
	}
	return RepositorySummary{
		ID:          id,
		Name:        name,
		FullName:    fullName,
		Description: description,
		HTMLURL:     htmlURL,
		Fork:        fork,
	}, nil
}

func projectActivities(document []byte, limit int) ([]Activity, error) {
	entries, err := decodeArray(document)
	if err != nil || len(entries) > limit {
		return nil, ErrUpstream
	}
	activities := make([]Activity, len(entries))
	for index, entry := range entries {
		object, err := rawObject(entry)
		if err != nil {
			return nil, err
		}
		id, err := requiredSafeInteger(object, "id")
		if err != nil {
			return nil, err
		}
		ref, err := requiredNonEmptyString(object, "ref")
		if err != nil {
			return nil, err
		}
		timestamp, err := requiredTimestamp(object, "timestamp")
		if err != nil {
			return nil, err
		}
		activityType, err := requiredNonEmptyString(object, "activity_type")
		if err != nil {
			return nil, err
		}
		actor, avatar, err := requiredActor(object)
		if err != nil {
			return nil, err
		}
		activities[index] = Activity{
			ID: id, Actor: actor, ActorAvatarURL: avatar, Ref: ref, Timestamp: timestamp, ActivityType: activityType,
		}
	}
	return activities, nil
}

func projectLanguages(document []byte) ([]Language, error) {
	object, err := decodeObject(document)
	if err != nil {
		return nil, err
	}
	languages := make([]Language, 0, len(object))
	for name, rawBytes := range object {
		if name == "" || !utf8.ValidString(name) {
			return nil, ErrUpstream
		}
		byteCount, err := safeInteger(rawBytes)
		if err != nil {
			return nil, err
		}
		languages = append(languages, Language{Name: name, Bytes: byteCount})
	}
	sort.Slice(languages, func(left, right int) bool {
		if languages[left].Bytes != languages[right].Bytes {
			return languages[left].Bytes > languages[right].Bytes
		}
		return languages[left].Name < languages[right].Name
	})
	return languages, nil
}

func projectTags(document []byte, limit int) ([]Tag, error) {
	entries, err := decodeArray(document)
	if err != nil || len(entries) > limit {
		return nil, ErrUpstream
	}
	tags := make([]Tag, len(entries))
	for index, entry := range entries {
		object, err := rawObject(entry)
		if err != nil {
			return nil, err
		}
		name, err := requiredNonEmptyString(object, "name")
		if err != nil {
			return nil, err
		}
		commitRaw, ok := object["commit"]
		if !ok {
			return nil, ErrUpstream
		}
		commit, err := rawObject(commitRaw)
		if err != nil {
			return nil, err
		}
		sha, err := requiredString(commit, "sha")
		if err != nil || !validObjectID(sha) {
			return nil, ErrUpstream
		}
		tags[index] = Tag{Name: name, SHA: sha}
	}
	return tags, nil
}

func decodeObject(document []byte) (map[string]json.RawMessage, error) {
	if _, err := portable.ParseStrictJSON(document); err != nil {
		return nil, ErrUpstream
	}
	return rawObject(document)
}

func rawObject(raw json.RawMessage) (map[string]json.RawMessage, error) {
	var object map[string]json.RawMessage
	if err := json.Unmarshal(raw, &object); err != nil || object == nil {
		return nil, ErrUpstream
	}
	return object, nil
}

func decodeArray(document []byte) ([]json.RawMessage, error) {
	if _, err := portable.ParseStrictJSON(document); err != nil {
		return nil, ErrUpstream
	}
	var array []json.RawMessage
	if err := json.Unmarshal(document, &array); err != nil || array == nil {
		return nil, ErrUpstream
	}
	return array, nil
}

func requiredString(object map[string]json.RawMessage, field string) (string, error) {
	raw, ok := object[field]
	if !ok {
		return "", ErrUpstream
	}
	var value string
	if err := json.Unmarshal(raw, &value); err != nil || !utf8.ValidString(value) {
		return "", ErrUpstream
	}
	return value, nil
}

func requiredNonEmptyString(object map[string]json.RawMessage, field string) (string, error) {
	value, err := requiredString(object, field)
	if err != nil || value == "" {
		return "", ErrUpstream
	}
	return value, nil
}

func requiredNullableString(object map[string]json.RawMessage, field string) (*string, error) {
	raw, ok := object[field]
	if !ok {
		return nil, ErrUpstream
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	value, err := requiredString(object, field)
	if err != nil {
		return nil, err
	}
	return &value, nil
}

func optionalDisplayString(object map[string]json.RawMessage, field string) (*string, error) {
	if _, ok := object[field]; !ok {
		return nil, nil
	}
	value, err := requiredNullableString(object, field)
	if err != nil || value == nil || *value == "" {
		return nil, err
	}
	return value, nil
}

func requiredBool(object map[string]json.RawMessage, field string) (bool, error) {
	raw, ok := object[field]
	value := string(bytes.TrimSpace(raw))
	if !ok || value != "true" && value != "false" {
		return false, ErrUpstream
	}
	return value == "true", nil
}

func requiredSafeInteger(object map[string]json.RawMessage, field string) (uint64, error) {
	raw, ok := object[field]
	if !ok {
		return 0, ErrUpstream
	}
	return safeInteger(raw)
}

func safeInteger(raw json.RawMessage) (uint64, error) {
	value := string(bytes.TrimSpace(raw))
	if value == "" || value != "0" && value[0] == '0' {
		return 0, ErrUpstream
	}
	for index := range len(value) {
		if value[index] < '0' || value[index] > '9' {
			return 0, ErrUpstream
		}
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil || parsed > maximumSafeInteger {
		return 0, ErrUpstream
	}
	return parsed, nil
}

func requiredURL(object map[string]json.RawMessage, field string) (string, error) {
	value, err := requiredNonEmptyString(object, field)
	if err != nil {
		return "", err
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Scheme != "http" && parsed.Scheme != "https" || parsed.Host == "" {
		return "", ErrUpstream
	}
	return value, nil
}

func requiredTimestamp(object map[string]json.RawMessage, field string) (time.Time, error) {
	value, err := requiredNonEmptyString(object, field)
	if err != nil {
		return time.Time{}, err
	}
	return parseTimestamp(value)
}

func requiredNullableTimestamp(object map[string]json.RawMessage, field string) (*time.Time, error) {
	raw, ok := object[field]
	if !ok {
		return nil, ErrUpstream
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	value, err := requiredNonEmptyString(object, field)
	if err != nil {
		return nil, err
	}
	parsed, err := parseTimestamp(value)
	if err != nil {
		return nil, err
	}
	return &parsed, nil
}

func parseTimestamp(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil || parsed.Nanosecond()%1_000_000 != 0 || parsed.Year() < 0 || parsed.Year() > 9999 {
		return time.Time{}, ErrUpstream
	}
	return parsed.UTC(), nil
}

func requiredActor(object map[string]json.RawMessage) (*string, *string, error) {
	raw, ok := object["actor"]
	if !ok {
		return nil, nil, ErrUpstream
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil, nil
	}
	actor, err := rawObject(raw)
	if err != nil {
		return nil, nil, err
	}
	login, err := requiredNonEmptyString(actor, "login")
	if err != nil {
		return nil, nil, err
	}
	avatar, err := requiredURL(actor, "avatar_url")
	if err != nil {
		return nil, nil, err
	}
	return &login, &avatar, nil
}

func optionalLicense(object map[string]json.RawMessage) (*string, error) {
	raw, ok := object["license"]
	if !ok || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return nil, nil
	}
	license, err := rawObject(raw)
	if err != nil {
		return nil, err
	}
	if _, ok := license["spdx_id"]; !ok {
		return nil, nil
	}
	spdx, err := requiredNullableString(license, "spdx_id")
	if err != nil || spdx == nil || *spdx == "" || *spdx == "NOASSERTION" {
		return nil, err
	}
	return spdx, nil
}

func optionalTopics(object map[string]json.RawMessage) ([]string, error) {
	raw, ok := object["topics"]
	if !ok {
		return []string{}, nil
	}
	var topics []string
	if err := json.Unmarshal(raw, &topics); err != nil || topics == nil {
		return nil, ErrUpstream
	}
	seen := make(map[string]struct{}, len(topics))
	for _, topic := range topics {
		if !utf8.ValidString(topic) {
			return nil, ErrUpstream
		}
		if _, duplicate := seen[topic]; duplicate {
			return nil, ErrUpstream
		}
		seen[topic] = struct{}{}
	}
	sort.Strings(topics)
	return topics, nil
}

func validObjectID(value string) bool {
	if len(value) != 40 && len(value) != 64 {
		return false
	}
	for index := range len(value) {
		if (value[index] < '0' || value[index] > '9') && (value[index] < 'a' || value[index] > 'f') {
			return false
		}
	}
	return true
}
